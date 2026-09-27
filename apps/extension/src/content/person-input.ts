/** How soon after a Tab press the focus Chrome moves counts as the person's input. */
export const tabFocusMs = 100;

export interface PressEvent {
  readonly type: string;
  readonly isTrusted: boolean;
  readonly key?: string;
  readonly timeStamp: number;
}

/** A page's `focus()` call raises a trusted focus event: focus alone never opens a menu. */
export class PersonInput {
  /** When the person last pressed Tab, until a focus takes it. */
  private tabbedAt: number | null = null;

  pressed(event: PressEvent): void {
    if (event.isTrusted && event.type === "keydown" && event.key === "Tab") {
      this.tabbedAt = event.timeStamp;
    }
  }

  /** A `focusin` consumes the preceding Tab press whether or not it opens a menu. */
  opensMenu(event: PressEvent): boolean {
    switch (event.type) {
      case "pointerdown":
        return event.isTrusted;
      case "keydown":
        return event.isTrusted && event.key !== "Tab" && event.key !== "Escape";
      case "focusin": {
        const tabbedAt = this.tabbedAt;
        this.tabbedAt = null;
        return (
          tabbedAt !== null &&
          event.timeStamp >= tabbedAt &&
          event.timeStamp - tabbedAt <= tabFocusMs
        );
      }
      default:
        return false;
    }
  }
}
