import type { LinkedApp } from "../vault-api.ts";

/** An app signed with two certificates has two keys. */
export function appKey(app: Pick<LinkedApp, "package" | "signer">): string {
  return `${app.package}:${app.signer}`;
}
