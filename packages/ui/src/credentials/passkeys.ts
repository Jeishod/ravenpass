import { CalendarDate } from "../identities/dates.ts";
import type { PasskeyView } from "../vault-api.ts";

export function passkeyAccount(
  passkey: Pick<PasskeyView, "account" | "displayName">,
): string {
  return passkey.account || passkey.displayName;
}

/** The local calendar day. */
export function passkeyCreatedOn(
  passkey: Pick<PasskeyView, "createdAt">,
  language: string,
): string {
  return CalendarDate.of(new Date(passkey.createdAt)).format(language);
}
