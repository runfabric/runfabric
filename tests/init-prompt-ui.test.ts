import test from "node:test";
import assert from "node:assert/strict";
import { filterPromptOptionIndexes } from "../apps/cli/src/commands/init/prompt-filter";
import {
  languagePromptOptions,
  providerPromptOptions,
  stateBackendPromptOptions,
  templatePromptOptions
} from "../apps/cli/src/commands/init/prompt-options";
import {
  supportedProvidersForTemplate,
  supportedTemplatesForAnyProvider
} from "../apps/cli/src/commands/init/template-support";

test("init prompt templates are grouped for interactive picker", () => {
  const options = templatePromptOptions(supportedTemplatesForAnyProvider());
  const api = options.find((option) => option.value === "api");
  const queue = options.find((option) => option.value === "queue");
  const pubsub = options.find((option) => option.value === "pubsub");
  assert.equal(api?.group, "HTTP");
  assert.equal(queue?.group, "Event");
  assert.equal(pubsub?.group, "Event");
  assert.equal(api?.description, "GET /hello endpoint");
});

test("init prompt templates can be filtered by allowed template names", () => {
  const options = templatePromptOptions(["api", "worker"]);
  assert.deepEqual(
    options.map((option) => option.value),
    ["api", "worker"]
  );
});

test("init prompt provider options include grouped metadata", () => {
  const options = providerPromptOptions();
  assert.equal(options.length > 5, true);
  const aws = options.find((option) => option.value === "aws-lambda");
  const kubernetes = options.find((option) => option.value === "kubernetes");
  const cloudflare = options.find((option) => option.value === "cloudflare-workers");
  assert.equal(aws?.group, "Cloud");
  assert.equal(kubernetes?.group, "Container");
  assert.equal(cloudflare?.group, "Edge");
});

test("init prompt provider options can be filtered by allowed providers", () => {
  const options = providerPromptOptions(["aws-lambda", "gcp-functions"]);
  assert.deepEqual(
    options.map((option) => option.value),
    ["aws-lambda", "gcp-functions"]
  );
});

test("init template support returns provider-filtered list for queue template", () => {
  assert.deepEqual(supportedProvidersForTemplate("queue"), [
    "aws-lambda",
    "gcp-functions",
    "azure-functions",
    "alibaba-fc"
  ]);
});

test("init template support returns globally supported templates only", () => {
  assert.deepEqual(supportedTemplatesForAnyProvider(), [
    "api",
    "worker",
    "queue",
    "cron",
    "storage",
    "eventbridge",
    "pubsub"
  ]);
});

test("init prompt filter matches labels, descriptions, and keywords", () => {
  const stateOptions = stateBackendPromptOptions();
  const languageOptions = languagePromptOptions();
  const azblobMatch = filterPromptOptionIndexes(stateOptions, "blob");
  const tsMatch = filterPromptOptionIndexes(languageOptions, "type safety");
  assert.deepEqual(azblobMatch, [4]);
  assert.deepEqual(tsMatch, [0]);
});
