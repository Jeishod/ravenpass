import type { VirtualItem } from "@tanstack/react-virtual";
import type { WorkspaceSection } from "./sections.ts";

/** `measured` restores the virtualizer's row sizes so `offset` lands on the same rows. */
export interface ListPosition {
  offset: number;
  measured: VirtualItem[];
}

/** Two lists with the same `key` show the same listing. */
export interface ListingPosition {
  readonly key: string;
  recall(): ListPosition;
  remember(position: ListPosition): void;
}

interface LeftPosition extends ListPosition {
  query: string;
}

/** ListPositions keeps each listing's scroll position; a position is recalled only under the same query. */
export class ListPositions {
  readonly #left = new Map<string, LeftPosition>();

  /** listing answers the listing of `place` in `section` and `group`, searched for `query`. */
  listing(
    place: string,
    section: WorkspaceSection,
    group: string,
    query: string,
  ): ListingPosition {
    const key = JSON.stringify([place, section, group]);
    return {
      key,
      recall: () => {
        const left = this.#left.get(key);
        if (!left || left.query !== query) return { offset: 0, measured: [] };
        return { offset: left.offset, measured: left.measured };
      },
      remember: ({ offset, measured }) => {
        if (offset > 0) this.#left.set(key, { offset, measured, query });
        else this.#left.delete(key);
      },
    };
  }
}
