/** How the page reaches its host; a request with id 0 expects no answer. */
export interface NativeAutofill {
  post(id: number, request: string): void;
}

/** A posted request: its id and the request itself as JSON text. */
interface Posted {
  id: number;
  request: string;
}

/** The Android host's `ravenpassAutofill` web message listener, which takes a `Posted` as JSON text. */
interface AndroidListener {
  postMessage(message: string): void;
}

/** The macOS AutoFill extension's `ravenpassAutofill` message handler. */
interface WebKitHandler {
  postMessage(message: Posted): void;
}

/** A host answer or notice: a JSON object whose `status` says how it ended or what it reports. */
export type Answer = Readonly<Record<string, unknown>> & {
  readonly status: string;
};

/** Sheets over the app being filled, or the whole window the system gives Ravenpass. */
export type AutofillSurface = "sheet" | "window";

declare global {
  interface Window {
    ravenpassAutofill?: AndroidListener;
    webkit?: {
      messageHandlers?: { ravenpassAutofill?: WebKitHandler };
    };
    /** Where the host delivers the answer to request `id`, as JSON text. */
    ravenpassAutofillAnswer?: (id: number, answer: string) => void;
    /** Where the host delivers a notice, as JSON text. */
    ravenpassAutofillNotice?: (notice: string) => void;
  }
}

const failed: Answer = { status: "failed" };

/** AutofillChannel matches the host's answers, which arrive in any order, to requests by id. */
export class AutofillChannel {
  readonly #native: NativeAutofill;
  readonly #waiting = new Map<number, (answer: Answer) => void>();
  readonly #listeners = new Set<(notice: Answer) => void>();
  #last = 0;

  constructor(native: NativeAutofill) {
    this.#native = native;
  }

  /** The channel of the page the host opened, and how the host shows it. */
  static ofPage(): { channel: AutofillChannel; surface: AutofillSurface } {
    const { native, surface } = hostOf(window);
    const channel = new AutofillChannel(native);
    window.ravenpassAutofillAnswer = (id, answer) => channel.answer(id, answer);
    window.ravenpassAutofillNotice = (notice) => channel.notice(notice);
    return { channel, surface };
  }

  /** Resolves with the answer; one that is not a JSON object with a status reads as failed. */
  request(op: string, fields: Record<string, unknown> = {}): Promise<Answer> {
    this.#last += 1;
    const id = this.#last;
    return new Promise((resolve) => {
      this.#waiting.set(id, resolve);
      this.#native.post(id, JSON.stringify({ ...fields, op }));
    });
  }

  /** Sends a request that takes no answer. */
  notify(op: string): void {
    this.#native.post(0, JSON.stringify({ op }));
  }

  /** Delivers the answer to request `id`; an answer to no waiting request is dropped. */
  answer(id: number, text: string): void {
    const resolve = this.#waiting.get(id);
    if (!resolve) return;
    this.#waiting.delete(id);
    resolve(parse(text) ?? failed);
  }

  /** Calls `listener` with each notice until the returned function is called. */
  watch(listener: (notice: Answer) => void): () => void {
    this.#listeners.add(listener);
    return () => this.#listeners.delete(listener);
  }

  /** Hands a notice to the listeners; one that is not a JSON object with a status is dropped. */
  notice(text: string): void {
    const notice = parse(text);
    if (!notice) return;
    for (const listener of this.#listeners) listener(notice);
  }
}

/** hostOf finds the Android object or the macOS message handler the page runs under. */
export function hostOf(page: Pick<Window, "ravenpassAutofill" | "webkit">): {
  native: NativeAutofill;
  surface: AutofillSurface;
} {
  const listener = page.ravenpassAutofill;
  if (listener) {
    return {
      native: {
        post: (id, request) =>
          listener.postMessage(
            JSON.stringify({ id, request } satisfies Posted),
          ),
      },
      surface: "sheet",
    };
  }
  const handler = page.webkit?.messageHandlers?.ravenpassAutofill;
  if (handler) {
    return {
      native: { post: (id, request) => handler.postMessage({ id, request }) },
      surface: "window",
    };
  }
  throw new Error("Ravenpass could not start this screen. Try again.");
}

/** The JSON object with a status `text` holds, or null for anything else. */
function parse(text: string): Answer | null {
  try {
    const value: unknown = JSON.parse(text);
    if (
      typeof value === "object" &&
      value !== null &&
      !Array.isArray(value) &&
      typeof (value as { status?: unknown }).status === "string"
    ) {
      return value as Answer;
    }
  } catch {
    return null;
  }
  return null;
}
