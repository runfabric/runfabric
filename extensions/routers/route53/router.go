package route53

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	route53svc "github.com/aws/aws-sdk-go-v2/service/route53"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"
)

const (
	defaultRegion = "us-east-1"
)

// Router implements route53-backed DNS reconciliation.
type Router struct{}

func NewRouter() sdkrouter.Router {
	return Router{}
}

func RouterMeta() sdkrouter.PluginMeta {
	return sdkrouter.PluginMeta{
		ID:          "route53",
		Name:        "Route53 Router",
		Description: "Route53 DNS reconciler",
	}
}

func (Router) Meta() sdkrouter.PluginMeta {
	return RouterMeta()
}

type apiClient interface {
	ListResourceRecordSets(ctx context.Context, params *route53svc.ListResourceRecordSetsInput, optFns ...func(*route53svc.Options)) (*route53svc.ListResourceRecordSetsOutput, error)
	ChangeResourceRecordSets(ctx context.Context, params *route53svc.ChangeResourceRecordSetsInput, optFns ...func(*route53svc.Options)) (*route53svc.ChangeResourceRecordSetsOutput, error)
}

func (Router) Sync(ctx context.Context, req sdkrouter.RouterSyncRequest) (*sdkrouter.RouterSyncResult, error) {
	if req.Routing == nil {
		return nil, fmt.Errorf("routing config is nil")
	}
	zoneID := strings.TrimSpace(req.ZoneID)
	if zoneID == "" {
		return nil, fmt.Errorf("ROUTE53 hosted zone ID is required (pass --zone-id)")
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(defaultRegion))
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	client := route53svc.NewFromConfig(cfg)
	return syncWithClient(ctx, client, normalizeZoneID(zoneID), req)
}

