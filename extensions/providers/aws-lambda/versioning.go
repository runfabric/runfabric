package aws

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	lambdav2 "github.com/aws/aws-sdk-go-v2/service/lambda"
)

// liveAlias is the alias every deploy points at its newest published version.
// Programmatic invocations resolve through it, so a rollback (repointing the
// alias to a prior version) changes what runs without redeploying code.
const liveAlias = "live"

// publishAndAlias publishes an immutable version of the function's current code
// and points the `live` alias at it (creating the alias on first deploy). This
// is what makes rollback possible — each deploy leaves a version behind.
func publishAndAlias(ctx context.Context, clients *AWSClients, functionName string) error {
	out, err := clients.Lambda.PublishVersion(ctx, &lambdav2.PublishVersionInput{
		FunctionName: awssdk.String(functionName),
	})
	if err != nil {
		return fmt.Errorf("PublishVersion %s: %w", functionName, err)
	}
	version := awssdk.ToString(out.Version)

	if _, err := clients.Lambda.GetAlias(ctx, &lambdav2.GetAliasInput{
		FunctionName: awssdk.String(functionName),
		Name:         awssdk.String(liveAlias),
	}); err != nil {
		if !isLambdaNotFound(err) {
			return fmt.Errorf("GetAlias %s: %w", functionName, err)
		}
		if _, err := clients.Lambda.CreateAlias(ctx, &lambdav2.CreateAliasInput{
			FunctionName:    awssdk.String(functionName),
			Name:            awssdk.String(liveAlias),
			FunctionVersion: awssdk.String(version),
		}); err != nil {
			return fmt.Errorf("CreateAlias %s: %w", functionName, err)
		}
		return nil
	}
	if _, err := clients.Lambda.UpdateAlias(ctx, &lambdav2.UpdateAliasInput{
		FunctionName:    awssdk.String(functionName),
		Name:            awssdk.String(liveAlias),
		FunctionVersion: awssdk.String(version),
	}); err != nil {
		return fmt.Errorf("UpdateAlias %s: %w", functionName, err)
	}
	return nil
}

// rollbackAlias repoints the `live` alias to the highest published version below
// its current target, returning the previous and new version. ok is false (with
// no error) when there is no earlier version to roll back to.
func rollbackAlias(ctx context.Context, clients *AWSClients, functionName string) (from, to string, ok bool, err error) {
	cur, err := clients.Lambda.GetAlias(ctx, &lambdav2.GetAliasInput{
		FunctionName: awssdk.String(functionName),
		Name:         awssdk.String(liveAlias),
	})
	if err != nil {
		if isLambdaNotFound(err) {
			return "", "", false, nil // never deployed with versioning
		}
		return "", "", false, fmt.Errorf("GetAlias %s: %w", functionName, err)
	}
	current := awssdk.ToString(cur.FunctionVersion)
	currentN, convErr := strconv.Atoi(current)
	if convErr != nil {
		return "", "", false, nil // alias points at $LATEST — nothing to roll back
	}

	versions, err := numericVersions(ctx, clients, functionName)
	if err != nil {
		return "", "", false, err
	}
	prev := 0
	for _, v := range versions {
		if v < currentN && v > prev {
			prev = v
		}
	}
	if prev == 0 {
		return current, "", false, nil // no earlier version
	}

	target := strconv.Itoa(prev)
	if _, err := clients.Lambda.UpdateAlias(ctx, &lambdav2.UpdateAliasInput{
		FunctionName:    awssdk.String(functionName),
		Name:            awssdk.String(liveAlias),
		FunctionVersion: awssdk.String(target),
	}); err != nil {
		return current, target, false, fmt.Errorf("UpdateAlias %s: %w", functionName, err)
	}
	return current, target, true, nil
}

// numericVersions lists the function's published (numeric) version numbers.
func numericVersions(ctx context.Context, clients *AWSClients, functionName string) ([]int, error) {
	var out []int
	var marker *string
	for {
		res, err := clients.Lambda.ListVersionsByFunction(ctx, &lambdav2.ListVersionsByFunctionInput{
			FunctionName: awssdk.String(functionName),
			Marker:       marker,
		})
		if err != nil {
			return nil, fmt.Errorf("ListVersionsByFunction %s: %w", functionName, err)
		}
		for _, v := range res.Versions {
			if n, err := strconv.Atoi(awssdk.ToString(v.Version)); err == nil {
				out = append(out, n)
			}
		}
		if res.NextMarker == nil {
			break
		}
		marker = res.NextMarker
	}
	sort.Ints(out)
	return out, nil
}
