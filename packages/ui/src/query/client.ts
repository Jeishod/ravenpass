import { QueryClient } from "@tanstack/react-query";
import type { MessageKey } from "../i18n/messages.ts";
import { vaultScope } from "./keys.ts";

/** FailureMeta names the toast a failed query or mutation shows; without it the caller reports the failure. */
interface FailureMeta extends Record<string, unknown> {
  failure?: MessageKey;
}

declare module "@tanstack/react-query" {
  interface Register {
    queryMeta: FailureMeta;
    mutationMeta: FailureMeta;
  }
}

/** Security: queries and mutations stay in memory only while shown (gcTime 0), and no persister exists. */
export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        gcTime: 0,
        staleTime: Number.POSITIVE_INFINITY,
        retry: false,
        refetchOnWindowFocus: false,
        refetchOnReconnect: false,
      },
      mutations: { gcTime: 0, retry: false },
    },
  });
}

/** Security: runs as the vault locks; a mutation's variables and function may hold a PIN or vault content. */
export function forgetVaultEdits(client: QueryClient): void {
  client.getMutationCache().clear();
}

/**
 * Runs once the screens that read the vault are gone: a removed query that a mounted screen renders again is fetched
 * again, from a vault that is locked by then.
 */
export function forgetVaultReads(client: QueryClient): void {
  client.removeQueries({ queryKey: vaultScope });
}
