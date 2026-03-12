import { stdin, stdout } from "node:process";
import { createInterface } from "node:readline/promises";
import { emitKeypressEvents } from "node:readline";
import { info, warn } from "../../utils/logger";

export function canPromptInteractively(): boolean {
  return Boolean(stdin.isTTY) && Boolean(stdout.isTTY);
}

async function promptWithReadline(question: string, choices: string[], defaultValue: string): Promise<string> {
  const rl = createInterface({ input: stdin, output: stdout });
  info(question);
  choices.forEach((choice, index) => {
    info(`  ${index + 1}. ${choice}`);
  });

  const answer = (await rl.question(`Select [1-${choices.length}] (default ${defaultValue}): `)).trim();
  await rl.close();
  if (!answer) {
    return defaultValue;
  }
  const asIndex = Number(answer);
  if (Number.isInteger(asIndex) && asIndex >= 1 && asIndex <= choices.length) {
    return choices[asIndex - 1];
  }
  if (choices.includes(answer)) {
    return answer;
  }
  warn(`invalid selection "${answer}", using default "${defaultValue}"`);
  return defaultValue;
}

function renderRawPrompt(
  output: NodeJS.WriteStream,
  question: string,
  choices: string[],
  selectedIndex: number,
  totalLines: number,
  firstRender: boolean
): void {
  if (!firstRender) {
    output.write(`\u001B[${totalLines}A`);
  }
  output.write("\u001B[0J");
  output.write(`${question}\n`);
  output.write("Use up/down arrows and Enter\n");
  for (let index = 0; index < choices.length; index += 1) {
    const prefix = index === selectedIndex ? ">" : " ";
    output.write(` ${prefix} ${choices[index]}\n`);
  }
}

async function promptWithRawMode(question: string, choices: string[], defaultValue: string): Promise<string> {
  const input = stdin as NodeJS.ReadStream;
  const output = stdout;
  let selectedIndex = Math.max(0, choices.indexOf(defaultValue));
  const totalLines = choices.length + 2;

  return new Promise<string>((resolvePromise, rejectPromise) => {
    const cleanup = (): void => {
      input.off("keypress", onKeypress);
      input.setRawMode(false);
      input.pause();
      output.write("\u001B[?25h");
      output.write("\n");
    };

    const onKeypress = (_str: string, key: { name?: string; ctrl?: boolean } | undefined): void => {
      if (!key) {
        return;
      }
      if (key.ctrl && key.name === "c") {
        cleanup();
        rejectPromise(new Error("prompt cancelled by user"));
        return;
      }
      if (key.name === "up" || key.name === "down") {
        selectedIndex =
          key.name === "up"
            ? (selectedIndex - 1 + choices.length) % choices.length
            : (selectedIndex + 1) % choices.length;
        renderRawPrompt(output, question, choices, selectedIndex, totalLines, false);
        return;
      }
      if (key.name === "return" || key.name === "enter") {
        const selected = choices[selectedIndex] || defaultValue;
        cleanup();
        resolvePromise(selected);
      }
    };

    emitKeypressEvents(input);
    input.setRawMode(true);
    input.resume();
    output.write("\u001B[?25l");
    renderRawPrompt(output, question, choices, selectedIndex, totalLines, true);
    input.on("keypress", onKeypress);
  });
}

export async function promptSelection(
  question: string,
  choices: string[],
  defaultValue: string
): Promise<string> {
  if (!canPromptInteractively()) {
    return defaultValue;
  }
  const input = stdin as NodeJS.ReadStream;
  if (!input.isTTY || typeof input.setRawMode !== "function") {
    return promptWithReadline(question, choices, defaultValue);
  }
  return promptWithRawMode(question, choices, defaultValue);
}
