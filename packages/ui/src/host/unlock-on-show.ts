export interface LockedScreen {
  visible: boolean;
  /** False while still unknown. */
  deviceUnlock: boolean;
  /** False right after the owner locked the vault, and while still unknown. */
  allowed: boolean;
  /** An unlock runs, the recovery key is being entered, or another vault is opening. */
  occupied: boolean;
}

/** Asks for the device's unlock at most once per showing, and not in a showing spent on something else. */
export class UnlockOnShow {
  #visible = false;
  #due = false;
  #hiddenDuringUnlock = false;

  next(screen: LockedScreen): boolean {
    if (!screen.visible) {
      if (this.#visible) this.#hiddenDuringUnlock = screen.occupied;
      this.#visible = false;
      this.#due = false;
      return false;
    }
    if (!this.#visible) {
      this.#visible = true;
      // A system prompt hides the page; its closing must not count as a new showing.
      this.#due = !this.#hiddenDuringUnlock;
      this.#hiddenDuringUnlock = false;
    }
    if (!this.#due) return false;
    if (screen.occupied) {
      this.#due = false;
      return false;
    }
    if (!screen.deviceUnlock || !screen.allowed) return false;
    this.#due = false;
    return true;
  }
}
