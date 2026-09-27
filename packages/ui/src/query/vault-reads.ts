import {
  type QueryClient,
  type QueryKey,
  queryOptions,
} from "@tanstack/react-query";
import type { MessageKey } from "../i18n/messages.ts";
import type { Group, VaultApi } from "../vault-api.ts";
import { queryKeys } from "./keys.ts";

/** VaultRead is one read of the open vault, shown from the query cache. */
export class VaultRead<Data> {
  readonly key: QueryKey;
  readonly #read: () => Promise<Data>;
  readonly #failure: MessageKey;

  constructor(key: QueryKey, read: () => Promise<Data>, failure: MessageKey) {
    this.key = key;
    this.#read = read;
    this.#failure = failure;
  }

  /** options report a failed read through the query's failure message. */
  options() {
    return queryOptions({
      queryKey: this.key,
      queryFn: this.#read,
      meta: { failure: this.#failure },
    });
  }

  /** refresh reads again and rejects on failure without reporting it: the caller does. */
  async refresh(client: QueryClient): Promise<void> {
    const data = await this.#read();
    await client.cancelQueries({ queryKey: this.key, exact: true });
    client.setQueryData<Data>(this.key, data);
  }
}

export interface VaultGroups {
  groups: Group[];
  /** The group new items join; empty for none. */
  defaultGroup: string;
}

export function vaultReads(api: VaultApi) {
  return {
    credentials: new VaultRead(
      queryKeys.credentials,
      () => api.listCredentials(),
      "workspace.error.list",
    ),
    identities: new VaultRead(
      queryKeys.identities,
      () => api.listIdentities(),
      "identity.error.list",
    ),
    cards: new VaultRead(
      queryKeys.cards,
      () => api.listCards(),
      "card.error.list",
    ),
    notes: new VaultRead(
      queryKeys.notes,
      () => api.listNotes(),
      "note.error.list",
    ),
    seeds: new VaultRead(
      queryKeys.seeds,
      () => api.listSeeds(),
      "seed.error.list",
    ),
    groups: new VaultRead(
      queryKeys.groups,
      async (): Promise<VaultGroups> => {
        const [groups, defaultGroup] = await Promise.all([
          api.listGroups(),
          api.defaultGroup(),
        ]);
        return { groups, defaultGroup };
      },
      "workspace.error.read",
    ),
    codeSetup: new VaultRead(
      queryKeys.codeSetup,
      () => api.getCodeSetup(),
      "workspace.error.read",
    ),
  };
}