func syncWithClient(ctx context.Context, client apiClient, zoneID string, req sdkrouter.RouterSyncRequest) (*sdkrouter.RouterSyncResult, error) {
	desired, err := desiredRecords(req.Routing)
	if err != nil {
		return nil, err
	}
	existing, err := loadExisting(ctx, client, zoneID, req.Routing.Hostname)
	if err != nil {
		return nil, err
	}
	result := &sdkrouter.RouterSyncResult{DryRun: req.DryRun}
	desiredMap := make(map[string]route53types.ResourceRecordSet, len(desired))
	for _, record := range desired {
		desiredMap[recordKey(record)] = record
	}
	existingMap := make(map[string]route53types.ResourceRecordSet, len(existing))
	for _, record := range existing {
		existingMap[recordKey(record)] = record
	}

	changeBatch := make([]route53types.Change, 0, len(desired))
	keys := make([]string, 0, len(desiredMap))
	for k := range desiredMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		desiredRecord := desiredMap[key]
		existingRecord, found := existingMap[key]
		name := strings.TrimSpace(strings.TrimSuffix(awsv2.ToString(desiredRecord.Name), "."))
		if !found {
			result.Actions = append(result.Actions, sdkrouter.RouterSyncAction{
				Resource: "dns_record",
				Action:   "create",
				Name:     name,
				Detail:   describeRecord(desiredRecord),
			})
			changeBatch = append(changeBatch, route53types.Change{Action: route53types.ChangeActionUpsert, ResourceRecordSet: &desiredRecord})
			continue
		}
		if recordsEqual(existingRecord, desiredRecord) {
			result.Actions = append(result.Actions, sdkrouter.RouterSyncAction{
				Resource: "dns_record",
				Action:   "no-op",
				Name:     name,
				Detail:   describeRecord(desiredRecord),
			})
			continue
		}
		result.Actions = append(result.Actions, sdkrouter.RouterSyncAction{
			Resource: "dns_record",
			Action:   "update",
			Name:     name,
			Detail:   describeRecord(desiredRecord),
		})
		changeBatch = append(changeBatch, route53types.Change{Action: route53types.ChangeActionUpsert, ResourceRecordSet: &desiredRecord})
	}
	for key, record := range existingMap {
		if _, keep := desiredMap[key]; keep {
			continue
		}
		name := strings.TrimSpace(strings.TrimSuffix(awsv2.ToString(record.Name), "."))
		result.Actions = append(result.Actions, sdkrouter.RouterSyncAction{
			Resource: "dns_record",
			Action:   "delete-candidate",
			Name:     name,
			Detail:   describeRecord(record),
		})
	}
	if req.DryRun || len(changeBatch) == 0 {
		return result, nil
	}
	_, err = client.ChangeResourceRecordSets(ctx, &route53svc.ChangeResourceRecordSetsInput{
		HostedZoneId: awsv2.String(zoneID),
		ChangeBatch: &route53types.ChangeBatch{
			Comment: awsv2.String("managed-by:runfabric"),
			Changes: changeBatch,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("route53 reconcile failed: %w", err)
	}
	return result, nil
}

func loadExisting(ctx context.Context, client apiClient, zoneID, hostname string) ([]route53types.ResourceRecordSet, error) {
	name := ensureFQDN(hostname)
	resp, err := client.ListResourceRecordSets(ctx, &route53svc.ListResourceRecordSetsInput{
		HostedZoneId:    awsv2.String(zoneID),
		StartRecordName: awsv2.String(name),
		StartRecordType: route53types.RRTypeCname,
		MaxItems:        awsv2.Int32(100),
	})
	if err != nil {
		return nil, fmt.Errorf("route53 list records: %w", err)
	}
	out := make([]route53types.ResourceRecordSet, 0, len(resp.ResourceRecordSets))
	for _, record := range resp.ResourceRecordSets {
		if record.Type != route53types.RRTypeCname {
			continue
		}
		if !sameFQDN(awsv2.ToString(record.Name), name) {
			continue
		}
		out = append(out, record)
	}
	return out, nil
}

func desiredRecords(routing *sdkrouter.RoutingConfig) ([]route53types.ResourceRecordSet, error) {
	if strings.TrimSpace(routing.Hostname) == "" {
		return nil, fmt.Errorf("routing hostname is required")
	}
	if len(routing.Endpoints) == 0 {
		return nil, fmt.Errorf("routing endpoints are required")
	}
	ttl := int64(routing.TTL)
	if ttl <= 0 {
		ttl = 60
	}
	name := ensureFQDN(routing.Hostname)
	out := make([]route53types.ResourceRecordSet, 0, len(routing.Endpoints))

	for idx, ep := range routing.Endpoints {
		target, err := cnameTarget(ep.URL)
		if err != nil {
			return nil, fmt.Errorf("endpoint %q: %w", ep.Name, err)
		}
		record := route53types.ResourceRecordSet{
			Name: awsv2.String(name),
			Type: route53types.RRTypeCname,
			TTL:  awsv2.Int64(ttl),
			ResourceRecords: []route53types.ResourceRecord{
				{Value: awsv2.String(target)},
			},
		}
		if strings.EqualFold(strings.TrimSpace(routing.Strategy), "failover") && len(routing.Endpoints) == 2 {
			if idx == 0 {
				record.Failover = route53types.ResourceRecordSetFailoverPrimary
			} else {
				record.Failover = route53types.ResourceRecordSetFailoverSecondary
			}
			record.SetIdentifier = awsv2.String(normalizeIdentifier(ep.Name, idx))
		} else {
			weight := int64(ep.Weight)
			if weight <= 0 {
				weight = 1
			}
			record.SetIdentifier = awsv2.String(normalizeIdentifier(ep.Name, idx))
			record.Weight = awsv2.Int64(weight)
		}
		out = append(out, record)
	}
	return out, nil
}

func normalizeIdentifier(name string, idx int) string {
	s := strings.ToLower(strings.TrimSpace(name))
	if s == "" {
		return fmt.Sprintf("endpoint-%d", idx+1)
	}
	return s
}

func recordKey(record route53types.ResourceRecordSet) string {
	if record.Failover != "" {
		return "failover:" + strings.ToLower(strings.TrimSpace(string(record.Failover)))
	}
	id := strings.ToLower(strings.TrimSpace(awsv2.ToString(record.SetIdentifier)))
	if id != "" {
		return "id:" + id
	}
	return "default"
}

func recordsEqual(a, b route53types.ResourceRecordSet) bool {
	if !sameFQDN(awsv2.ToString(a.Name), awsv2.ToString(b.Name)) {
		return false
	}
	if a.Type != b.Type {
		return false
	}
	if awsv2.ToInt64(a.TTL) != awsv2.ToInt64(b.TTL) {
		return false
	}
	if a.Failover != b.Failover {
		return false
	}
	if awsv2.ToInt64(a.Weight) != awsv2.ToInt64(b.Weight) {
		return false
	}
	if strings.TrimSpace(awsv2.ToString(a.SetIdentifier)) != strings.TrimSpace(awsv2.ToString(b.SetIdentifier)) {
		return false
	}
	if len(a.ResourceRecords) != len(b.ResourceRecords) {
		return false
	}
	for i := range a.ResourceRecords {
		if !sameFQDN(awsv2.ToString(a.ResourceRecords[i].Value), awsv2.ToString(b.ResourceRecords[i].Value)) {
			return false
		}
	}
	return true
}

func describeRecord(record route53types.ResourceRecordSet) string {
	target := ""
	if len(record.ResourceRecords) > 0 {
		target = strings.TrimSpace(strings.TrimSuffix(awsv2.ToString(record.ResourceRecords[0].Value), "."))
	}
	if record.Failover != "" {
		return fmt.Sprintf("route53 cname=%s failover=%s ttl=%d", target, record.Failover, awsv2.ToInt64(record.TTL))
	}
	if record.Weight != nil {
		return fmt.Sprintf("route53 cname=%s weight=%d ttl=%d", target, awsv2.ToInt64(record.Weight), awsv2.ToInt64(record.TTL))
	}
	return fmt.Sprintf("route53 cname=%s ttl=%d", target, awsv2.ToInt64(record.TTL))
}

func normalizeZoneID(zoneID string) string {
	zoneID = strings.TrimSpace(zoneID)
	zoneID = strings.TrimPrefix(zoneID, "/hostedzone/")
	if !strings.HasPrefix(zoneID, "Z") && strings.Contains(zoneID, "/") {
		parts := strings.Split(zoneID, "/")
		zoneID = parts[len(parts)-1]
	}
	return zoneID
}

func ensureFQDN(name string) string {
	n := strings.TrimSpace(strings.ToLower(name))
	n = strings.TrimSuffix(n, ".")
	if n == "" {
		return "."
	}
	return n + "."
}

func sameFQDN(a, b string) bool {
	return ensureFQDN(a) == ensureFQDN(b)
}

func cnameTarget(raw string) (string, error) {
	target := strings.TrimSpace(raw)
	if target == "" {
		return "", fmt.Errorf("endpoint URL is empty")
	}
	parsed, err := url.Parse(target)
	if err == nil && parsed.Host != "" {
		target = parsed.Host
	}
	target = strings.TrimSpace(strings.TrimSuffix(target, "."))
	if target == "" {
		return "", fmt.Errorf("endpoint URL %q has no host", raw)
	}
	if strings.Contains(target, "/") {
		return "", fmt.Errorf("endpoint URL %q resolves to invalid hostname %q", raw, target)
	}
	return target + ".", nil
}
