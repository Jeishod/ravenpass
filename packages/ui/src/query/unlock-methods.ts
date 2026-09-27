import { queryOptions } from "@tanstack/react-query";
import type { VaultApi } from "../vault-api.ts";
import { queryKeys } from "./keys.ts";

export function unlockMethodsQuery(api: Pick<VaultApi, "unlockMethods">) {
  return queryOptions({
    queryKey: queryKeys.unlockMethods,
    queryFn: () => api.unlockMethods(),
    refetchOnMount: "always",
    meta: { failure: "unlock-methods.errors.unreadable" },
  });
}
