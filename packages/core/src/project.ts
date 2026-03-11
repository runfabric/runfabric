import type {
  AwsIamEffectEnum,
  AwsQueueFunctionResponseTypeEnum,
  TriggerEnum
} from "./enums";

export interface ProjectResources {
  memory?: number;
  timeout?: number;
  queues?: QueueResourceConfig[];
  buckets?: BucketResourceConfig[];
  topics?: TopicResourceConfig[];
  databases?: DatabaseResourceConfig[];
}

export interface TriggerConfig {
  type: TriggerEnum;
  method?: string;
  path?: string;
  schedule?: string;
  timezone?: string;
  queue?: string;
  bus?: string;
  pattern?: Record<string, unknown>;
  topic?: string;
  subscription?: string;
  brokers?: string[];
  groupId?: string;
  exchange?: string;
  routingKey?: string;
  batchSize?: number;
  maximumBatchingWindowSeconds?: number;
  maximumConcurrency?: number;
  enabled?: boolean;
  functionResponseType?: AwsQueueFunctionResponseTypeEnum;
  bucket?: string;
  events?: string[];
  prefix?: string;
  suffix?: string;
  existingBucket?: boolean;
  [key: string]: unknown;
}

export interface AwsIamRoleStatement {
  sid?: string;
  effect: AwsIamEffectEnum;
  actions: string[];
  resources: string[];
  condition?: Record<string, unknown>;
}

export interface AwsIamConfig {
  role?: {
    statements?: AwsIamRoleStatement[];
  };
}

export interface AwsLambdaExtensionConfig {
  stage?: string;
  region?: string;
  roleArn?: string;
  functionName?: string;
  runtime?: string;
  iam?: AwsIamConfig;
  [key: string]: unknown;
}

export interface ProjectExtensions {
  "aws-lambda"?: AwsLambdaExtensionConfig;
  [provider: string]: Record<string, unknown> | AwsLambdaExtensionConfig | undefined;
}

export type StateBackendType = "local" | "postgres" | "s3" | "gcs" | "azblob";

export interface StateLockConfig {
  enabled?: boolean;
  timeoutSeconds?: number;
  heartbeatSeconds?: number;
  staleAfterSeconds?: number;
}

export interface LocalStateConfig {
  dir?: string;
}

export interface PostgresStateConfig {
  connectionStringEnv?: string;
  schema?: string;
  table?: string;
}

export interface S3StateConfig {
  bucket?: string;
  region?: string;
  keyPrefix?: string;
  useLockfile?: boolean;
}

export interface GcsStateConfig {
  bucket?: string;
  prefix?: string;
}

export interface AzBlobStateConfig {
  container?: string;
  prefix?: string;
}

export interface ProjectStateConfig {
  backend?: StateBackendType;
  keyPrefix?: string;
  lock?: StateLockConfig;
  local?: LocalStateConfig;
  postgres?: PostgresStateConfig;
  s3?: S3StateConfig;
  gcs?: GcsStateConfig;
  azblob?: AzBlobStateConfig;
}

export interface FunctionConfig {
  name: string;
  entry?: string;
  runtime?: string;
  triggers?: TriggerConfig[];
  resources?: ProjectResources;
  env?: Record<string, string>;
}

export interface QueueResourceConfig {
  name: string;
  fifo?: boolean;
  dlq?: string;
  visibilityTimeoutSeconds?: number;
  [key: string]: unknown;
}

export interface BucketResourceConfig {
  name: string;
  versioning?: boolean;
  public?: boolean;
  [key: string]: unknown;
}

export interface TopicResourceConfig {
  name: string;
  fifo?: boolean;
  [key: string]: unknown;
}

export interface DatabaseResourceConfig {
  name: string;
  engine?: string;
  version?: string;
  [key: string]: unknown;
}

export interface WorkflowRetryConfig {
  attempts?: number;
  backoffSeconds?: number;
}

export interface WorkflowStepConfig {
  function: string;
  next?: string;
  retry?: WorkflowRetryConfig;
  timeoutSeconds?: number;
}

export interface WorkflowConfig {
  name: string;
  steps: WorkflowStepConfig[];
}

export interface ProjectConfig {
  service: string;
  runtime: string;
  entry: string;
  stage?: string;
  providers: string[];
  triggers: TriggerConfig[];
  functions?: FunctionConfig[];
  hooks?: string[];
  resources?: ProjectResources;
  env?: Record<string, string>;
  secrets?: Record<string, string>;
  workflows?: WorkflowConfig[];
  params?: Record<string, string>;
  extensions?: ProjectExtensions;
  state?: ProjectStateConfig;
}
