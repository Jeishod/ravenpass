import type { OneTimeCode } from "../vault-api.ts";

const minimumDelay = 1000;

/** CodeRenewal asks for a one-time code and again whenever the current one expires, until stopped; null reports a failed request, after which it stops asking. */
export class CodeRenewal {
  readonly #generate: () => Promise<OneTimeCode>;
  readonly #listener: (code: OneTimeCode | null) => void;
  #timer: ReturnType<typeof setTimeout> | undefined;
  #stopped = false;

  constructor(
    generate: () => Promise<OneTimeCode>,
    listener: (code: OneTimeCode | null) => void,
  ) {
    this.#generate = generate;
    this.#listener = listener;
  }

  start(): void {
    void this.#request();
  }

  stop(): void {
    this.#stopped = true;
    clearTimeout(this.#timer);
  }

  async #request(): Promise<void> {
    let code: OneTimeCode;
    try {
      code = await this.#generate();
    } catch {
      if (!this.#stopped) this.#listener(null);
      return;
    }
    if (this.#stopped) return;
    this.#listener(code);
    this.#timer = setTimeout(
      () => void this.#request(),
      Math.max(code.expiresAt - Date.now(), minimumDelay),
    );
  }
}
