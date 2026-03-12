import { parseScalar } from "./shared";

interface ParsedLine {
  indent: number;
  content: string;
  line: number;
}

interface YamlKeyValue {
  key: string;
  hasValue: boolean;
  value: string;
}

function parseInlineKeyValue(input: string): YamlKeyValue | null {
  let separator = -1;
  for (let index = 0; index < input.length; index += 1) {
    if (input[index] !== ":") {
      continue;
    }
    const next = input[index + 1];
    if (next === undefined || /\s/.test(next)) {
      separator = index;
      break;
    }
  }
  if (separator <= 0) {
    return null;
  }

  const key = input.slice(0, separator).trim();
  if (!key) {
    return null;
  }

  const valuePart = input.slice(separator + 1);
  return {
    key,
    hasValue: valuePart.trim().length > 0,
    value: valuePart.trim()
  };
}

function prepareLines(content: string): ParsedLine[] {
  const parsed: ParsedLine[] = [];
  const rawLines = content.split(/\r?\n/);

  for (let index = 0; index < rawLines.length; index += 1) {
    const rawLine = rawLines[index];
    const trimmed = rawLine.trim();
    if (!trimmed || trimmed.startsWith("#")) {
      continue;
    }

    const indent = rawLine.match(/^ */)?.[0].length || 0;
    if (indent % 2 !== 0) {
      throw new Error(`Invalid runfabric.yml: line ${index + 1} uses non-2-space indentation`);
    }

    parsed.push({ indent, content: trimmed, line: index + 1 });
  }

  return parsed;
}

function parseObject(lines: ParsedLine[], state: { index: number }, indent: number): Record<string, unknown> {
  const object: Record<string, unknown> = {};

  while (state.index < lines.length) {
    const line = lines[state.index];
    if (line.indent < indent) {
      break;
    }
    if (line.indent > indent) {
      throw new Error(`Invalid runfabric.yml: unexpected indentation at line ${line.line}`);
    }
    if (line.content.startsWith("-")) {
      break;
    }

    const keyValue = parseInlineKeyValue(line.content);
    if (!keyValue) {
      throw new Error(`Invalid runfabric.yml: expected key/value at line ${line.line}`);
    }

    if (keyValue.hasValue) {
      object[keyValue.key] = parseScalar(keyValue.value);
      state.index += 1;
      continue;
    }

    state.index += 1;
    if (state.index >= lines.length || lines[state.index].indent <= indent) {
      object[keyValue.key] = {};
      continue;
    }
    object[keyValue.key] = parseNode(lines, state, lines[state.index].indent);
  }

  return object;
}

function parseEmptyArrayItem(lines: ParsedLine[], state: { index: number }, indent: number): unknown {
  state.index += 1;
  if (state.index >= lines.length || lines[state.index].indent <= indent) {
    return {};
  }
  return parseNode(lines, state, lines[state.index].indent);
}

function parseInlineArrayObject(
  lines: ParsedLine[],
  state: { index: number },
  indent: number,
  inlineKeyValue: YamlKeyValue
): Record<string, unknown> {
  const item: Record<string, unknown> = {};

  if (inlineKeyValue.hasValue) {
    item[inlineKeyValue.key] = parseScalar(inlineKeyValue.value);
    state.index += 1;
  } else {
    state.index += 1;
    if (state.index >= lines.length || lines[state.index].indent <= indent) {
      item[inlineKeyValue.key] = {};
    } else {
      item[inlineKeyValue.key] = parseNode(lines, state, lines[state.index].indent);
    }
  }

  if (state.index < lines.length && lines[state.index].indent > indent) {
    const continuation = parseObject(lines, state, indent + 2);
    for (const [key, value] of Object.entries(continuation)) {
      item[key] = value;
    }
  }

  return item;
}

function parseArrayItem(
  lines: ParsedLine[],
  state: { index: number },
  indent: number,
  line: ParsedLine
): unknown {
  const rest = line.content.slice(1).trimStart();
  if (!rest) {
    return parseEmptyArrayItem(lines, state, indent);
  }

  const inlineKeyValue = parseInlineKeyValue(rest);
  if (!inlineKeyValue) {
    state.index += 1;
    return parseScalar(rest);
  }

  return parseInlineArrayObject(lines, state, indent, inlineKeyValue);
}

function parseArray(lines: ParsedLine[], state: { index: number }, indent: number): unknown[] {
  const values: unknown[] = [];

  while (state.index < lines.length) {
    const line = lines[state.index];
    if (line.indent < indent) {
      break;
    }
    if (line.indent > indent) {
      throw new Error(`Invalid runfabric.yml: unexpected indentation at line ${line.line}`);
    }
    if (!line.content.startsWith("-")) {
      break;
    }

    values.push(parseArrayItem(lines, state, indent, line));
  }

  return values;
}

function parseNode(lines: ParsedLine[], state: { index: number }, indent: number): unknown {
  const line = lines[state.index];
  if (!line) {
    return {};
  }
  if (line.indent !== indent) {
    throw new Error(`Invalid runfabric.yml: invalid indentation near line ${line.line}`);
  }
  return line.content.startsWith("-") ? parseArray(lines, state, indent) : parseObject(lines, state, indent);
}

export function parseYamlDocument(content: string): unknown {
  const lines = prepareLines(content);
  if (lines.length === 0) {
    return {};
  }

  const state = { index: 0 };
  const root = parseNode(lines, state, lines[0].indent);
  if (state.index !== lines.length) {
    const unparsed = lines[state.index];
    throw new Error(`Invalid runfabric.yml: could not parse line ${unparsed.line}`);
  }

  return root;
}
