const groupLength = 4;

/** A run of a connection key; `start` is its offset in the key. */
export interface KeyGroup {
  readonly start: number;
  readonly text: string;
}

/** The connection key in groups of four characters for reading; joined, the groups are the key. */
export function keyGroups(key: string): KeyGroup[] {
  const groups: KeyGroup[] = [];
  for (let start = 0; start < key.length; start += groupLength) {
    groups.push({ start, text: key.slice(start, start + groupLength) });
  }
  return groups;
}
