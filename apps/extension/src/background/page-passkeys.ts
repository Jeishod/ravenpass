import {
  type LinkClient,
  type PasskeyChoice,
  type PasskeyOption,
  SessionError,
  type ShareProgress,
} from "../link/client.ts";
import type {
  Answers,
  PasskeyFailure,
  PasskeyListing,
  PasskeyPageRequest,
} from "../messages.ts";
import {
  alreadyPending,
  alreadyRegistered,
  native,
  notAllowed,
  type PageAnswer,
} from "../passkeys/page-channel.ts";
import {
  type PageRequest,
  readPageRequest,
  servable,
} from "../passkeys/requests.ts";
import type { CreatedPasskey, SignedPasskey } from "../passkeys/responses.ts";
import { RefusedRequest, type TabMessenger } from "./menus.ts";
import type { PendingPasskey, PendingPasskeys } from "./pending-passkeys.ts";
import {
  type MenuSession,
  type MenuSessions,
  menuDocumentOf,
  type PageFrame,
  type Sender,
  type TabDocument,
} from "./sessions.ts";
import type { SignInCards } from "./sign-in-cards.ts";

export type PasskeyClient = Pick<
  LinkClient,
  "linked" | "passkeys" | "passkeyTargets" | "createPasskey" | "signPasskey"
>;

export interface PagePasskeysDependencies {
  readonly client: PasskeyClient;
  readonly sessions: MenuSessions;
  readonly cards: SignInCards;
  readonly pending: PendingPasskeys;
  readonly relay: TabMessenger;
}

type Outcome = { ok: true } | { ok: false; reason: PasskeyFailure };

/** Serves only a tab's top frame with the origin Chrome reported, and answers the request's document alone. */
export class PagePasskeys {
  private readonly client: PasskeyClient;
  private readonly sessions: MenuSessions;
  private readonly cards: SignInCards;
  private readonly pending: PendingPasskeys;
  private readonly relay: TabMessenger;

  constructor({
    client,
    sessions,
    cards,
    pending,
    relay,
  }: PagePasskeysDependencies) {
    this.client = client;
    this.sessions = sessions;
    this.cards = cards;
    this.pending = pending;
    this.relay = relay;
  }

  async serve(
    request: PasskeyPageRequest,
    sender: Sender,
  ): Promise<Answers[PasskeyPageRequest["kind"]]> {
    switch (request.kind) {
      case "passkey-request":
        return this.request(request.id, request.request, sender);
      case "passkey-withdraw":
        return this.withdraw(request.id, sender);
      case "passkey-linked":
        return {
          linked:
            this.topFrameOf(sender) !== null && (await this.client.linked()),
        };
    }
  }

  async tabClosed(tabId: number): Promise<void> {
    await this.pending.forget(tabId);
  }

  async optionsFor(page: TabDocument): Promise<PasskeyOption[]> {
    const pending = await this.pending.conditionalIn(page);
    if (pending?.request.mode !== "get") return [];
    // Passkeys Ravenpass cannot list leave the menu to its passwords.
    return this.client
      .passkeys(pending.page.origin, pending.request.options)
      .catch(() => []);
  }

  async sign(session: MenuSession, choice: PasskeyChoice): Promise<Outcome> {
    const pending = await this.pendingOf(session);
    if (pending?.request.mode !== "get") return { ok: false, reason: "failed" };
    let signed: SignedPasskey;
    try {
      signed = await this.client.signPasskey(
        pending.page.origin,
        pending.request.options,
        choice,
        this.progressTo(session),
      );
    } catch (error) {
      return this.refused(session, pending, error);
    }
    return this.answer(session, pending, { kind: "signed", passkey: signed });
  }

  /** `target` is a new item or a credential the card offers. */
  async save(session: MenuSession, target: string): Promise<Outcome> {
    const pending = await this.pendingOf(session);
    if (pending?.request.mode !== "create") {
      return { ok: false, reason: "failed" };
    }
    let created: CreatedPasskey;
    try {
      created = await this.client.createPasskey(
        pending.page.origin,
        pending.request.options,
        target,
        this.progressTo(session),
      );
    } catch (error) {
      return this.refused(session, pending, error);
    }
    return this.answer(session, pending, { kind: "created", passkey: created });
  }

  /** Hands a card's request to Chrome, as Other options does. */
  async elsewhere(session: MenuSession): Promise<Answers["passkey-elsewhere"]> {
    const pending = await this.pendingOf(session);
    if (pending) await this.answer(session, pending, native);
    else await this.cards.hide(session, false);
    return { ok: true };
  }

  /** A request with nothing left to list goes to Chrome. */
  async review(session: MenuSession): Promise<Answers["passkey-review"]> {
    const pending = await this.pendingOf(session);
    if (!pending || session.content.state !== "passkey") {
      await this.cards.hide(session, false);
      return { listing: null };
    }
    const listing = await this.listingFor(pending.page, pending.request);
    if (!listing) {
      await this.answer(session, pending, native);
      return { listing: null };
    }
    await this.sessions.revise(session.token, { ...session.content, listing });
    return { listing };
  }

