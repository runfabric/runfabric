import { TriggerEnum, type ProjectConfig } from "@runfabric/core";

export type DevPreset =
  | "http"
  | "queue"
  | "storage"
  | "cron"
  | "eventbridge"
  | "pubsub"
  | "kafka"
  | "rabbitmq";

const eventPresetBuilders = {
  queue(provider: string, project: ProjectConfig): unknown {
    const trigger = project.triggers.find((item) => item.type === TriggerEnum.Queue);
    const queueName =
      typeof trigger?.queue === "string" && trigger.queue.trim().length > 0 ? trigger.queue.trim() : "dev-queue";
    if (provider === "aws-lambda") {
      return {
        Records: [
          {
            messageId: "dev-message-1",
            receiptHandle: "dev-receipt-1",
            body: JSON.stringify({ source: "runfabric-dev", queue: queueName }),
            attributes: { ApproximateReceiveCount: "1" },
            messageAttributes: {},
            md5OfBody: "d41d8cd98f00b204e9800998ecf8427e",
            eventSource: "aws:sqs",
            eventSourceARN: `arn:aws:sqs:us-east-1:000000000000:${queueName}`,
            awsRegion: "us-east-1"
          }
        ]
      };
    }
    if (provider === "gcp-functions") {
      return {
        message: {
          data: Buffer.from(JSON.stringify({ source: "runfabric-dev", queue: queueName })).toString("base64"),
          attributes: { queue: queueName }
        },
        subscription: `projects/dev/subscriptions/${queueName}`
      };
    }
    return { queue: queueName, records: [{ body: { source: "runfabric-dev", queue: queueName } }] };
  },
  storage(provider: string, project: ProjectConfig): unknown {
    const trigger = project.triggers.find((item) => item.type === TriggerEnum.Storage);
    const bucket =
      typeof trigger?.bucket === "string" && trigger.bucket.trim().length > 0
        ? trigger.bucket.trim()
        : "dev-bucket";
    const eventName =
      Array.isArray(trigger?.events) && typeof trigger.events[0] === "string" && trigger.events[0].trim().length > 0
        ? trigger.events[0]
        : "ObjectCreated:Put";
    if (provider === "aws-lambda") {
      return {
        Records: [
          {
            eventVersion: "2.1",
            eventSource: "aws:s3",
            awsRegion: "us-east-1",
            eventTime: new Date().toISOString(),
            eventName,
            s3: { bucket: { name: bucket }, object: { key: "dev/object.json", size: 42 } }
          }
        ]
      };
    }
    if (provider === "gcp-functions") {
      return {
        bucket,
        name: "dev/object.json",
        contentType: "application/json",
        timeCreated: new Date().toISOString()
      };
    }
    return { bucket, key: "dev/object.json", event: eventName };
  },
  cron(provider: string, project: ProjectConfig): unknown {
    const trigger = project.triggers.find((item) => item.type === TriggerEnum.Cron);
    const schedule =
      typeof trigger?.schedule === "string" && trigger.schedule.trim().length > 0
        ? trigger.schedule.trim()
        : "*/5 * * * *";
    if (provider === "aws-lambda") {
      return {
        version: "0",
        id: `dev-cron-${Date.now()}`,
        "detail-type": "Scheduled Event",
        source: "aws.events",
        time: new Date().toISOString(),
        resources: [`arn:aws:events:us-east-1:000000000000:rule/${project.service}`],
        detail: { schedule }
      };
    }
    return { schedule, triggeredAt: new Date().toISOString() };
  },
  eventbridge(_provider: string, project: ProjectConfig): unknown {
    return {
      version: "0",
      id: `dev-eventbridge-${Date.now()}`,
      source: "runfabric.dev",
      "detail-type": "dev.event",
      time: new Date().toISOString(),
      detail: { service: project.service, message: "eventbridge preset simulation" }
    };
  },
  pubsub(provider: string, project: ProjectConfig): unknown {
    const trigger = project.triggers.find((item) => item.type === TriggerEnum.PubSub);
    const topic =
      typeof trigger?.topic === "string" && trigger.topic.trim().length > 0 ? trigger.topic.trim() : "dev-topic";
    const message = { source: "runfabric-dev", topic };
    if (provider === "gcp-functions") {
      return {
        message: {
          data: Buffer.from(JSON.stringify(message)).toString("base64"),
          attributes: { topic }
        },
        subscription: `projects/dev/subscriptions/${topic}-sub`
      };
    }
    return { topic, message };
  },
  kafka(_provider: string, project: ProjectConfig): unknown {
    const trigger = project.triggers.find((item) => item.type === TriggerEnum.Kafka);
    const topic =
      typeof trigger?.topic === "string" && trigger.topic.trim().length > 0 ? trigger.topic.trim() : "dev-topic";
    const groupId =
      typeof trigger?.groupId === "string" && trigger.groupId.trim().length > 0
        ? trigger.groupId.trim()
        : "dev-group";
    return {
      topic,
      groupId,
      records: [{ key: "dev-key", value: { source: "runfabric-dev", topic, groupId }, partition: 0, offset: 1 }]
    };
  },
  rabbitmq(_provider: string, project: ProjectConfig): unknown {
    const trigger = project.triggers.find((item) => item.type === TriggerEnum.RabbitMq);
    const queue =
      typeof trigger?.queue === "string" && trigger.queue.trim().length > 0 ? trigger.queue.trim() : "jobs";
    const routingKey =
      typeof trigger?.routingKey === "string" && trigger.routingKey.trim().length > 0
        ? trigger.routingKey.trim()
        : "jobs.created";
    return {
      queue,
      routingKey,
      contentType: "application/json",
      body: { source: "runfabric-dev", queue, routingKey }
    };
  }
} satisfies Record<Exclude<DevPreset, "http">, (provider: string, project: ProjectConfig) => unknown>;

export function parsePreset(value: string | undefined, project: ProjectConfig): DevPreset {
  if (value) {
    const normalized = value.trim().toLowerCase();
    if (
      normalized === "http" ||
      normalized === "queue" ||
      normalized === "storage" ||
      normalized === "cron" ||
      normalized === "eventbridge" ||
      normalized === "pubsub" ||
      normalized === "kafka" ||
      normalized === "rabbitmq"
    ) {
      return normalized;
    }
    throw new Error(
      `unsupported preset: ${value}. expected one of: http, queue, storage, cron, eventbridge, pubsub, kafka, rabbitmq`
    );
  }

  const firstType = project.triggers[0]?.type;
  if (firstType === TriggerEnum.Queue) {
    return "queue";
  }
  if (firstType === TriggerEnum.Storage) {
    return "storage";
  }
  if (firstType === TriggerEnum.Cron) {
    return "cron";
  }
  if (firstType === TriggerEnum.EventBridge) {
    return "eventbridge";
  }
  if (firstType === TriggerEnum.PubSub) {
    return "pubsub";
  }
  if (firstType === TriggerEnum.Kafka) {
    return "kafka";
  }
  if (firstType === TriggerEnum.RabbitMq) {
    return "rabbitmq";
  }
  return "http";
}

export function defaultProvider(project: ProjectConfig, override?: string): string {
  return override || project.providers[0] || "aws-lambda";
}

export function createEventPresetEvent(
  provider: string,
  project: ProjectConfig,
  preset: Exclude<DevPreset, "http">
): unknown {
  return eventPresetBuilders[preset](provider, project);
}
