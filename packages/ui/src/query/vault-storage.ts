import {
  keepPreviousData,
  queryOptions,
  useQuery,
} from "@tanstack/react-query";
import type { Language } from "../i18n/language.ts";
import type { MessageKey } from "../i18n/messages.ts";
import { useTranslator } from "../i18n/translator.tsx";
import type { VaultApi } from "../vault-api.ts";
import { queryKeys } from "./keys.ts";

/** The host words where each vault is kept in the language in use. */
export function vaultStorageQuery(
  api: Pick<VaultApi, "getStorage">,
  language: Language,
  failure: MessageKey,
) {
  return queryOptions({
    queryKey: queryKeys.storage(language),
    queryFn: () => api.getStorage(),
    placeholderData: keepPreviousData,
    refetchOnMount: "always",
    meta: { failure },
  });
}

export function useVaultStorage(
  api: Pick<VaultApi, "getStorage">,
  failure: MessageKey,
) {
  const { language } = useTranslator();
  return useQuery(vaultStorageQuery(api, language, failure));
}
