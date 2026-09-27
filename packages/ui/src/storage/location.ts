import type { StorageStatus } from "../vault-api.ts";

/** The file name without its extension. */
export function vaultName(file: string): string {
  const extension = file.lastIndexOf(".");
  return extension > 0 ? file.slice(0, extension) : file;
}

export function vaultLocation({
  name,
  place,
}: Pick<StorageStatus, "name" | "place">): string {
  return place ? `${place} › ${name}` : name;
}