  /** Refuses as Chrome refuses a dismissed dialog, or as already registered once the card showed an excluded passkey. */
  async dismiss(session: MenuSession): Promise<void> {
    if (session.content.state !== "passkey") return;
    const pending = await this.pendingOf(session);
    if (!pending) {
      await this.cards.hide(session, false);
      return;
    }
    await this.answer(
      session,
      pending,
      session.content.listing.state === "excluded"
        ? alreadyRegistered
        : notAllowed,
    );
  }

  private async request(
    id: number,
    value: unknown,
    sender: Sender,
  ): Promise<Answers["passkey-request"]> {
    const page = this.topFrameOf(sender);
    const request = readPageRequest(value);
    if (!page || !request || !Number.isSafeInteger(id) || !servable(request)) {
      return { answer: native };
    }
    if (request.mode === "get" && request.conditional) {
      if (!(await this.client.linked())) return { answer: native };
      await this.pending.add({ page, id, request });
      return { answer: null };
    }
    if (await this.pending.modalIn(page)) return { answer: alreadyPending };
    const listing = await this.listingFor(page, request);
    if (!listing) return { answer: native };
    await this.pending.add({ page, id, request });
    const token = await this.cards.showPasskey(page, {
      state: "passkey",
      request: id,
      mode: request.mode,
      listing,
    });
    if (token) return { answer: null };
    await this.pending.take(page, id);
    return { answer: native };
  }

  /** Ends a request the page aborted, whose timeout ran out, or that Chrome answered first. */
  private async withdraw(
    id: number,
    sender: Sender,
  ): Promise<Answers["passkey-withdraw"]> {
    const page = this.topFrameOf(sender);
    if (!page) throw new RefusedRequest("passkey-withdraw");
    if (await this.pending.take(page, id)) {
      const card = await this.sessions.cardOf(page.tabId);
      if (
        card?.content.state === "passkey" &&
        card.content.request === id &&
        card.page.documentId === page.documentId
      ) {
        await this.cards.hide(card, false);
      }
    }
    return { ok: true };
  }

  /** Null sends the request to Chrome. */
  private async listingFor(
    page: PageFrame,
    request: PageRequest,
  ): Promise<PasskeyListing | null> {
    try {
      if (request.mode === "get") {
        const passkeys = await this.client.passkeys(
          page.origin,
          request.options,
        );
        return passkeys.length ? { state: "sign-in", passkeys } : null;
      }
      const { targets, excluded } = await this.client.passkeyTargets(
        page.origin,
        request.options,
      );
      return excluded
        ? { state: "excluded" }
        : { state: "save", account: request.options.user.name, targets };
    } catch (error) {
      if (
        error instanceof SessionError &&
        (error.reason === "locked" || error.reason === "not-open")
      ) {
        return { state: error.reason };
      }
      return null;
    }
  }

  /** A request the page withdrew meanwhile takes nothing. */
  private async answer(
    session: MenuSession,
    pending: PendingPasskey,
    answer: PageAnswer,
  ): Promise<Outcome> {
    const taken = await this.pending.take(pending.page, pending.id);
    if (taken) {
      await this.relay(pending.page, {
        kind: "passkey-answer",
        id: pending.id,
        answer,
      });
    }
    if (session.content.state === "passkey") {
      await this.cards.hide(session, false);
    }
    return taken ? { ok: true } : { ok: false, reason: "failed" };
  }

  private async refused(
    session: MenuSession,
    pending: PendingPasskey,
    error: unknown,
  ): Promise<Outcome> {
    const reason = error instanceof SessionError ? error.reason : "failed";
    const card = session.content.state === "passkey" ? session.content : null;
    switch (reason) {
      case "locked":
      case "not-open":
      case "excluded":
        if (card) {
          await this.sessions.revise(session.token, {
            ...card,
            listing: { state: reason },
          });
        }
        return { ok: false, reason };
      case "declined":
      case "unverifiable":
      case "full":
        return { ok: false, reason };
      case "invalid-rp":
      case "invalid-request":
      case "invalid-origin":
        if (card) return this.answer(session, pending, native);
        return { ok: false, reason: "failed" };
      default:
        return { ok: false, reason: "failed" };
    }
  }

  private pendingOf(session: MenuSession): Promise<PendingPasskey | null> {
    return session.content.state === "passkey"
      ? this.pending.find(session.page, session.content.request)
      : this.pending.conditionalIn(session.page);
  }

  private progressTo(session: MenuSession): (progress: ShareProgress) => void {
    const menu = menuDocumentOf(session);
    return (progress) => {
      if (menu) {
        void this.relay(menu, {
          kind: "passkey-progress",
          token: session.token,
          progress,
        });
      }
    };
  }

  /** `https` and `http://localhost` are the WebAuthn secure contexts served. */
  private topFrameOf(sender: Sender): PageFrame | null {
    const page = this.sessions.pageOf(sender);
    if (!page || sender.frameId !== 0) return null;
    const { protocol, hostname } = new URL(page.origin);
    return protocol === "https:" ||
      (protocol === "http:" && hostname === "localhost")
      ? page
      : null;
  }
}
