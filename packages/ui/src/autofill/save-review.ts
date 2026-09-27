import {
  type RefusedField,
  reviseChoice,
  type SaveChoice,
  suggestedChoice,
} from "../saving/offer.ts";
import type { AutofillApi, SaveReviewOffer } from "./autofill-api.ts";
import { Watched } from "./watched.ts";

export interface SaveReviewView {
  readonly choice: SaveChoice;
  readonly saving: boolean;
  readonly refused: RefusedField | null;
  readonly failed: boolean;
}

/** SaveReview runs the save review; a refused name or account stays marked until it changes. */
export class SaveReview {
  readonly offer: SaveReviewOffer;
  readonly state: Watched<SaveReviewView>;
  readonly #save: AutofillApi["save"];

  constructor(offer: SaveReviewOffer, save: AutofillApi["save"]) {
    this.offer = offer;
    this.#save = save;
    this.state = new Watched<SaveReviewView>({
      choice: suggestedChoice(offer),
      saving: false,
      refused: null,
      failed: false,
    });
  }

  edit(change: Partial<SaveChoice>): void {
    const view = this.state.get();
    if (view.saving) return;
    this.state.set({
      ...view,
      ...reviseChoice(view.choice, view.refused, change),
    });
  }

  async save(): Promise<void> {
    const view = this.state.get();
    if (view.saving) return;
    this.state.set({ ...view, saving: true, refused: null, failed: false });
    const outcome = await this.#save(view.choice).catch(
      () => ({ kind: "failed" }) as const,
    );
    const now = this.state.get();
    switch (outcome.kind) {
      case "saved":
        return;
      case "name-refused":
        this.state.set({ ...now, saving: false, refused: "name" });
        return;
      case "account-refused":
        this.state.set({ ...now, saving: false, refused: "account" });
        return;
      case "failed":
        this.state.set({ ...now, saving: false, failed: true });
        return;
    }
  }
}
