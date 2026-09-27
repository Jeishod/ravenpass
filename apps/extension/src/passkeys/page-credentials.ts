import {
  type BridgeMessage,
  type ChannelPort,
  notAllowed,
  type PageAnswer,
  type PageMessage,
  readBridgeMessage,
} from "./page-channel.ts";
import {
  createOptionsOf,
  type GetOptions,
  getOptionsOf,
  type PageCreationOptions,
  type PageRequest,
  type PageRequestOptions,
} from "./requests.ts";
import {
  createdCredential,
  signedCredential,
  type WebAuthnPrototypes,
} from "./responses.ts";

/** Adds the `mediation` Chrome reads and TypeScript's DOM library omits. */
export type PageCreateCall = Omit<CredentialCreationOptions, "publicKey"> & {
  readonly publicKey?: PageCreationOptions;
  readonly mediation?: CredentialMediationRequirement;
};

export type PageGetCall = Omit<CredentialRequestOptions, "publicKey"> & {
  readonly publicKey?: PageRequestOptions;
};

/** The page's own WebAuthn functions, kept before they were replaced. */
export interface NativeCredentials {
  create(options?: PageCreateCall): Promise<Credential | null>;
  get(options?: PageGetCall): Promise<Credential | null>;
  /** Chrome's `isConditionalMediationAvailable`. */
  conditionalMediation(): Promise<boolean>;
}

export interface PageCredentialsDependencies {
  readonly native: NativeCredentials;
  readonly prototypes: WebAuthnPrototypes;
  /** Builds the DOMException a request rejects with. */
  readonly exception: (message: string, name: string) => Error;
}

/** The main world's replacement for the page's `create`, `get` and `isConditionalMediationAvailable`. */
export class PageCredentials {
  private readonly native: NativeCredentials;
  private readonly prototypes: WebAuthnPrototypes;
  private readonly exception: (message: string, name: string) => Error;
  private port: ChannelPort<PageMessage> | null = null;
  private readonly queued: PageMessage[] = [];
  private readonly waiting = new Map<
    number,
    (message: BridgeMessage) => void
  >();
  private lastId = 0;

  constructor({ native, prototypes, exception }: PageCredentialsDependencies) {
    this.native = native;
    this.prototypes = prototypes;
    this.exception = exception;
  }

  /** Takes the port to the isolated script. Only the first port is taken. */
  connect(port: ChannelPort<PageMessage>): boolean {
    if (this.port) return false;
    this.port = port;
    port.addEventListener("message", (event) => this.receive(event.data));
    port.start();
    for (const message of this.queued.splice(0)) port.postMessage(message);
    return true;
  }

  create(options?: PageCreateCall): Promise<Credential | null> {
    const publicKey = options?.publicKey;
    if (!publicKey || options.mediation === "conditional") {
      return this.native.create(options);
    }
    let request: PageRequest;
    try {
      request = { mode: "create", options: createOptionsOf(publicKey) };
    } catch {
      return this.native.create(options);
    }
    return this.modal(request, options.signal, () =>
      this.native.create(options),
    );
  }

  get(options?: PageGetCall): Promise<Credential | null> {
    const publicKey = options?.publicKey;
    if (!publicKey || options.mediation === "silent") {
      return this.native.get(options);
    }
    let getOptions: GetOptions;
    try {
      getOptions = getOptionsOf(publicKey);
    } catch {
      return this.native.get(options);
    }
    if (options.mediation === "conditional") {
      return this.conditional(options, getOptions);
    }
    return this.modal(
      { mode: "get", options: getOptions, conditional: false },
      options.signal,
      () => this.native.get(options),
    );
  }

