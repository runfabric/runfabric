export enum TriggerEnum {
  Http = "http",
  Cron = "cron",
  Queue = "queue",
  Storage = "storage",
  EventBridge = "eventbridge",
  PubSub = "pubsub",
  Kafka = "kafka",
  RabbitMq = "rabbitmq"
}

export enum AwsIamEffectEnum {
  Allow = "Allow",
  Deny = "Deny"
}

export enum AwsQueueFunctionResponseTypeEnum {
  ReportBatchItemFailures = "ReportBatchItemFailures"
}
