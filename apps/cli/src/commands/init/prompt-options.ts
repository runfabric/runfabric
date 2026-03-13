import { PROVIDER_IDS, RUNTIME_FAMILIES } from "@runfabric/core";
import type { PromptOption } from "./prompt-option";
import type { InitTemplateName } from "./types";

const TEMPLATE_PROMPT_OPTIONS: PromptOption[] = [
  {
    value: "api",
    label: "api",
    group: "HTTP",
    description: "GET /hello endpoint",
    keywords: ["http", "rest", "endpoint", "web"]
  },
  {
    value: "worker",
    label: "worker",
    group: "HTTP",
    description: "POST /work endpoint",
    keywords: ["http", "background", "job", "webhook"]
  },
  {
    value: "queue",
    label: "queue",
    group: "Event",
    description: "Queue consumer (jobs)",
    keywords: ["event", "async", "consumer", "jobs"]
  },
  {
    value: "cron",
    label: "cron",
    group: "Event",
    description: "Scheduled task template",
    keywords: ["schedule", "timer", "periodic"]
  },
  {
    value: "storage",
    label: "storage",
    group: "Event",
    description: "Object storage event trigger",
    keywords: ["bucket", "s3", "gcs", "blob"]
  },
  {
    value: "eventbridge",
    label: "eventbridge",
    group: "Event",
    description: "EventBridge bus event trigger",
    keywords: ["aws", "event bus", "rule"]
  },
  {
    value: "pubsub",
    label: "pubsub",
    group: "Event",
    description: "Pub/Sub topic subscriber",
    keywords: ["gcp", "topic", "message"]
  },
  {
    value: "kafka",
    label: "kafka",
    group: "Streaming",
    description: "Kafka topic consumer",
    keywords: ["broker", "stream", "event"]
  },
  {
    value: "rabbitmq",
    label: "rabbitmq",
    group: "Messaging",
    description: "RabbitMQ queue consumer",
    keywords: ["amqp", "queue", "message"]
  }
];

export function templatePromptOptions(allowedTemplates?: readonly InitTemplateName[]): PromptOption[] {
  if (!allowedTemplates) {
    return TEMPLATE_PROMPT_OPTIONS;
  }
  const allowed = new Set(allowedTemplates);
  return TEMPLATE_PROMPT_OPTIONS.filter((option) =>
    allowed.has(option.value as InitTemplateName)
  );
}

export function providerPromptOptions(
  allowedProviders?: readonly (typeof PROVIDER_IDS)[number][]
): PromptOption[] {
  const metadata: Record<(typeof PROVIDER_IDS)[number], Omit<PromptOption, "value" | "label">> = {
    "aws-lambda": { group: "Cloud", description: "AWS Lambda", keywords: ["amazon", "aws"] },
    "gcp-functions": { group: "Cloud", description: "Google Cloud Functions", keywords: ["google", "gcp"] },
    "azure-functions": { group: "Cloud", description: "Azure Functions", keywords: ["microsoft", "azure"] },
    kubernetes: { group: "Container", description: "Kubernetes", keywords: ["k8s", "kubectl", "cluster"] },
    "cloudflare-workers": { group: "Edge", description: "Cloudflare Workers", keywords: ["edge", "cdn"] },
    vercel: { group: "Edge", description: "Vercel Functions", keywords: ["nextjs", "edge"] },
    netlify: { group: "Edge", description: "Netlify Functions", keywords: ["jamstack"] },
    "alibaba-fc": { group: "Other", description: "Alibaba Function Compute", keywords: ["alibaba", "cn"] },
    "digitalocean-functions": {
      group: "Other",
      description: "DigitalOcean Functions",
      keywords: ["do", "digitalocean"]
    },
    "fly-machines": { group: "Other", description: "Fly Machines", keywords: ["flyio"] },
    "ibm-openwhisk": { group: "Other", description: "IBM OpenWhisk", keywords: ["ibm", "openwhisk"] }
  };
  const options = PROVIDER_IDS.map((provider) => ({
    value: provider,
    label: provider,
    group: metadata[provider].group,
    description: metadata[provider].description,
    keywords: metadata[provider].keywords
  }));
  if (!allowedProviders) {
    return options;
  }
  const allowed = new Set(allowedProviders);
  return options.filter((option) =>
    allowed.has(option.value as (typeof PROVIDER_IDS)[number])
  );
}

export function languagePromptOptions(): PromptOption[] {
  return [
    {
      value: "ts",
      label: "ts",
      group: "Runtime",
      description: "TypeScript scaffold",
      keywords: ["typescript", "type safety"]
    },
    {
      value: "js",
      label: "js",
      group: "Runtime",
      description: "JavaScript scaffold",
      keywords: ["javascript", "plain js"]
    }
  ];
}

export function runtimePromptOptions(): PromptOption[] {
  return RUNTIME_FAMILIES.map((runtime) => ({
    value: runtime,
    label: runtime,
    group: runtime === "nodejs" ? "General" : "Additional",
    description: runtime === "nodejs" ? "Default runtime family" : `Use ${runtime} runtime family`
  }));
}

export function stateBackendPromptOptions(): PromptOption[] {
  return [
    {
      value: "local",
      label: "local",
      group: "File",
      description: "Project-local state file",
      keywords: ["local disk", "default"]
    },
    {
      value: "postgres",
      label: "postgres",
      group: "Database",
      description: "Shared Postgres table",
      keywords: ["sql", "rds", "database"]
    },
    {
      value: "s3",
      label: "s3",
      group: "Object Storage",
      description: "Amazon S3 state object",
      keywords: ["aws", "bucket"]
    },
    {
      value: "gcs",
      label: "gcs",
      group: "Object Storage",
      description: "Google Cloud Storage state object",
      keywords: ["google", "bucket"]
    },
    {
      value: "azblob",
      label: "azblob",
      group: "Object Storage",
      description: "Azure Blob Storage state object",
      keywords: ["azure", "blob"]
    }
  ];
}
