import { failureCode } from "../failures.ts";
import type {
  ExtensionLinking,
  ExtensionLinkOffer,
  LinkedExtension,
} from "../vault-api.ts";

export type LinkEvent =
  | { kind: "offered"; offer: ExtensionLinkOffer }
  | { kind: "linked"; extension: LinkedExtension }
  | { kind: "expired" }
  | { kind: "failed"; cause: unknown };

/** The host keeps one waiting key and a new one replaces it, so each offer awaits the previous key's creation. */
export class ExtensionLinker {
  private readonly linking: ExtensionLinking;
  private lastCreation: Promise<void> = Promise.resolve();

  constructor(linking: ExtensionLinking) {
    this.linking = linking;
  }

  /** The returned function stops the offer: it reports nothing more and leaves no key waiting. */
  offer(report: (event: LinkEvent) => void): () => void {
    const controller = new AbortController();
    const previous = this.lastCreation;
    let created = () => {};
    this.lastCreation = new Promise<void>((resolve) => {
      created = resolve;
    });
    void this.run(previous, controller.signal, report, created);
    return () => controller.abort();
  }

  private async run(
    previous: Promise<void>,
    signal: AbortSignal,
    report: (event: LinkEvent) => void,
    created: () => void,
  ): Promise<void> {
    await previous;
    let offer: ExtensionLinkOffer | null = null;
    try {
      if (!signal.aborted) offer = await this.linking.beginExtensionLink();
    } catch (cause) {
      if (!signal.aborted) report({ kind: "failed", cause });
    }
    if (offer && signal.aborted) {
      // A stopped offer has no listener left; a key that stays waiting still expires.
      await this.linking.cancelExtensionLink().catch(() => {});
    }
    created();
    if (!offer || signal.aborted) return;

    report({ kind: "offered", offer });
    try {
      const extension = await this.linking.awaitExtensionLink(signal);
      if (!signal.aborted) report({ kind: "linked", extension });
    } catch (cause) {
      if (signal.aborted) return;
      report(
        failureCode(cause) === "link-expired"
          ? { kind: "expired" }
          : { kind: "failed", cause },
      );
    }
  }
}