  /** True when Chrome's own conditional mediation is available, or when Ravenpass is linked. */
  async isConditionalMediationAvailable(): Promise<boolean> {
    if (await this.native.conditionalMediation().catch(() => false)) {
      return true;
    }
    return new Promise((resolve) => {
      const id = this.nextId();
      this.waiting.set(id, (message) => {
        this.waiting.delete(id);
        resolve(message.kind === "linked" && message.linked);
      });
      this.post({ kind: "linked", id });
    });
  }

  private modal(
    request: PageRequest,
    signal: AbortSignal | undefined,
    toChrome: () => Promise<Credential | null>,
  ): Promise<Credential | null> {
    if (signal?.aborted) return Promise.reject(signal.reason);
    return new Promise((resolve, reject) => {
      const id = this.nextId();
      const onAbort = () => {
        this.waiting.delete(id);
        this.post({ kind: "withdraw", id });
        reject(signal?.reason);
      };
      signal?.addEventListener("abort", onAbort, { once: true });
      this.waiting.set(id, (message) => {
        if (message.kind !== "answer") return;
        this.waiting.delete(id);
        signal?.removeEventListener("abort", onAbort);
        this.settle(request, message.answer, toChrome).then(resolve, reject);
      });
      this.post({ kind: "request", id, request });
    });
  }

  /** Races Ravenpass and Chrome: Ravenpass wins only with a signature, Chrome with any settlement. */
  private conditional(
    options: PageGetCall,
    getOptions: GetOptions,
  ): Promise<Credential | null> {
    const { signal } = options;
    if (signal?.aborted) return Promise.reject(signal.reason);
    return new Promise((resolve, reject) => {
      const id = this.nextId();
      const chromeRequest = new AbortController();
      let settled = false;
      const finish = () => {
        settled = true;
        this.waiting.delete(id);
        signal?.removeEventListener("abort", onAbort);
      };
      const onAbort = () => {
        finish();
        chromeRequest.abort(signal?.reason);
        this.post({ kind: "withdraw", id });
        reject(signal?.reason);
      };
      signal?.addEventListener("abort", onAbort, { once: true });
      this.waiting.set(id, (message) => {
        if (message.kind !== "answer" || message.answer.kind !== "signed") {
          return;
        }
        let credential: Credential;
        try {
          credential = this.signed(message.answer.passkey);
        } catch {
          return;
        }
        finish();
        chromeRequest.abort();
        resolve(credential);
      });
      this.post({
        kind: "request",
        id,
        request: { mode: "get", options: getOptions, conditional: true },
      });
      this.native.get({ ...options, signal: chromeRequest.signal }).then(
        (credential) => {
          if (settled) return;
          finish();
          this.post({ kind: "withdraw", id });
          resolve(credential);
        },
        (error: unknown) => {
          if (settled) return;
          finish();
          this.post({ kind: "withdraw", id });
          reject(error);
        },
      );
    });
  }

  private async settle(
    request: PageRequest,
    answer: PageAnswer,
    toChrome: () => Promise<Credential | null>,
  ): Promise<Credential | null> {
    if (answer.kind === "refused") {
      throw this.exception(answer.message, answer.name);
    }
    try {
      if (answer.kind === "created" && request.mode === "create") {
        return createdCredential(
          answer.passkey,
          request.options.credProps,
          this.prototypes,
        ) as Credential;
      }
      if (answer.kind === "signed" && request.mode === "get") {
        return this.signed(answer.passkey);
      }
    } catch {
      throw this.exception(notAllowed.message, notAllowed.name);
    }
    return toChrome();
  }

  private signed(
    passkey: Extract<PageAnswer, { kind: "signed" }>["passkey"],
  ): Credential {
    return signedCredential(passkey, this.prototypes) as Credential;
  }

  private receive(data: unknown): void {
    const message = readBridgeMessage(data);
    if (message) this.waiting.get(message.id)?.(message);
  }

  private post(message: PageMessage): void {
    if (this.port) this.port.postMessage(message);
    else this.queued.push(message);
  }

  private nextId(): number {
    this.lastId += 1;
    return this.lastId;
  }
}
