package aws

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	elasticacheapi "github.com/aws/aws-sdk-go-v2/service/elasticache"
	rdsapi "github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/runfabric/runfabric/engine/internal/provisioning"
)

// AWSProvisioner implements provisioning.Provisioner for AWS RDS and ElastiCache.
// For provision: true resources, the spec may include:
//   - type: "database" | "rds" | "cache" | "elasticache"
//   - identifier: RDS DB instance ID or ElastiCache replication group ID (required for lookup)
//   - region: optional; defaults to AWS_REGION or provider region
//   - (RDS only) userEnv, passwordEnv, dbNameEnv: env var names for user, password, db name to build connection string
//   - (RDS only) engine: "postgres" | "mysql" for scheme (default postgres)
func (p *AWSProvisioner) Provision(ctx context.Context, provider, resourceKey string, spec map[string]any) (string, error) {
	if spec == nil {
		return "", provisioning.ErrNotImplemented
	}
	typ, _ := spec["type"].(string)
	id, _ := spec["identifier"].(string)
	if id == "" {
		return "", provisioning.ErrNotImplemented
	}

	region := os.Getenv("AWS_REGION")
	if r, _ := spec["region"].(string); r != "" {
		region = r
	}
	if region == "" {
		region = "us-east-1"
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return "", fmt.Errorf("aws config: %w", err)
	}

	switch typ {
	case "database", "rds":
		return p.provisionRDS(ctx, cfg, spec, id)
	case "cache", "elasticache":
		return p.provisionElastiCache(ctx, cfg, spec, id)
	default:
		return "", provisioning.ErrNotImplemented
	}
}

func (p *AWSProvisioner) provisionRDS(ctx context.Context, cfg aws.Config, spec map[string]any, instanceID string) (string, error) {
	client := rdsapi.NewFromConfig(cfg)
	out, err := client.DescribeDBInstances(ctx, &rdsapi.DescribeDBInstancesInput{
		DBInstanceIdentifier: &instanceID,
	})
	if err != nil {
		return "", fmt.Errorf("rds describe: %w", err)
	}
	if len(out.DBInstances) == 0 {
		return "", fmt.Errorf("rds: instance %s not found", instanceID)
	}
	inst := out.DBInstances[0]
	if inst.Endpoint == nil || inst.Endpoint.Address == nil {
		return "", fmt.Errorf("rds: instance %s has no endpoint", instanceID)
	}
	addr := *inst.Endpoint.Address
	port := 5432
	if inst.Endpoint.Port != nil {
		port = int(*inst.Endpoint.Port)
	}

	userEnv, _ := spec["userEnv"].(string)
	passwordEnv, _ := spec["passwordEnv"].(string)
	dbNameEnv, _ := spec["dbNameEnv"].(string)
	engine, _ := spec["engine"].(string)
	if engine == "" {
		engine = "postgres"
	}
	if engine != "postgres" && engine != "mysql" {
		engine = "postgres"
	}

	user := ""
	password := ""
	dbName := "postgres"
	if engine == "mysql" {
		dbName = "mysql"
	}
	if userEnv != "" {
		user = os.Getenv(userEnv)
	}
	if passwordEnv != "" {
		password = os.Getenv(passwordEnv)
	}
	if dbNameEnv != "" {
		if d := os.Getenv(dbNameEnv); d != "" {
			dbName = d
		}
	}

	if user == "" || password == "" {
		return "", provisioning.ErrNotImplemented
	}

	// Build URL: scheme://user:password@host:port/dbname
	userPart := url.UserPassword(user, password).String()
	raw := fmt.Sprintf("%s://%s@%s:%d/%s", engine, userPart, addr, port, dbName)
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (p *AWSProvisioner) provisionElastiCache(ctx context.Context, cfg aws.Config, spec map[string]any, id string) (string, error) {
	client := elasticacheapi.NewFromConfig(cfg)

	// Prefer replication group (Redis with replication); fallback to cache cluster.
	out, err := client.DescribeReplicationGroups(ctx, &elasticacheapi.DescribeReplicationGroupsInput{
		ReplicationGroupId: &id,
	})
	if err == nil && len(out.ReplicationGroups) > 0 {
		rg := out.ReplicationGroups[0]
		if len(rg.NodeGroups) == 0 {
			return "", fmt.Errorf("elasticache: replication group %s has no node groups", id)
		}
		ng := rg.NodeGroups[0]
		if ng.PrimaryEndpoint == nil {
			return "", fmt.Errorf("elasticache: replication group %s has no primary endpoint", id)
		}
		addr := ""
		if ng.PrimaryEndpoint.Address != nil {
			addr = *ng.PrimaryEndpoint.Address
		}
		port := 6379
		if ng.PrimaryEndpoint.Port != nil {
			port = int(*ng.PrimaryEndpoint.Port)
		}
		return fmt.Sprintf("redis://%s:%d", addr, port), nil
	}

	// Single cluster (no replication group)
	co, err := client.DescribeCacheClusters(ctx, &elasticacheapi.DescribeCacheClustersInput{
		CacheClusterId:    &id,
		ShowCacheNodeInfo: aws.Bool(true),
	})
	if err != nil {
		return "", fmt.Errorf("elasticache describe: %w", err)
	}
	if len(co.CacheClusters) == 0 {
		return "", fmt.Errorf("elasticache: cluster %s not found", id)
	}
	cl := co.CacheClusters[0]
	if len(cl.CacheNodes) == 0 {
		return "", fmt.Errorf("elasticache: cluster %s has no nodes", id)
	}
	node := cl.CacheNodes[0]
	if node.Endpoint == nil {
		return "", fmt.Errorf("elasticache: cluster %s node has no endpoint", id)
	}
	addr := ""
	if node.Endpoint.Address != nil {
		addr = *node.Endpoint.Address
	}
	port := 6379
	if node.Endpoint.Port != nil {
		port = int(*node.Endpoint.Port)
	}
	return fmt.Sprintf("redis://%s:%d", addr, port), nil
}

// AWSProvisioner is the AWS implementation of provisioning.Provisioner.
type AWSProvisioner struct{}

func init() {
	// Register AWS provisioner so deploy can resolve provision: true resources.
	provisioning.Register("aws", &AWSProvisioner{})
	provisioning.Register("aws-lambda", &AWSProvisioner{})
}
