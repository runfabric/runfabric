import { PROVIDER_IDS } from "@runfabric/core";

const triggerTypes = [
  "http",
  "cron",
  "queue",
  "storage",
  "eventbridge",
  "pubsub",
  "kafka",
  "rabbitmq"
];

const stringMapSchema = {
  type: "object",
  additionalProperties: {
    type: "string"
  }
};

const runfabricSchema: Record<string, unknown> = {
  $schema: "https://json-schema.org/draft/2020-12/schema",
  $id: "https://runfabric.dev/schema/runfabric.schema.json",
  title: "RunFabric Config",
  description: "Schema for runfabric.yml",
  type: "object",
  additionalProperties: true,
  required: ["service", "runtime", "entry", "providers", "triggers"],
  properties: {
    service: { type: "string", minLength: 1, description: "Service name" },
    runtime: { type: "string", examples: ["nodejs"], description: "Runtime identifier" },
    entry: { type: "string", minLength: 1, description: "Handler entry file path" },
    stage: { type: "string", minLength: 1 },
    providers: {
      type: "array",
      minItems: 1,
      items: { type: "string", enum: [...PROVIDER_IDS] }
    },
    triggers: {
      type: "array",
      minItems: 1,
      items: { $ref: "#/$defs/trigger" }
    },
    functions: {
      type: "array",
      items: { $ref: "#/$defs/functionConfig" }
    },
    hooks: {
      type: "array",
      items: { type: "string" }
    },
    resources: { $ref: "#/$defs/resources" },
    env: stringMapSchema,
    params: stringMapSchema,
    secrets: {
      type: "object",
      additionalProperties: {
        type: "string",
        pattern: "^secret://.+"
      }
    },
    workflows: {
      type: "array",
      items: { $ref: "#/$defs/workflow" }
    },
    extensions: {
      type: "object",
      additionalProperties: { type: "object", additionalProperties: true },
      properties: {
        "aws-lambda": { $ref: "#/$defs/awsLambdaExtension" },
        kubernetes: { $ref: "#/$defs/kubernetesExtension" }
      }
    },
    state: { $ref: "#/$defs/stateConfig" },
    stages: {
      type: "object",
      additionalProperties: { $ref: "#/$defs/stageOverride" }
    }
  },
  $defs: {
    trigger: {
      type: "object",
      additionalProperties: true,
      properties: {
        type: { type: "string", enum: triggerTypes },
        method: { type: "string", description: "HTTP method" },
        path: { type: "string", description: "HTTP path" },
        schedule: { type: "string", description: "Cron schedule expression" },
        timezone: { type: "string" },
        queue: { type: "string" },
        bus: { type: "string" },
        pattern: { type: "object", additionalProperties: true },
        topic: { type: "string" },
        subscription: { type: "string" },
        brokers: { type: "array", items: { type: "string" } },
        groupId: { type: "string" },
        exchange: { type: "string" },
        routingKey: { type: "string" },
        batchSize: { type: "number", minimum: 0 },
        maximumBatchingWindowSeconds: { type: "number", minimum: 0 },
        maximumConcurrency: { type: "number", minimum: 0 },
        enabled: { type: "boolean" },
        functionResponseType: { type: "string", enum: ["ReportBatchItemFailures"] },
        bucket: { type: "string" },
        events: { type: "array", items: { type: "string" } },
        prefix: { type: "string" },
        suffix: { type: "string" },
        existingBucket: { type: "boolean" }
      },
      required: ["type"]
    },
    namedResource: {
      type: "object",
      additionalProperties: true,
      properties: {
        name: { type: "string", minLength: 1 }
      },
      required: ["name"]
    },
    resources: {
      type: "object",
      additionalProperties: false,
      properties: {
        memory: { type: "number" },
        timeout: { type: "number" },
        queues: { type: "array", items: { $ref: "#/$defs/namedResource" } },
        buckets: { type: "array", items: { $ref: "#/$defs/namedResource" } },
        topics: { type: "array", items: { $ref: "#/$defs/namedResource" } },
        databases: { type: "array", items: { $ref: "#/$defs/namedResource" } }
      }
    },
    functionConfig: {
      type: "object",
      additionalProperties: false,
      properties: {
        name: { type: "string", minLength: 1 },
        entry: { type: "string" },
        runtime: { type: "string" },
        triggers: { type: "array", minItems: 1, items: { $ref: "#/$defs/trigger" } },
        resources: { $ref: "#/$defs/resources" },
        env: stringMapSchema
      },
      required: ["name"]
    },
    workflowStep: {
      type: "object",
      additionalProperties: false,
      properties: {
        function: { type: "string", minLength: 1 },
        next: { type: "string" },
        retry: {
          type: "object",
          additionalProperties: false,
          properties: {
            attempts: { type: "number", minimum: 1 },
            backoffSeconds: { type: "number", minimum: 0 }
          }
        },
        timeoutSeconds: { type: "number", minimum: 1 }
      },
      required: ["function"]
    },
    workflow: {
      type: "object",
      additionalProperties: false,
      properties: {
        name: { type: "string", minLength: 1 },
        steps: { type: "array", minItems: 1, items: { $ref: "#/$defs/workflowStep" } }
      },
      required: ["name", "steps"]
    },
    stateConfig: {
      type: "object",
      additionalProperties: false,
      properties: {
        backend: { type: "string", enum: ["local", "postgres", "s3", "gcs", "azblob"] },
        keyPrefix: { type: "string" },
        lock: {
          type: "object",
          additionalProperties: false,
          properties: {
            enabled: { type: "boolean" },
            timeoutSeconds: { type: "number", minimum: 1 },
            heartbeatSeconds: { type: "number", minimum: 1 },
            staleAfterSeconds: { type: "number", minimum: 1 }
          }
        },
        local: {
          type: "object",
          additionalProperties: false,
          properties: {
            dir: { type: "string" }
          }
        },
        postgres: {
          type: "object",
          additionalProperties: false,
          properties: {
            connectionStringEnv: { type: "string" },
            schema: { type: "string" },
            table: { type: "string" }
          }
        },
        s3: {
          type: "object",
          additionalProperties: false,
          properties: {
            bucket: { type: "string" },
            region: { type: "string" },
            keyPrefix: { type: "string" },
            useLockfile: { type: "boolean" }
          }
        },
        gcs: {
          type: "object",
          additionalProperties: false,
          properties: {
            bucket: { type: "string" },
            prefix: { type: "string" }
          }
        },
        azblob: {
          type: "object",
          additionalProperties: false,
          properties: {
            container: { type: "string" },
            prefix: { type: "string" }
          }
        }
      }
    },
    stageOverride: {
      type: "object",
      additionalProperties: true,
      properties: {
        runtime: { type: "string" },
        entry: { type: "string" },
        providers: {
          type: "array",
          minItems: 1,
          items: { type: "string", enum: [...PROVIDER_IDS] }
        },
        triggers: {
          type: "array",
          minItems: 1,
          items: { $ref: "#/$defs/trigger" }
        },
        functions: {
          type: "array",
          items: { $ref: "#/$defs/functionConfig" }
        },
        hooks: { type: "array", items: { type: "string" } },
        resources: { $ref: "#/$defs/resources" },
        env: stringMapSchema,
        params: stringMapSchema,
        secrets: {
          type: "object",
          additionalProperties: {
            type: "string",
            pattern: "^secret://.+"
          }
        },
        workflows: {
          type: "array",
          items: { $ref: "#/$defs/workflow" }
        },
        extensions: {
          type: "object",
          additionalProperties: { type: "object", additionalProperties: true }
        },
        state: { $ref: "#/$defs/stateConfig" }
      }
    },
    awsIamStatement: {
      type: "object",
      additionalProperties: false,
      properties: {
        sid: { type: "string" },
        effect: { type: "string", enum: ["Allow", "Deny"] },
        actions: { type: "array", items: { type: "string" } },
        resources: { type: "array", items: { type: "string" } },
        condition: { type: "object", additionalProperties: true }
      },
      required: ["effect", "actions", "resources"]
    },
    awsLambdaExtension: {
      type: "object",
      additionalProperties: true,
      properties: {
        stage: { type: "string" },
        region: { type: "string" },
        roleArn: { type: "string" },
        functionName: { type: "string" },
        runtime: { type: "string" },
        iam: {
          type: "object",
          additionalProperties: false,
          properties: {
            role: {
              type: "object",
              additionalProperties: false,
              properties: {
                statements: { type: "array", items: { $ref: "#/$defs/awsIamStatement" } }
              }
            }
          }
        }
      }
    },
    kubernetesExtension: {
      type: "object",
      additionalProperties: false,
      properties: {
        namespace: { type: "string" },
        context: { type: "string" },
        deploymentName: { type: "string" },
        serviceName: { type: "string" },
        ingressHost: { type: "string" }
      }
    }
  }
};

export const RUNFABRIC_SCHEMA_RELATIVE_PATH = "node_modules/@runfabric/core/runfabric.schema.json";
export const RUNFABRIC_YAML_SCHEMA_DIRECTIVE = `# yaml-language-server: $schema=./${RUNFABRIC_SCHEMA_RELATIVE_PATH}`;

export function buildRunfabricSchemaContent(): string {
  return `${JSON.stringify(runfabricSchema)}\n`;
}
