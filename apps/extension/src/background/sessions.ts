import { type MenuContent, menuPage } from "../messages.ts";

const lifetimeMs = 10 * 60_000;
const keyPrefix = "menu:";
const menuPath = `/${menuPage}`;

/** Chrome fills in the sender; a page cannot forge it. */
export interface Sender {
  readonly id?: string;
  readonly url?: string;
  readonly origin?: string;
  /** Chrome reports `url`, the tab's top-frame document, to every content script's message. */
  readonly tab?: { readonly id?: number; readonly url?: string };
  readonly frameId?: number;
  readonly documentId?: string;
}

/** A relay addressed to a document never reaches the page that replaced it. */
export interface TabDocument {
  readonly tabId: number;
  readonly documentId: string;
}

/** Frame 0 is the top frame; a relay reaches whatever document the frame holds at that moment. */
export interface TabFrame {
  readonly tabId: number;
  readonly frameId: number;
}

/** `origin` is the one Chrome reported. */
export interface PageFrame extends TabDocument {
  readonly origin: string;
}

/** `menu` is the menu page's document id once that page has asked for its content. */
export interface MenuSession {
  readonly token: string;
  readonly page: PageFrame;
  readonly content: MenuContent;
  readonly menu: string | null;
  readonly expiresAt: number;
}

/** Chrome stops an idle service worker after 30 seconds while a menu may stay open. */
export interface SessionArea {
  get(keys: string | null): Promise<Record<string, unknown>>;
  set(items: Record<string, unknown>): Promise<void>;
  remove(keys: string | string[]): Promise<void>;
}

export interface MenuSessionsDependencies {
  readonly area: SessionArea;
  /** The extension's own ID, which Chrome reports as the sender of its content scripts and pages. */
  readonly extensionId: string;
  readonly now?: () => number;
  readonly newToken?: () => string;
}

/** A token binds to the asking frame's tab, document and origin, then to the first menu page that asks. */
export class MenuSessions {
  private readonly area: SessionArea;
  private readonly extensionId: string;
  private readonly now: () => number;
  private readonly newToken: () => string;

  constructor({
    area,
    extensionId,
    now = Date.now,
    newToken = () => crypto.randomUUID(),
  }: MenuSessionsDependencies) {
    this.area = area;
    this.extensionId = extensionId;
    this.now = now;
    this.newToken = newToken;
  }

  pageOf(sender: Sender): PageFrame | null {
    const tabId = sender.tab?.id;
    if (
      sender.id !== this.extensionId ||
      tabId === undefined ||
      sender.frameId === undefined ||
      !sender.documentId ||
      !sender.origin ||
      !isWebOrigin(sender.origin)
    ) {
      return null;
    }
    return { tabId, documentId: sender.documentId, origin: sender.origin };
  }

  async open(page: PageFrame, content: MenuContent): Promise<string> {
    await this.sweep();
    const session: MenuSession = {
      token: this.newToken(),
      page,
      content,
      menu: null,
      expiresAt: this.now() + lifetimeMs,
    };
    await this.area.set({ [keyPrefix + session.token]: session });
    return session.token;
  }

  /** The first menu document to ask keeps the token; any other is refused. */
  async bind(token: string, sender: Sender): Promise<MenuSession | null> {
    const session = await this.load(token);
    const menu = this.menuOf(sender);
    if (!session || !menu || menu.tabId !== session.page.tabId) return null;
    if (session.menu === null) {
      const bound = { ...session, menu: menu.documentId };
      await this.area.set({ [keyPrefix + token]: bound });
      return bound;
    }
    return session.menu === menu.documentId ? session : null;
  }

  async forMenu(token: string, sender: Sender): Promise<MenuSession | null> {
    const session = await this.load(token);
    const menu = this.menuOf(sender);
    if (
      !session ||
      !menu ||
      menu.tabId !== session.page.tabId ||
      menu.documentId !== session.menu
    ) {
      return null;
    }
    return session;
  }

  async forPage(token: string, sender: Sender): Promise<MenuSession | null> {
    const session = await this.load(token);
    const page = this.pageOf(sender);
    if (
      !session ||
      !page ||
      page.tabId !== session.page.tabId ||
      page.documentId !== session.page.documentId ||
      page.origin !== session.page.origin
    ) {
      return null;
    }
    return session;
  }

