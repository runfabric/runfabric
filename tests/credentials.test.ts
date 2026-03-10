import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { evaluateCredentialSchema } from "../packages/core/src/credentials.ts";
import { createAlibabaFcProvider } from "../packages/provider-alibaba-fc/src/index.ts";
import { createAwsLambdaProvider } from "../packages/provider-aws-lambda/src/index.ts";
import { createAzureFunctionsProvider } from "../packages/provider-azure-functions/src/index.ts";
import { createCloudflareWorkersProvider } from "../packages/provider-cloudflare-workers/src/index.ts";
import { createDigitalOceanFunctionsProvider } from "../packages/provider-digitalocean-functions/src/index.ts";
import { createFlyMachinesProvider } from "../packages/provider-fly-machines/src/index.ts";
import { createGcpFunctionsProvider } from "../packages/provider-gcp-functions/src/index.ts";
import { createIbmOpenWhiskProvider } from "../packages/provider-ibm-openwhisk/src/index.ts";
import { createNetlifyProvider } from "../packages/provider-netlify/src/index.ts";
import { createVercelProvider } from "../packages/provider-vercel/src/index.ts";

test("evaluateCredentialSchema reports required and optional gaps", () => {
  const evaluation = evaluateCredentialSchema(
    {
      provider: "test-provider",
      fields: [
        { env: "REQUIRED_TOKEN", description: "Required token" },
        { env: "OPTIONAL_REGION", description: "Optional region", required: false }
      ]
    },
    {}
  );

  assert.equal(evaluation.missingRequired.length, 1);
  assert.equal(evaluation.missingRequired[0].env, "REQUIRED_TOKEN");
  assert.equal(evaluation.missingOptional.length, 1);
  assert.equal(evaluation.missingOptional[0].env, "OPTIONAL_REGION");
});

test("all providers expose required credential schemas", async () => {
  const projectDir = await mkdtemp(join(tmpdir(), "runfabric-credentials-"));
  const providers = [
    createAwsLambdaProvider({ projectDir }),
    createGcpFunctionsProvider({ projectDir }),
    createAzureFunctionsProvider({ projectDir }),
    createCloudflareWorkersProvider({ projectDir }),
    createVercelProvider({ projectDir }),
    createNetlifyProvider({ projectDir }),
    createAlibabaFcProvider({ projectDir }),
    createDigitalOceanFunctionsProvider({ projectDir }),
    createFlyMachinesProvider({ projectDir }),
    createIbmOpenWhiskProvider({ projectDir })
  ];

  for (const provider of providers) {
    const schema = provider.getCredentialSchema?.();
    assert.ok(schema, `${provider.name} should define a credential schema`);
    assert.equal(schema?.provider, provider.name);
    assert.ok((schema?.fields.length || 0) > 0, `${provider.name} should define credential fields`);

    const evaluation = evaluateCredentialSchema(schema!, {});
    assert.ok(evaluation.missingRequired.length > 0, `${provider.name} should require credentials`);
  }
});
