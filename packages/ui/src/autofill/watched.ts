/** Watched is a value a view reads through `useSyncExternalStore`. */
export class Watched<Value> {
  #value: Value;
  readonly #listeners = new Set<() => void>();

  constructor(value: Value) {
    this.#value = value;
  }

  readonly subscribe = (listener: () => void): (() => void) => {
    this.#listeners.add(listener);
    return () => this.#listeners.delete(listener);
  };

  readonly get = (): Value => this.#value;

  set(value: Value): void {
    this.#value = value;
    for (const listener of this.#listeners) listener();
  }
}
