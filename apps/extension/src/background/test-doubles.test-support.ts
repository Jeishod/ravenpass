import type {
  PasskeyChoice,
  PasskeyOption,
  PasskeyTargets,
  SessionError,
  ShareProgress,
} from "../link/client.ts";
import type { CreateOptions, GetOptions } from "../passkeys/requests.ts";
import type { CreatedPasskey, SignedPasskey } from "../passkeys/responses.ts";
import type { PasskeyClient } from "./page-passkeys.ts";
import type { Sender, SessionArea } from "./sessions.ts";

export const signedPasskey: SignedPasskey = {
  credentialId: "AQID",
  clientDataJSON: "eyJ0eXBlIjoid2ViYXV0aG4uZ2V0In0",
  authenticatorData: "BAUG",
  signature: "MEUCIQ",
  userHandle: "dXNlcg",
};

export const createdPasskey: CreatedPasskey = {
  credentialId: "AQID",
  clientDataJSON: "eyJ0eXBlIjoid2ViYXV0aG4uY3JlYXRlIn0",
  attestationObject: "o2NmbXRkbm9uZQ",
  authenticatorData: "BAUG",
  publicKey: "MFkwEwYHKoZIzj0CAQ",
  publicKeyAlgorithm: -7,
};

/** Signing and creating report `progress` before `verification` may refuse. */
export class ScriptedPasskeys implements PasskeyClient {
  linkedNow = true;
  listed: PasskeyOption[] = [];
  targets: PasskeyTargets = { targets: [], excluded: false };
  refusal: SessionError | null = null;
  verification: SessionError | null = null;
  progress: ShareProgress[] = [];
  readonly asked: [string, string, GetOptions | CreateOptions][] = [];
  readonly signed: [string, GetOptions, PasskeyChoice][] = [];
  readonly created: [string, CreateOptions, string][] = [];

  async linked(): Promise<boolean> {
    return this.linkedNow;
  }

  async passkeys(
    origin: string,
    options: GetOptions,
  ): Promise<PasskeyOption[]> {
    this.asked.push(["passkeys", origin, options]);
    if (this.refusal) throw this.refusal;
    return [...this.listed];
  }

  async passkeyTargets(
    origin: string,
    options: CreateOptions,
  ): Promise<PasskeyTargets> {
    this.asked.push(["targets", origin, options]);
    if (this.refusal) throw this.refusal;
    return this.targets;
  }

  async createPasskey(
    origin: string,
    options: CreateOptions,
    target: string,
    onProgress: (progress: ShareProgress) => void,
  ): Promise<CreatedPasskey> {
    this.created.push([origin, options, target]);
    for (const step of this.progress) onProgress(step);
    if (this.verification) throw this.verification;
    return createdPasskey;
  }

  async signPasskey(
    origin: string,
    options: GetOptions,
    choice: PasskeyChoice,
    onProgress: (progress: ShareProgress) => void,
  ): Promise<SignedPasskey> {
    this.signed.push([origin, options, choice]);
    for (const step of this.progress) onProgress(step);
    if (this.verification) throw this.verification;
    return signedPasskey;
  }
}

/** `chrome.storage.session` as far as the service worker uses it. */
export class MemoryArea implements SessionArea {
  readonly items = new Map<string, unknown>();

  async get(keys: string | null): Promise<Record<string, unknown>> {
    if (keys === null) return Object.fromEntries(this.items);
    return this.items.has(keys) ? { [keys]: this.items.get(keys) } : {};
  }

  async set(items: Record<string, unknown>): Promise<void> {
    for (const [key, value] of Object.entries(items)) {
      this.items.set(key, structuredClone(value));
    }
  }

  async remove(keys: string | string[]): Promise<void> {
    for (const key of typeof keys === "string" ? [keys] : keys) {
      this.items.delete(key);
    }
  }
}

export class Clock {
  time = 1_750_000_000_000;
  readonly now = () => this.time;
}

export const extensionId = "cceiadaelnccfbakmhcleifjfilkakag";

/** The content script of a GitHub page's top frame in tab 7, as Chrome reports it. */
export function pageSender(overrides: Partial<Sender> = {}): Sender {
  return {
    id: extensionId,
    url: "https://github.com/login",
    origin: "https://github.com",
    tab: { id: 7, url: "https://github.com/login" },
    frameId: 0,
    documentId: "page-document",
    ...overrides,
  };
}

/** The menu page in a frame of tab 7, as Chrome reports it. */
export function menuSender(overrides: Partial<Sender> = {}): Sender {
  return {
    id: extensionId,
    url: "chrome-extension://4f0e7c1a-dynamic/pages/menu.html",
    origin: "chrome-extension://4f0e7c1a-dynamic",
    tab: { id: 7, url: "https://github.com/login" },
    frameId: 3,
    documentId: "menu-document",
    ...overrides,
  };
}
