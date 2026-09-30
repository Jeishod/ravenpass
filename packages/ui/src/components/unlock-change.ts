import type { UnlockChoice, UnlockMethods } from "../vault-api.ts";

/** A change to the ways in the device is applying; turning device authentication on creates a hardware key, which is slow. */
export type UnlockPending = "biometry-on" | "other";

/** How the owner proves who they are before a way in changes on this device; the host holds the same rule. */
export type OwnerCheck = "device" | "pin" | "recovery-key";

/** Device authentication where the vault opens with it, else the current PIN, else the vault's recovery key. */
export function ownerCheck(methods: UnlockMethods): OwnerCheck {
  if (methods.biometryEnabled && methods.biometryAvailable) return "device";
  return methods.pinSet ? "pin" : "recovery-key";
}

/** The choice of no way in; committed in recovery, it keeps the ways in the device holds. */
export const noUnlockChoice: UnlockChoice = { biometry: false, pin: "" };

/** Whether committing `choice` creates a device authentication key: one chosen, or the kept one bound again. */
export function bindsBiometry(
  choice: UnlockChoice,
  kept: UnlockMethods | null,
): boolean {
  if (choice.biometry) return true;
  return choice.pin === "" && Boolean(kept?.biometryEnabled);
}
