import { stdin, stdout } from "node:process";
import { createInterface } from "node:readline/promises";
import { info, warn } from "../../utils/logger";
import type { PromptOption } from "./prompt-option";
import { RawSelectionPrompt } from "./prompt-raw";

export function canPromptInteractively(): boolean {
  return Boolean(stdin.isTTY) && Boolean(stdout.isTTY);
}

function normalizePromptOptions(choices: readonly string[] | readonly PromptOption[]): PromptOption[] {
  return choices.map((choice) => {
    if (typeof choice === "string") {
      return { value: choice, label: choice };
    }
    return {
      value: choice.value,
      label: choice.label || choice.value,
      description: choice.description,
      group: choice.group,
      keywords: choice.keywords
    };
  });
}

async function promptWithReadline(question: string, options: readonly PromptOption[], defaultValue: string): Promise<string> {
  const rl = createInterface({ input: stdin, output: stdout });
  info(question);
  let previousGroup = "";
  for (let index = 0; index < options.length; index += 1) {
    const option = options[index];
    const group = option.group || "Options";
    if (group !== previousGroup) {
      info(`  ${group}`);
      previousGroup = group;
    }
    const detail = option.description ? ` - ${option.description}` : "";
    info(`  ${index + 1}. ${option.label || option.value}${detail}`);
  }

  const answer = (await rl.question(`Select [1-${options.length}] (default ${defaultValue}): `)).trim();
  await rl.close();
  if (!answer) {
    return defaultValue;
  }
  const asIndex = Number(answer);
  if (Number.isInteger(asIndex) && asIndex >= 1 && asIndex <= options.length) {
    return options[asIndex - 1].value;
  }
  const matchedOption = options.find((option) => option.value === answer || option.label === answer);
  if (matchedOption) {
    return matchedOption.value;
  }
  warn(`invalid selection "${answer}", using default "${defaultValue}"`);
  return defaultValue;
}

export async function promptSelection(
  question: string,
  choices: readonly string[] | readonly PromptOption[],
  defaultValue: string
): Promise<string> {
  if (!canPromptInteractively()) {
    return defaultValue;
  }
  const options = normalizePromptOptions(choices);
  const input = stdin as NodeJS.ReadStream;
  if (!input.isTTY || typeof input.setRawMode !== "function") {
    return promptWithReadline(question, options, defaultValue);
  }
  return new RawSelectionPrompt(question, options, defaultValue).run();
}
