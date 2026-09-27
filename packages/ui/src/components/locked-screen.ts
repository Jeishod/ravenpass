import type { StorageStatus, UnlockMethods } from "../vault-api.ts";

/** `missing`: the vault file is gone from its place; `restore`: this device has no way in; `unlock`: it has one. */
export type LockedScreen = "missing" | "restore" | "unlock";

type WaysIn = Pick<
  UnlockMethods,
  "pinSet" | "biometryEnabled" | "biometryAvailable"
>;

export interface LockedActions {
  pin: boolean;
  deviceUnlock: boolean;
  openFile: boolean;
  /** The recovery key is the main way in, a secondary one, or not offered. */
  recovery: "primary" | "secondary" | null;
}

export function lockedScreen(
  storage: Pick<StorageStatus, "missing"> | null,
  methods: WaysIn | null,
): LockedScreen {
  if (storage?.missing) return "missing";
  if (methods === null) return "unlock";
  return methods.pinSet || deviceUnlock(methods) ? "unlock" : "restore";
}

/** A missing file offers only its reopening: every way in, the recovery key included, reads that file. */
export function lockedActions(
  screen: LockedScreen,
  methods: WaysIn | null,
): LockedActions {
  switch (screen) {
    case "missing":
      return {
        pin: false,
        deviceUnlock: false,
        openFile: true,
        recovery: null,
      };
    case "restore":
      return {
        pin: false,
        deviceUnlock: false,
        openFile: false,
        recovery: "primary",
      };
    case "unlock":
      return {
        pin: Boolean(methods?.pinSet),
        deviceUnlock: methods !== null && deviceUnlock(methods),
        openFile: false,
        recovery: "secondary",
      };
  }
}

function deviceUnlock(methods: WaysIn): boolean {
  return methods.biometryEnabled && methods.biometryAvailable;
}
