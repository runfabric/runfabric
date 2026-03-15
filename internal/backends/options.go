package backends

type Options struct {
	Kind            string
	Root            string
	AWSRegion       string
	S3Bucket        string
	S3Prefix        string
	DynamoTableName string
}
