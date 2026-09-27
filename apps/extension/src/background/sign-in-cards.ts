import type { SuggestPurpose } from "../link/client.ts";
import {
  type CredentialMenuContent,
  isPresent,
  isShown,
  type PasskeyCardContent,
} from "../messages.ts";
import type { TabMessenger } from "./menus.ts";
import {
  hostOf,
  type MenuSession,
  type MenuSessions,
  type PageFrame,
} from "./sessions.ts";

export interface SignInCardsDependencies {
  readonly sessions: MenuSessions;
  readonly relay: TabMessenger;
}

/** A reported form keeps the card until the form leaves; a focused field takes it at once; a passkey card blocks both, an open field menu the first. */
export class SignInCards {
  private readonly sessions: MenuSessions;
  private readonly relay: TabMessenger;
  /** Each decision about a card waits for the one before, so it sees the card that one left. */
  private decided: Promise<void> = Promise.resolve();

  constructor({ sessions, relay }: SignInCardsDependencies) {
    this.sessions = sessions;
    this.relay = relay;
  }

  /** `focused` means the person's input on a field asked for the card, which marks it requested. */
  show(
    page: PageFrame,
    purpose: SuggestPurpose,
    listing: CredentialMenuContent,
    focused: boolean,
  ): Promise<void> {
    const shown = this.decided.then(() =>
      this.place(page, purpose, listing, focused),
    );
    this.decided = shown.catch(() => {});
    return shown;
  }

  /** Null when the top frame does not show the card, as while a save offer shows. */
  showPasskey(
    page: PageFrame,
    content: PasskeyCardContent,
  ): Promise<string | null> {
    const shown = this.decided.then(() => this.placePasskey(page, content));
    this.decided = shown.then(
      () => {},
      () => {},
    );
    return shown;
  }

  /** Closes the tab's sign-in card before a field menu opens in the tab; a passkey card stays. */
  withdraw(tabId: number): Promise<void> {
    const withdrawn = this.decided.then(async () => {
      const current = await this.sessions.cardOf(tabId);
      if (current?.content.state === "sign-in-card") {
        await this.hide(current, false);
      }
    });
    this.decided = withdrawn.catch(() => {});
    return withdrawn;
  }

  /** `dismissed` means the person closed the card. */
  async hide(card: MenuSession, dismissed: boolean): Promise<void> {
    await this.relay(hostOf(card), {
      kind: "card-hide",
      token: card.token,
      dismissed,
    });
    await this.sessions.end(card.token);
  }

  private async place(
    page: PageFrame,
    purpose: SuggestPurpose,
    listing: CredentialMenuContent,
    focused: boolean,
  ): Promise<void> {
    if (!focused && (await this.sessions.fieldMenuIn(page.tabId))) return;
    const current = await this.sessions.cardOf(page.tabId);
    if (current?.content.state === "passkey") return;
    if (current) {
      const sameFrame = current.page.documentId === page.documentId;
      if (
        sameFrame &&
        current.content.state === "sign-in-card" &&
        current.content.purpose === purpose
      ) {
        if (focused && !current.content.requested) {
          await this.sessions.revise(current.token, {
            ...current.content,
            requested: true,
          });
        }
        return;
      }
      if (!focused && !sameFrame && (await this.formPresent(current))) return;
      await this.hide(current, false);
    }
    const token = await this.sessions.open(page, {
      state: "sign-in-card",
      purpose,
      listing,
      requested: focused,
    });
    const answer = await this.relay(
      { tabId: page.tabId, frameId: 0 },
      { kind: "card-show", token, reopen: focused },
    );
    if (!isShown(answer)) await this.sessions.end(token);
  }

  private async placePasskey(
    page: PageFrame,
    content: PasskeyCardContent,
  ): Promise<string | null> {
    const current = await this.sessions.cardOf(page.tabId);
    if (current) await this.hide(current, false);
    const token = await this.sessions.open(page, content);
    const answer = await this.relay(
      { tabId: page.tabId, frameId: 0 },
      { kind: "card-show", token, reopen: true },
    );
    if (isShown(answer)) return token;
    await this.sessions.end(token);
    return null;
  }

  /** A frame that navigated away answers nothing, which reads as the form gone. */
  private async formPresent(card: MenuSession): Promise<boolean> {
    return isPresent(await this.relay(card.page, { kind: "form-present" }));
  }
}
