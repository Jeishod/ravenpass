import type {
  AutofillApi,
  AutofillCredential,
  AutofillPasskey,
  AutofillResults,
  FillOutcome,
  SearchOpening,
  SignInOutcome,
} from "./autofill-api.ts";
import { Watched } from "./watched.ts";

/** What a credential that does not match yet needs before it fills. */
export type Addition = "add-site" | "link";

export type FillNote = Exclude<FillOutcome, "filled">;

export type SignInNote = Exclude<SignInOutcome, "signed">;

export interface SearchScreenView {
  readonly query: string;
  readonly results: AutofillResults;
  readonly searchFailed: boolean;
  /** The credential waiting for the owner to agree to an addition. */
  readonly asking: {
    readonly credential: AutofillCredential;
    readonly addition: Addition;
  } | null;
  /** The id of the credential being filled. */
  readonly filling: string | null;
  /** The key of the passkey signing in. */
  readonly signingIn: string | null;
  readonly note: FillNote | null;
  readonly signInNote: SignInNote | null;
}

/** SearchScreen runs the search screen; a credential that does not match asks for its addition before it fills. */
export class SearchScreen {
  readonly state: Watched<SearchScreenView>;
  readonly #opening: SearchOpening;
  readonly #api: Pick<AutofillApi, "search" | "fill" | "signIn">;
  /** Counts queries; an answer a later query overtook is dropped. */
  #queries = 0;

  constructor(
    opening: SearchOpening,
    api: Pick<AutofillApi, "search" | "fill" | "signIn">,
  ) {
    this.#opening = opening;
    this.#api = api;
    this.state = new Watched<SearchScreenView>({
      query: "",
      results: opening,
      searchFailed: false,
      asking: null,
      filling: null,
      signingIn: null,
      note: null,
      signInNote: null,
    });
  }

  /** False when an app asks. */
  get forSite(): boolean {
    return this.#opening.site !== "";
  }

  /** Whether the screen answers a passkey request, and lists passkeys alone. */
  get passkeyOnly(): boolean {
    return this.#opening.passkey;
  }

  /** A query narrows the site's passkeys only for a passkey request, and hides them otherwise. */
  get passkeys(): readonly AutofillPasskey[] {
    const query = this.state.get().query.trim().toLowerCase();
    if (query === "") return this.#opening.passkeys;
    if (!this.#opening.passkey) return [];
    return this.#opening.passkeys.filter((passkey) =>
      [passkey.label, passkey.account, passkey.site].some((text) =>
        text.toLowerCase().includes(query),
      ),
    );
  }

  /** Whether a credential fills or a passkey signs in. */
  get busy(): boolean {
    const { filling, signingIn } = this.state.get();
    return filling !== null || signingIn !== null;
  }

  async type(query: string): Promise<void> {
    this.#queries += 1;
    const turn = this.#queries;
    const view = this.state.get();
    if (query.trim() === "" || this.#opening.passkey) {
      this.state.set({
        ...view,
        query,
        results: this.#opening,
        searchFailed: false,
      });
      return;
    }
    this.state.set({ ...view, query });
    const results = await this.#api.search(query).catch(() => null);
    if (turn !== this.#queries) return;
    const now = this.state.get();
    this.state.set(
      results
        ? { ...now, results, searchFailed: false }
        : { ...now, searchFailed: true },
    );
  }

  choose(credential: AutofillCredential): void {
    if (this.busy) return;
    if (credential.matches) {
      void this.#fill(credential, false);
      return;
    }
    this.state.set({
      ...this.state.get(),
      asking: { credential, addition: this.forSite ? "add-site" : "link" },
      note: null,
      signInNote: null,
    });
  }

  /** Adds the site or the app to the credential waiting for it, and fills. */
  confirm(): void {
    const { asking } = this.state.get();
    if (asking && !this.busy) void this.#fill(asking.credential, true);
  }

  decline(): void {
    if (!this.busy) this.state.set({ ...this.state.get(), asking: null });
  }

  async signIn(passkey: AutofillPasskey): Promise<void> {
    if (this.busy) return;
    this.state.set({
      ...this.state.get(),
      signingIn: passkey.key,
      note: null,
      signInNote: null,
    });
    const outcome = await this.#api
      .signIn(passkey)
      .catch(() => "failed" as const);
    if (outcome === "signed") return;
    this.state.set({
      ...this.state.get(),
      signingIn: null,
      signInNote: outcome,
    });
  }

  async #fill(credential: AutofillCredential, add: boolean): Promise<void> {
    this.state.set({
      ...this.state.get(),
      filling: credential.id,
      note: null,
      signInNote: null,
    });
    const outcome = await this.#api
      .fill(credential, add)
      .catch(() => "failed" as const);
    if (outcome === "filled") return;
    this.state.set({
      ...this.state.get(),
      asking: null,
      filling: null,
      note: outcome,
    });
  }
}
