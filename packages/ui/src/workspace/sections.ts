import { networkName } from "../cards/card.ts";
import type {
  CardSummary,
  CredentialSummary,
  IdentitySummary,
  NoteSummary,
  SeedSummary,
} from "../vault-api.ts";

export type WorkspaceSection = "all" | "recent" | "pinned";

export const recentLimit = 10;

/** No group chosen: the list is not narrowed to one. */
export const everyGroup = "";

/** What the list needs of an item of any kind. */
export interface ListedItem {
  id: string;
  label: string;
  pinned: boolean;
  lastUsedAt: number;
  groups: string[];
}

/** The values besides its label an item is found by, in the order they break label ties. */
export type SearchValues<Item> = (item: Item) => readonly string[];

export const credentialSearchValues: SearchValues<CredentialSummary> = (
  credential,
) => [credential.login, credential.email, ...credential.sites];

export const identitySearchValues: SearchValues<IdentitySummary> = (
  identity,
) => [identity.email];

export const cardSearchValues: SearchValues<CardSummary> = (card) => [
  card.bankName,
  card.lastFour,
  networkName(card.network),
];

export const noteSearchValues: SearchValues<NoteSummary> = (note) => [
  note.preview,
];

export const seedSearchValues: SearchValues<SeedSummary> = (seed) => [
  seed.wallet,
];

export function selectEntries<Item extends ListedItem>(
  entries: Item[],
  values: SearchValues<Item>,
  section: WorkspaceSection,
  query: string,
  group: string = everyGroup,
): Item[] {
  const matching = entries.filter(
    (entry) =>
      matchesQuery([entry.label, ...values(entry)], query) &&
      inGroup(entry, group),
  );
  if (section === "recent") {
    return matching
      .filter((entry) => entry.lastUsedAt > 0)
      .sort((left, right) => right.lastUsedAt - left.lastUsedAt)
      .slice(0, recentLimit);
  }
  const scoped =
    section === "pinned" ? matching.filter((entry) => entry.pinned) : matching;
  return scoped.sort((left, right) => byLabel(left, right, values));
}

/** The most items of one kind the command palette lists for a search. */
export const paletteLimit = 20;

/** paletteEntries lists the recent items for an empty query, else the best matches across sections and groups. */
export function paletteEntries<Item extends ListedItem>(
  entries: Item[],
  values: SearchValues<Item>,
  query: string,
): Item[] {
  if (!query.trim()) return selectEntries(entries, values, "recent", "");
  return selectEntries(entries, values, "all", query).slice(0, paletteLimit);
}

/** matchesQuery tells whether any value holds the query, ignoring case; an empty query matches all. */
export function matchesQuery(
  values: readonly string[],
  query: string,
): boolean {
  const term = query.trim().toLocaleLowerCase();
  if (!term) return true;
  return values.some((value) => value.toLocaleLowerCase().includes(term));
}

function inGroup(entry: ListedItem, group: string): boolean {
  return group === everyGroup || entry.groups.includes(group);
}

function byLabel<Item extends ListedItem>(
  left: Item,
  right: Item,
  values: SearchValues<Item>,
): number {
  const leftKeys = [left.label, ...values(left)];
  const rightKeys = [right.label, ...values(right)];
  for (const [index, key] of leftKeys.entries()) {
    const order = key.localeCompare(rightKeys[index] ?? "", undefined, {
      sensitivity: "base",
    });
    if (order !== 0) return order;
  }
  return left.id.localeCompare(right.id);
}

/** countInGroup counts the items of any kind a group holds. */
export function countInGroup(
  items: readonly { groups: string[] }[],
  group: string,
): number {
  return items.filter((item) => item.groups.includes(group)).length;
}
