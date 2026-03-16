package backends

type Options struct {
	Kind            string
	Root            string
	AWSRegion       string
	S3Bucket        string
	S3Prefix        string
	DynamoTableName string
	// DB-backed deploy state (receipts): 1.5 / 1.6
	PostgresDSN   string
	PostgresTable string
	SqlitePath    string
	ReceiptTable  string // DynamoDB table for receipts; if empty and Kind==dynamodb, use DynamoTableName
}
