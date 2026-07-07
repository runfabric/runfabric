package aws

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// Scaffold declares how `runfabric init` scaffolds an AWS Lambda project.
var Scaffold = sdkprovider.Scaffold{
	Comment: "Provider: aws-lambda — set AWS_REGION; optional backend: s3 + DynamoDB for state",
}
