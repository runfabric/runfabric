export function printJson(value: unknown): void {
  console.log(JSON.stringify(value, null, 2));
}

export function printList(header: string, lines: string[]): void {
  console.log(header);
  for (const line of lines) {
    console.log(`- ${line}`);
  }
}
