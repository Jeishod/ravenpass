import {
  closing,
  periodEnd,
  periodLeft,
} from "@ravenpass/ui/credentials/one-time-code.ts";
import type { OneTimeCode } from "@ravenpass/ui/vault-api.ts";

// Floor on a reveal's lifetime when a code arrives already expired, as from a skewed desktop clock.
const minRevealMs = 1000;

export interface CodeRow {
  readonly id: string;
  /** Seconds. */
  readonly period: number;
}

export interface CodeMenuView {
  readonly revealed: { readonly id: string; readonly code: string } | null;
  /** The row whose fill waits for the next period. */
  readonly waiting: string | null;
}

export interface CodeMenuDependencies {
  readonly rows: readonly CodeRow[];
  /** A rejection leaves the row masked. */
  readonly reveal: (id: string) => Promise<OneTimeCode>;
  readonly fill: (id: string) => void;
}

/**
 * Asks for a code only while Option is held on the highlighted row; a shown code drops on release, move or period end.
 * A code still on its way survives release: the desktop may be waiting for the owner, who lets go of Option to type a PIN.
 */
export class CodeMenu {
  private readonly periods: ReadonlyMap<string, number>;
  private readonly reveal: (id: string) => Promise<OneTimeCode>;
  private readonly fill: (id: string) => void;
  private readonly listeners = new Set<() => void>();
  private current: CodeMenuView = { revealed: null, waiting: null };
  private held = false;
  private highlighted: string | null = null;
  /** An answer to any reveal but the latest is dropped. */
  private asked = 0;
  /** The row whose code was asked for and has not arrived. */
  private pending: string | null = null;
  private remask: ReturnType<typeof setTimeout> | undefined;
  private wait: ReturnType<typeof setTimeout> | undefined;

  constructor({ rows, reveal, fill }: CodeMenuDependencies) {
    this.periods = new Map(rows.map(({ id, period }) => [id, period]));
    this.reveal = reveal;
    this.fill = fill;
  }

  readonly subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  readonly view = (): CodeMenuView => this.current;

  /** Option pressed or released. */
  hold(held: boolean): void {
    if (held === this.held) return;
    this.held = held;
    this.retarget();
  }

  highlight(id: string | null): void {
    if (id === this.highlighted) return;
    this.highlighted = id;
    this.retarget();
  }

  /** Fills now, or once the next period begins when this one is closing. */
  choose(id: string): void {
    const period = this.periods.get(id);
    if (period === undefined) return;
    this.cancel();
    const now = Date.now();
    if (!closing(periodLeft(period, now))) {
      this.fill(id);
      return;
    }
    this.update({ waiting: id });
    this.fillAt(id, periodEnd(period, now));
  }

  /** Returns whether a fill was waiting. */
  cancel(): boolean {
    if (this.current.waiting === null) return false;
    clearTimeout(this.wait);
    this.update({ waiting: null });
    return true;
  }

  stop(): void {
    this.asked += 1;
    this.pending = null;
    clearTimeout(this.remask);
    clearTimeout(this.wait);
  }

  /** setTimeout can fire before `moment`. */
  private fillAt(id: string, moment: number): void {
    this.wait = setTimeout(() => {
      if (Date.now() < moment) {
        this.fillAt(id, moment);
        return;
      }
      this.update({ waiting: null });
      this.fill(id);
    }, moment - Date.now());
  }

  private retarget(): void {
    const id = this.held ? this.highlighted : null;
    if (this.pending !== null && (id === null || id === this.pending)) return;
    this.asked += 1;
    this.pending = null;
    clearTimeout(this.remask);
    if (this.current.revealed) this.update({ revealed: null });
    if (id !== null) this.ask(id);
  }

  private ask(id: string): void {
    const ticket = this.asked;
    this.pending = id;
    this.reveal(id).then(
      (code) => {
        if (ticket !== this.asked) return;
        this.pending = null;
        this.update({ revealed: { id, code: code.code } });
        this.remask = setTimeout(
          () => this.retarget(),
          Math.max(code.expiresAt - Date.now(), minRevealMs),
        );
      },
      () => {
        if (ticket === this.asked) this.pending = null;
      },
    );
  }

  private update(change: Partial<CodeMenuView>): void {
    this.current = { ...this.current, ...change };
    for (const listener of this.listeners) listener();
  }
}
