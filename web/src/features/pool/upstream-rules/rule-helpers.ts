export function parseNumbers(text: string): number[] {
  return text.split(/[\s,，]+/).map(s => s.trim()).filter(Boolean).map(Number);
}

export function validNumbers(list: number[], min: number, max: number) {
  return list.every(n => Number.isInteger(n) && n >= min && n <= max);
}

export function joinNumbers(list: readonly number[]) {
  return list.join(", ");
}
