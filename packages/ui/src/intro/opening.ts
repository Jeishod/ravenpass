import type { VaultApi, VaultState } from "../vault-api.ts";

/** The vault's phase, or the introduction that comes before setup. */
export type OpeningPhase = VaultState["phase"] | "intro";

/** openingPhase opens setup on the introduction only when the device is known to hold no vault. */
export async function openingPhase(
  api: Pick<VaultApi, "getState" | "knowsVault">,
): Promise<OpeningPhase> {
  const { phase } = await api.getState();
  if (phase !== "setup") return phase;
  const knowsVault = await api.knowsVault().catch(() => true);
  return knowsVault ? "setup" : "intro";
}
