import type { UnlockMethods } from "../vault-api.ts";
import type { AutofillApi, UnlockOpening } from "./autofill-api.ts";
import { Watched } from "./watched.ts";

/** Why the last attempt left the vault locked. */
export type UnlockNote = "wrong-pin" | "pin-removed" | "too-soon" | "failed";

/** The way in being tried. */
export type UnlockTry = "pin" | "device";

export interface UnlockScreenView {
  readonly methods: UnlockMethods;
  readonly pin: string;
  readonly trying: UnlockTry | null;
  readonly note: UnlockNote | null;
  /** Counts the PINs that left the vault locked, each one cleared from the field. */
  readonly rejections: number;
}

/** UnlockScreen runs the autofill unlock screen; a device unlock the owner turned down is no failure. */
export class UnlockScreen {
  readonly state: Watched<UnlockScreenView>;
  readonly #unlock: AutofillApi["unlock"];

  constructor(opening: UnlockOpening, unlock: AutofillApi["unlock"]) {
    this.#unlock = unlock;
    this.state = new Watched<UnlockScreenView>({
      methods: opening.methods,
      pin: "",
      trying: null,
      note: null,
      rejections: 0,
    });
  }

  get biometry(): boolean {
    const { methods } = this.state.get();
    return methods.biometryEnabled && methods.biometryAvailable;
  }

  /** Whether the typed PIN is long enough to try. */
  get pinReady(): boolean {
    const { methods, pin } = this.state.get();
    return methods.pinSet && pin.length >= methods.pinMinLength;
  }

  enter(pin: string): void {
    this.state.set({ ...this.state.get(), pin });
  }

  submit(): Promise<void> {
    return this.pinReady
      ? this.#attempt(this.state.get().pin)
      : Promise.resolve();
  }

  unlockWithDevice(): Promise<void> {
    return this.biometry ? this.#attempt("") : Promise.resolve();
  }

  async #attempt(pin: string): Promise<void> {
    const view = this.state.get();
    if (view.trying) return;
    this.state.set({ ...view, trying: pin ? "pin" : "device", note: null });
    const outcome = await this.#unlock(pin).catch(
      () => ({ kind: "failed" }) as const,
    );
    const now = this.state.get();
    // Whatever leaves the vault locked clears the field, and a PIN tried counts as turned down.
    const locked = {
      ...now,
      trying: null,
      pin: "",
      rejections: now.rejections + (pin ? 1 : 0),
    };
    switch (outcome.kind) {
      case "opened":
        return;
      case "wrong-pin":
        this.state.set({
          ...locked,
          methods: { ...now.methods, pinAttemptsLeft: outcome.attemptsLeft },
          note: "wrong-pin",
        });
        return;
      case "pin-removed":
        this.state.set({
          ...locked,
          methods: { ...now.methods, pinSet: false },
          note: "pin-removed",
        });
        return;
      case "canceled":
        this.state.set({ ...now, trying: null });
        return;
      case "too-soon":
      case "failed":
        this.state.set({ ...locked, note: outcome.kind });
        return;
    }
  }
}
