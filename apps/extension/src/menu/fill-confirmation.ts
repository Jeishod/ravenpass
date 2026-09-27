import type { Suggestion } from "../link/client.ts";
import type { Listed } from "../messages.ts";

interface Pending {
  readonly credential: Listed<Suggestion>;
  readonly proceed: () => void;
}

/** Remembers the credentials the person confirmed, and those whose page they asked to remember; their fills report both. */
export class FillConfirmation {
  private pending: Pending | null = null;
  private readonly confirmedIds = new Set<string>();
  private readonly rememberedIds = new Set<string>();
  private readonly listeners = new Set<() => void>();

  readonly subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  readonly view = (): Listed<Suggestion> | null =>
    this.pending?.credential ?? null;

  /** A choice that is not strong replaces any choice still awaiting confirmation. */
  choose(credential: Listed<Suggestion>, proceed: () => void): void {
    if (credential.strength === "strong") {
      proceed();
      return;
    }
    this.update({ credential, proceed });
  }

  /** `remember` counts only for an `other-site` suggestion, the one whose confirmation offers it. */
  confirm(remember: boolean): void {
    const pending = this.pending;
    if (!pending) return;
    const { id, strength } = pending.credential;
    this.confirmedIds.add(id);
    if (remember && strength === "other-site") {
      this.rememberedIds.add(id);
    } else {
      this.rememberedIds.delete(id);
    }
    this.update(null);
    pending.proceed();
  }

  cancel(): void {
    if (this.pending) this.update(null);
  }

  confirmed(id: string): boolean {
    return this.confirmedIds.has(id);
  }

  remembers(id: string): boolean {
    return this.rememberedIds.has(id);
  }

  private update(pending: Pending | null): void {
    this.pending = pending;
    for (const listener of this.listeners) listener();
  }
}
