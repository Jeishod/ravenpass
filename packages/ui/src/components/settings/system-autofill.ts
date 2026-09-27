import type { SystemAutofillStatus } from "../../vault-api.ts";

/** The device's service that fills in passwords and codes, or its passkey provider. */
export type SystemAutofillRole = "autofill" | "passkeys";

export interface SystemAutofillRoleState {
  role: SystemAutofillRole;
  on: boolean;
}

/** The autofill role always, passkeys only where the device takes passkey providers. */
export function systemAutofillRoles(
  status: SystemAutofillStatus,
): SystemAutofillRoleState[] {
  const roles: SystemAutofillRoleState[] = [
    { role: "autofill", on: status.autofill },
  ];
  if (status.passkeyProviders) {
    roles.push({ role: "passkeys", on: status.passkeys });
  }
  return roles;
}
