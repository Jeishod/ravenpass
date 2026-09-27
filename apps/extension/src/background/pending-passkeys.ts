import type { PageRequest } from "../passkeys/requests.ts";
import type { PageFrame, SessionArea, TabDocument } from "./sessions.ts";

const keyPrefix = "passkey:";

/** `id` is the one the page's script gave; `page.origin` is the one Chrome reported. */
export interface PendingPasskey {
  readonly page: PageFrame;
  readonly id: number;
  readonly request: PageRequest;
}

export interface PendingPasskeysDependencies {
  readonly area: SessionArea;
}

/** Session storage outlives the service worker; a document keeps only its latest conditional request. */
export class PendingPasskeys {
  private readonly area: SessionArea;

  constructor({ area }: PendingPasskeysDependencies) {
    this.area = area;
  }

  async add(pending: PendingPasskey): Promise<void> {
    const { page, request } = pending;
    const replaced = (await this.entries())
      .filter(
        ([, other]) =>
          other.page.tabId === page.tabId &&
          (other.page.documentId !== page.documentId ||
            (isConditional(other.request) && isConditional(request))),
      )
      .map(([key]) => key);
    if (replaced.length) await this.area.remove(replaced);
    await this.area.set({ [keyOf(page, pending.id)]: pending });
  }

  async find(page: TabDocument, id: number): Promise<PendingPasskey | null> {
    const key = keyOf(page, id);
    return (
      ((await this.area.get(key))[key] as PendingPasskey | undefined) ?? null
    );
  }

  async take(page: TabDocument, id: number): Promise<PendingPasskey | null> {
    const pending = await this.find(page, id);
    if (pending) await this.area.remove(keyOf(page, id));
    return pending;
  }

  async conditionalIn(page: TabDocument): Promise<PendingPasskey | null> {
    return (
      (await this.entries())
        .map(([, pending]) => pending)
        .find(
          (pending) =>
            sameDocument(pending.page, page) && isConditional(pending.request),
        ) ?? null
    );
  }

  async modalIn(page: TabDocument): Promise<boolean> {
    return (await this.entries()).some(
      ([, pending]) =>
        sameDocument(pending.page, page) && !isConditional(pending.request),
    );
  }

  async forget(tabId: number): Promise<void> {
    const ended = (await this.entries())
      .filter(([, pending]) => pending.page.tabId === tabId)
      .map(([key]) => key);
    if (ended.length) await this.area.remove(ended);
  }

  private async entries(): Promise<[string, PendingPasskey][]> {
    return Object.entries(await this.area.get(null)).filter(([key]) =>
      key.startsWith(keyPrefix),
    ) as [string, PendingPasskey][];
  }
}

function keyOf(page: TabDocument, id: number): string {
  return `${keyPrefix}${page.tabId}:${page.documentId}:${id}`;
}

function sameDocument(one: TabDocument, other: TabDocument): boolean {
  return one.tabId === other.tabId && one.documentId === other.documentId;
}

function isConditional(request: PageRequest): boolean {
  return request.mode === "get" && request.conditional;
}
