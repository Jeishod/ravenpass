import {
  type RefusedField,
  reviseChoice,
  type SaveChoice,
  type SaveResult,
  saveResultOf,
  suggestedChoice,
} from "@ravenpass/ui/saving/offer.ts";
import type { Answers, SaveOffer } from "../messages.ts";
import { sendIgnoringClosedPort } from "../messaging/send.ts";
import { LockedPrompt } from "./locked-prompt.ts";
import type { VaultWatcher } from "./vault-state.ts";

const savedForMs = 1600;

/** `refused` is the new credential's field the vault did not take, until it is changed. */
export interface AskingView {
  readonly stage: "asking";
  readonly offer: SaveOffer;
  readonly choice: SaveChoice;
  readonly saving: boolean;
  readonly failed: boolean;
  readonly refused: RefusedField | null;
  readonly unlockFailed: boolean;
}

export type ShownView =
  | AskingView
  | { readonly stage: "not-open"; readonly offer: SaveOffer }
  | {
      readonly stage: "saved";
      readonly offer: SaveOffer;
      readonly choice: SaveChoice;
      readonly result: SaveResult;
    };

/** A closed offer keeps what it showed while its window goes. */
export type OfferView =
  | ShownView
  | { readonly stage: "closed"; readonly shown: ShownView };

export interface OfferRequests {
  readonly review: () => Promise<Answers["offer-review"]>;
  readonly save: (choice: SaveChoice) => Promise<Answers["offer-save"]>;
  readonly discard: () => Promise<Answers["offer-discard"]>;
  readonly unlock: () => Promise<Answers["menu-unlock"]>;
  readonly watchVault: VaultWatcher;
  readonly close: () => void;
}

/** Closes when the capture expires or is gone. */
export class OfferPrompt extends LockedPrompt<OfferView> {
  private readonly requests: OfferRequests;
  private expiry: ReturnType<typeof setTimeout> | undefined;
  private pause: ReturnType<typeof setTimeout> | undefined;

  constructor(offer: SaveOffer, requests: OfferRequests) {
    super(requests.watchVault, asking(offer));
    this.requests = requests;
  }

  override start(): void {
    const view = this.current;
    if (this.running || view.stage === "closed") return;
    super.start();
    this.expiry = setTimeout(
      () => this.close(),
      Math.max(view.offer.expiresAt - Date.now(), 0),
    );
    if (view.stage === "saved") this.closeLater();
  }

  override stop(): void {
    clearTimeout(this.expiry);
    clearTimeout(this.pause);
    super.stop();
  }

  edit(change: Partial<SaveChoice>): void {
    const view = this.current;
    if (view.stage !== "asking" || view.saving) return;
    this.update({
      ...view,
      ...reviseChoice(view.choice, view.refused, change),
    });
  }

  async save(): Promise<void> {
    const view = this.current;
    if (
      view.stage !== "asking" ||
      view.saving ||
      view.offer.state !== "ready"
    ) {
      return;
    }
    const run = this.run;
    this.update({ ...view, saving: true, failed: false, refused: null });
    const answer = await this.requests
      .save(view.choice)
      .catch(() => ({ ok: false, reason: "failed" }) as const);
    if (run !== this.run) return;
    if (answer.ok) {
      this.update({
        stage: "saved",
        offer: view.offer,
        choice: view.choice,
        result: saveResultOf(
          answer.saved === "created",
          view.offer,
          view.choice,
        ),
      });
      this.closeLater();
      return;
    }
    switch (answer.reason) {
      case "locked":
        this.update(asking({ ...view.offer, state: "locked" }));
        break;
      case "not-open":
        this.update({ stage: "not-open", offer: view.offer });
        break;
      case "gone":
        this.close();
        break;
      case "failed":
        this.update({ ...view, saving: false, failed: true, refused: null });
        break;
      case "invalid-account":
        this.update({ ...view, saving: false, refused: "account" });
        break;
      case "invalid-name":
        this.update({ ...view, saving: false, refused: "name" });
        break;
    }
  }

  /** Discards the capture and closes, even when the discard fails. */
  async dismiss(): Promise<void> {
    const view = this.current;
    if (view.stage === "closed" || (view.stage === "asking" && view.saving)) {
      return;
    }
    await sendIgnoringClosedPort(this.requests.discard());
    this.close();
  }

  /** Brings Ravenpass forward; the vault state change that follows reviews the offer. */
  async unlock(): Promise<void> {
    const view = this.current;
    if (view.stage !== "asking" || view.offer.state !== "locked") return;
    const run = this.run;
    this.update({ ...view, unlockFailed: false });
    const answer = await this.requests
      .unlock()
      .catch(() => ({ ok: false, reason: "failed" }) as const);
    const now = this.current;
    if (run !== this.run || answer.ok || now.stage !== "asking") return;
    if (answer.reason === "not-open") {
      this.update({ stage: "not-open", offer: now.offer });
    } else if (answer.reason === "failed" && now.offer.state === "locked") {
      this.update({ ...now, unlockFailed: true });
    }
  }

  protected showsLocked(view: OfferView): boolean {
    return view.stage === "asking" && view.offer.state === "locked";
  }

  protected async review(): Promise<void> {
    const run = this.run;
    const answer = await this.requests
      .review()
      .catch(() => ({ ok: false, reason: "failed" }) as const);
    const view = this.current;
    if (run !== this.run || view.stage !== "asking") return;
    if (answer.ok) {
      if (answer.offer.state === "ready") this.update(asking(answer.offer));
    } else if (answer.reason === "gone") {
      this.close();
    } else if (answer.reason === "not-open") {
      this.update({ stage: "not-open", offer: view.offer });
    }
  }

  private closeLater(): void {
    if (!this.running) return;
    clearTimeout(this.pause);
    this.pause = setTimeout(() => this.close(), savedForMs);
  }

  private close(): void {
    const shown = this.current;
    if (shown.stage === "closed") return;
    this.stop();
    this.update({ stage: "closed", shown });
    this.requests.close();
  }
}

function asking(offer: SaveOffer): AskingView {
  return {
    stage: "asking",
    offer,
    choice: suggestedChoice(offer),
    saving: false,
    failed: false,
    refused: null,
    unlockFailed: false,
  };
}
