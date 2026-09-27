/** initial is the label's first letter or digit, uppercased, or empty when it has none. */
export function initial(label: string): string {
  for (const character of label.trim()) {
    if (/[\p{L}\p{N}]/u.test(character)) return character.toLocaleUpperCase();
  }
  return "";
}
