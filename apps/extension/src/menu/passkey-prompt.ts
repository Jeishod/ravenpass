import type { MessageKey } from "@ravenpass/ui/i18n/messages.ts";
import type { PasskeyChoice, ShareProgress } from "../link/client.ts";
import type {
  Answers,
  PasskeyCardContent,
  PasskeyFailure,
  PasskeyListing,
} from "../messages.ts";
import { LockedPrompt } from "./locked-prompt.ts";
import type { VaultWatcher } from "./vault-state.ts";

export const newItem = "new";

export type PasskeyNote = Exclude<
  PasskeyFailure,
  "locked" | "not-open" | "excluded"
>;

/** `target` is where a save goes: a credential the card offers, or `newItem`. */
export interface PasskeyView {
  readonly listing: PasskeyListing;
  readonly target: string;
  readonly waiting: { readonly progress: ShareProgress | null } | null;
  readonly note: PasskeyNote | null;
  readonly unlockFailed: boolean;
}

export interface PasskeyPromptRequests {
  readonly sign: (choice: PasskeyChoice) => Promise<Answers["passkey-sign"]>;
  readonly save: (target: string) => Promise<Answers["passkey-save"]>;
  readonly elsewhere: () => Promise<Answers["passkey-elsewhere"]>;
  readonly review: () => Promise<Answers["passkey-review"]>;
  readonly unlock: () => Promise<Answers["menu-unlock"]>;
  readonly watchVault: VaultWatcher;
  /** Refuses the page's request. */
  readonly close: () => void;
}

/** The service worker closes the card once the page has its answer. */
export class PasskeyPrompt extends LockedPrompt<PasskeyView> {
  private readonly requests: PasskeyPromptRequests;

  constructor(content: PasskeyCardContent, requests: PasskeyPromptRequests) {
    super(requests.watchVault, listed(content.listing));
    this.requests = requests;
  }

  choose(target: string): void {
    if (this.current.waiting) return;
    this.update({ ...this.current, target, note: null });
  }

  progress(progress: ShareProgress): void {
    if (this.current.waiting) {
      this.update({ ...this.current, waiting: { progress } });
    }
  }

  sign(choice: PasskeyChoice): Promise<void> {
    if (this.current.listing.state !== "sign-in") return Promise.resolve();
    return this.act(() => this.requests.sign(choice));
  }

  save(): Promise<void> {
    if (this.current.listing.state !== "save") return Promise.resolve();
    const { target } = this.current;
    return this.act(() => this.requests.save(target));
  }

  /** Hands the page's request to Chrome. */
  async elsewhere(): Promise<void> {
    if (this.current.waiting) return;
    const handed = await this.requests.elsewhere().then(
      () => true,
      () => false,
    );
    if (!handed) this.requests.close();
  }

  /** Brings Ravenpass forward; the vault state change that follows lists the card again. */
  async unlock(): Promise<void> {
    if (this.current.listing.state !== "locked") return;
    const run = this.run;
    this.update({ ...this.current, unlockFailed: false });
    const answer = await this.requests
      .unlock()
      .catch(() => ({ ok: false, reason: "failed" }) as const);
    if (run !== this.run || answer.ok) return;
    if (answer.reason === "not-open") {
      this.update(listed({ state: "not-open" }));
    } else if (
      answer.reason === "failed" &&
      this.current.listing.state === "locked"
    ) {
      this.update({ ...this.current, unlockFailed: true });
    }
  }

  close(): void {
    if (!this.current.waiting) this.requests.close();
  }

  private async act(
    request: () => Promise<Answers["passkey-sign" | "passkey-save"]>,
  ): Promise<void> {
    if (this.current.waiting) return;
    const run = this.run;
    this.update({ ...this.current, waiting: { progress: null }, note: null });
    const answer = await request().catch(
      () => ({ ok: false, reason: "failed" }) as const,
    );
    if (run !== this.run || answer.ok) return;
    switch (answer.reason) {
      case "locked":
      case "not-open":
      case "excluded":
        this.update(listed({ state: answer.reason }));
        break;
      default:
        this.update({ ...this.current, waiting: null, note: answer.reason });
    }
  }

  protected showsLocked(view: PasskeyView): boolean {
    return view.listing.state === "locked";
  }

  protected async review(): Promise<void> {
    const run = this.run;
    const answer = await this.requests.review().catch(() => null);
    if (run !== this.run || this.current.listing.state !== "locked") return;
    if (
      answer &&
      answer.listing !== null &&
      answer.listing.state !== "locked"
    ) {
      this.update(listed(answer.listing));
    }
  }
}

export function passkeyNote(
  note: PasskeyNote,
  mode: PasskeyCardContent["mode"],
): MessageKey {
  switch (note) {
    case "declined":
    case "unverifiable":
    case "full":
      return `extension.passkey.${note}`;
    default:
      return mode === "get"
        ? "extension.passkey.sign-in.error"
        : "extension.passkey.save.error";
  }
}

/** A save starts on the first offered credential with the page's account, else a new item. */
function listed(listing: PasskeyListing): PasskeyView {
  const target =
    listing.state === "save"
      ? (listing.targets.find(({ account }) => account === listing.account)
          ?.credential ?? newItem)
      : newItem;
  return { listing, target, waiting: null, note: null, unlockFailed: false };
}
