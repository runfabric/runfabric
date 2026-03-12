import test from "node:test";
import assert from "node:assert/strict";
import { filterPromptOptionIndexes } from "../apps/cli/src/commands/init/prompt-filter";
import {
  languagePromptOptions,
  providerPromptOptions,
  stateBackendPromptOptions,
  templatePromptOptions
} from "../apps/cli/src/commands/init/prompt-options";

test("init prompt templates are grouped for interactive picker", () => {
  const options = templatePromptOptions();
  const api = options.find((option) => option.value === "api");
  const queue = options.find((option) => option.value === "queue");
  assert.equal(api?.group, "HTTP");
  assert.equal(queue?.group, "Event");
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
  const cloudflare = options.find((option) => option.value === "cloudflare-workers");
  assert.equal(aws?.group, "Cloud");
  assert.equal(cloudflare?.group, "Edge");
});

test("init prompt filter matches labels, descriptions, and keywords", () => {
  const stateOptions = stateBackendPromptOptions();
  const languageOptions = languagePromptOptions();
  const azblobMatch = filterPromptOptionIndexes(stateOptions, "blob");
  const tsMatch = filterPromptOptionIndexes(languageOptions, "type safety");
  assert.deepEqual(azblobMatch, [4]);
  assert.deepEqual(tsMatch, [0]);
});
