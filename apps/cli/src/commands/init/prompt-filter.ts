import type { PromptOption } from "./prompt-option";

export function buildPromptSearchText(option: PromptOption): string {
  const searchableParts = [
    option.value,
    option.label || "",
    option.description || "",
    option.group || "",
    ...(option.keywords || [])
  ];
  return searchableParts.join(" ").toLowerCase();
}

export function filterPromptOptionIndexes(options: readonly PromptOption[], query: string): number[] {
  const needle = query.trim().toLowerCase();
  if (!needle) {
    return options.map((_, index) => index);
  }
  const matches: number[] = [];
  for (let index = 0; index < options.length; index += 1) {
    if (buildPromptSearchText(options[index]).includes(needle)) {
      matches.push(index);
    }
  }
  return matches;
}
