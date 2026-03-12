import { emitKeypressEvents } from "node:readline";
import { stdin, stdout } from "node:process";
import { filterPromptOptionIndexes } from "./prompt-filter";
import type { PromptOption } from "./prompt-option";

const PROMPT_HELP_TEXT =
  "Use Up/Down to move, type to filter, Backspace to edit, Esc to clear, Enter to select, Ctrl+C to cancel";

type Keypress = { name?: string; ctrl?: boolean; meta?: boolean } | undefined;

export class RawSelectionPrompt {
  private readonly input = stdin as NodeJS.ReadStream;
  private readonly output = stdout;
  private readonly selectedByDefault = Math.max(
    0,
    this.options.findIndex((option) => option.value === this.defaultValue)
  );
  private readonly onKeypress = (typed: string, key: Keypress): void => {
    this.handleKeypress(typed, key);
  };
  private filteredIndexes = this.options.map((_, index) => index);
  private query = "";
  private selectedIndex = this.filteredIndexes.indexOf(this.selectedByDefault);
  private renderedRowCount = 0;
  private resolvePromise: ((value: string) => void) | null = null;
  private rejectPromise: ((reason?: unknown) => void) | null = null;

  constructor(
    private readonly question: string,
    private readonly options: readonly PromptOption[],
    private readonly defaultValue: string
  ) {
    if (this.selectedIndex < 0) {
      this.selectedIndex = 0;
    }
  }

  run(): Promise<string> {
    return new Promise<string>((resolvePromise, rejectPromise) => {
      this.resolvePromise = resolvePromise;
      this.rejectPromise = rejectPromise;
      emitKeypressEvents(this.input);
      this.input.setRawMode(true);
      this.input.resume();
      this.output.write("\u001B[?25l");
      this.input.on("keypress", this.onKeypress);
      this.render();
    });
  }

  private finish(value: string): void {
    this.cleanup();
    this.resolvePromise?.(value);
  }

  private cancel(): void {
    this.cleanup();
    this.rejectPromise?.(new Error("prompt cancelled by user"));
  }

  private cleanup(): void {
    this.input.off("keypress", this.onKeypress);
    this.input.setRawMode(false);
    this.input.pause();
    this.output.write("\u001B[?25h");
    this.output.write("\n");
  }

  private handleKeypress(typed: string, key: Keypress): void {
    if (!key) {
      return;
    }
    if (key.ctrl && key.name === "c") {
      this.cancel();
      return;
    }
    if (key.name === "up" || key.name === "down") {
      this.rotateSelection(key.name === "up" ? -1 : 1);
      this.render();
      return;
    }
    if (key.name === "return" || key.name === "enter") {
      const selection = this.currentSelection()?.value || this.defaultValue;
      this.finish(selection);
      return;
    }
    if (key.name === "escape") {
      if (this.query.length > 0) {
        this.query = "";
        this.refilter();
        this.render();
      }
      return;
    }
    if (key.name === "backspace") {
      if (this.query.length > 0) {
        this.query = this.query.slice(0, -1);
        this.refilter();
        this.render();
      }
      return;
    }
    if (!key.ctrl && !key.meta && typed && typed >= " " && typed !== "\u007f") {
      this.query += typed;
      this.refilter();
      this.render();
    }
  }

  private rotateSelection(direction: number): void {
    if (this.filteredIndexes.length === 0) {
      return;
    }
    this.selectedIndex = (this.selectedIndex + direction + this.filteredIndexes.length) % this.filteredIndexes.length;
  }

  private refilter(): void {
    const previousValue = this.currentSelection()?.value;
    this.filteredIndexes = filterPromptOptionIndexes(this.options, this.query);
    if (this.filteredIndexes.length === 0) {
      this.selectedIndex = 0;
      return;
    }
    if (previousValue) {
      const previousOptionIndex = this.options.findIndex((option) => option.value === previousValue);
      const nextPosition = this.filteredIndexes.indexOf(previousOptionIndex);
      if (nextPosition >= 0) {
        this.selectedIndex = nextPosition;
        return;
      }
    }
    if (this.selectedIndex >= this.filteredIndexes.length) {
      this.selectedIndex = this.filteredIndexes.length - 1;
      return;
    }
    if (this.selectedIndex < 0) {
      this.selectedIndex = 0;
    }
  }

  private currentSelection(): PromptOption | null {
    const currentIndex = this.filteredIndexes[this.selectedIndex];
    if (currentIndex === undefined) {
      return null;
    }
    return this.options[currentIndex] || null;
  }

  private render(): void {
    if (this.renderedRowCount > 0) {
      this.output.write(`\u001B[${this.renderedRowCount}A`);
    }
    this.output.write("\u001B[0J");
    const lines = this.renderLines();
    for (const line of lines) {
      this.output.write(`${line}\n`);
    }
    this.renderedRowCount = this.countRenderedRows(lines);
  }

  private countRenderedRows(lines: readonly string[]): number {
    const columns = Math.max(1, this.output.columns || 80);
    let totalRows = 0;
    for (const line of lines) {
      totalRows += this.rowsForLine(line, columns);
    }
    return totalRows;
  }

  private rowsForLine(line: string, columns: number): number {
    const textLength = [...line].length;
    if (textLength === 0) {
      return 1;
    }
    return Math.ceil(textLength / columns);
  }

  private renderLines(): string[] {
    const lines = [
      this.question,
      PROMPT_HELP_TEXT,
      `Search: ${this.query || "(all)"} (${this.filteredIndexes.length}/${this.options.length})`
    ];
    if (this.filteredIndexes.length === 0) {
      lines.push("  No matches. Keep typing or press Esc to clear the filter.");
      return lines;
    }
    let previousGroup = "";
    for (let index = 0; index < this.filteredIndexes.length; index += 1) {
      const option = this.options[this.filteredIndexes[index]];
      const group = option.group || "Options";
      if (group !== previousGroup) {
        lines.push(`  ${group}`);
        previousGroup = group;
      }
      const marker = index === this.selectedIndex ? ">" : " ";
      const detail = option.description ? ` - ${option.description}` : "";
      lines.push(` ${marker} ${option.label || option.value}${detail}`);
    }
    return lines;
  }
}
