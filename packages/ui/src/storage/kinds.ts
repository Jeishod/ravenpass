import type { MessageKey } from "../i18n/messages.ts";
import type { StorageKind, StorageStatus } from "../vault-api.ts";

/** `unrestricted`: the file's location cannot limit who reads it; `concurrent`: another device may write it at once. */
export type StorageWarning = "unrestricted" | "concurrent";

/** A local file reads differently where the owner picks it than where the host keeps it. */
export function storageKindDetail(
  kind: StorageKind,
  chosen: readonly StorageKind[],
): MessageKey {
  if (kind === "document") return "storage.type.document.detail";
  return chosen.includes(kind)
    ? "storage.type.local-file.detail"
    : "storage.type.local-file.private";
}

export function storageWarnings(
  status: Pick<StorageStatus, "kind" | "restricted">,
): StorageWarning[] {
  if (status.kind === "document") return ["concurrent"];
  if (status.kind === "local-file" && !status.restricted) {
    return ["unrestricted"];
  }
  return [];
}