  async forHost(token: string, sender: Sender): Promise<MenuSession | null> {
    const session = await this.load(token);
    if (!session || !inCorner(session.content)) {
      return this.forPage(token, sender);
    }
    const page = this.pageOf(sender);
    return page?.tabId === session.page.tabId && sender.frameId === 0
      ? session
      : null;
  }

  async cardOf(tabId: number): Promise<MenuSession | null> {
    const now = this.now();
    return (
      Object.values(await this.sessions()).find(
        (session) =>
          session.page.tabId === tabId &&
          inCorner(session.content) &&
          session.expiresAt > now,
      ) ?? null
    );
  }

  async fieldMenuIn(tabId: number): Promise<boolean> {
    const now = this.now();
    return Object.values(await this.sessions()).some(
      (session) =>
        session.page.tabId === tabId &&
        listsCredentials(session.content) &&
        session.expiresAt > now,
    );
  }

  async revise(token: string, content: MenuContent): Promise<void> {
    const session = await this.load(token);
    if (session) {
      await this.area.set({ [keyPrefix + token]: { ...session, content } });
    }
  }

  async end(token: string): Promise<void> {
    await this.area.remove(keyPrefix + token);
  }

  async endTab(tabId: number): Promise<void> {
    const ended = Object.entries(await this.sessions())
      .filter(([, session]) => session.page.tabId === tabId)
      .map(([key]) => key);
    if (ended.length) await this.area.remove(ended);
  }

  /** `use_dynamic_url` gives the menu page a per-session host; only its scheme and path are stable. */
  private menuOf(sender: Sender): TabDocument | null {
    const tabId = sender.tab?.id;
    if (
      sender.id !== this.extensionId ||
      tabId === undefined ||
      !sender.documentId ||
      !sender.url ||
      !URL.canParse(sender.url)
    ) {
      return null;
    }
    const url = new URL(sender.url);
    if (url.protocol !== "chrome-extension:" || url.pathname !== menuPath) {
      return null;
    }
    return { tabId, documentId: sender.documentId };
  }

  private async load(token: string): Promise<MenuSession | null> {
    const key = keyPrefix + token;
    const session = (await this.area.get(key))[key] as MenuSession | undefined;
    if (!session) return null;
    if (session.expiresAt <= this.now()) {
      await this.area.remove(key);
      return null;
    }
    return session;
  }

  private async sweep(): Promise<void> {
    const now = this.now();
    const expired = Object.entries(await this.sessions())
      .filter(([, session]) => session.expiresAt <= now)
      .map(([key]) => key);
    if (expired.length) await this.area.remove(expired);
  }

  private async sessions(): Promise<Record<string, MenuSession>> {
    const items = await this.area.get(null);
    return Object.fromEntries(
      Object.entries(items).filter(([key]) => key.startsWith(keyPrefix)),
    ) as Record<string, MenuSession>;
  }
}

/** A card is framed by the tab's top frame; any other menu by the frame it opened for. */
export function hostOf(session: MenuSession): TabDocument | TabFrame {
  return inCorner(session.content)
    ? { tabId: session.page.tabId, frameId: 0 }
    : session.page;
}

/** The origin of the document the sender's tab holds in its top frame, as Chrome reports it, or null for none. */
export function topOriginOf(sender: Sender): string | null {
  const url = sender.tab?.url;
  return url && URL.canParse(url) ? new URL(url).origin : null;
}

export function menuDocumentOf(session: MenuSession): TabDocument | null {
  return session.menu === null
    ? null
    : { tabId: session.page.tabId, documentId: session.menu };
}

/** Cards show in the tab's top frame one at a time; a save offer shows there too but is not a card. */
function inCorner(content: MenuContent): boolean {
  return content.state === "sign-in-card" || content.state === "passkey";
}

/** A field's menu of passwords or codes, as opposed to a card, a save offer or a file menu. */
function listsCredentials(content: MenuContent): boolean {
  return (
    content.state === "list" ||
    content.state === "locked" ||
    content.state === "not-open"
  );
}

function isWebOrigin(origin: string): boolean {
  if (!URL.canParse(origin)) return false;
  const { protocol } = new URL(origin);
  return protocol === "http:" || protocol === "https:";
}
