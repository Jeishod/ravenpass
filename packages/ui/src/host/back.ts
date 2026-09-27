import { useEffect, useEffectEvent } from "react";

export interface BackHistory {
  push(): void;
  back(): void;
  onPop(listener: () => void): void;
}

interface BackScreen {
  close: () => void;
  held: () => boolean;
}

/** Android goes back in page history before leaving the app, so one pushed entry turns back into a close. */
export class BackStack {
  readonly #history: BackHistory;
  readonly #screens: BackScreen[] = [];
  #armed = false;
  #ignoring = 0;

  constructor(history: BackHistory) {
    this.#history = history;
    history.onPop(() => this.#popped());
  }

  /** Call the returned function once the screen is gone. While `held` is true the gesture leaves the screen open. */
  open(close: () => void, held: () => boolean = () => false): () => void {
    const screen = { close, held };
    this.#screens.push(screen);
    this.#arm();
    return () => {
      const index = this.#screens.indexOf(screen);
      if (index >= 0) this.#screens.splice(index, 1);
      // A screen that replaces another in the same render registers before this runs.
      queueMicrotask(() => this.#settle());
    };
  }

  #popped() {
    if (this.#ignoring > 0) {
      this.#ignoring--;
      return;
    }
    this.#armed = false;
    if (this.#screens.at(-1)?.held() !== true) this.#screens.pop()?.close();
    if (this.#screens.length > 0) this.#arm();
  }

  #arm() {
    if (this.#armed) return;
    this.#armed = true;
    this.#history.push();
  }

  /** Drops the entry once no screen needs it, so the next back leaves the app. */
  #settle() {
    if (this.#screens.length > 0 || !this.#armed) return;
    this.#armed = false;
    this.#ignoring++;
    this.#history.back();
  }
}

let pageStack: BackStack | undefined;

function stackOfPage(): BackStack {
  pageStack ??= new BackStack({
    push: () => window.history.pushState(null, ""),
    back: () => window.history.back(),
    onPop: (listener) => window.addEventListener("popstate", listener),
  });
  return pageStack;
}

/** Calls `close` on the system back gesture while the screen is shown and innermost; a held screen stays open. */
export function useSystemBack(close: () => void, shown = true, held = false) {
  const onClose = useEffectEvent(close);
  const isHeld = useEffectEvent(() => held);
  useEffect(
    () =>
      shown
        ? stackOfPage().open(
            () => onClose(),
            () => isHeld(),
          )
        : undefined,
    [shown],
  );
}
