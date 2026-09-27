import { matchLanguage } from "@ravenpass/ui/i18n/language.ts";
import { catalogs } from "@ravenpass/ui/i18n/messages.ts";
import type { StoredLanguage } from "../language.ts";
import type { FileMenuOpen } from "../messages.ts";
import { sendIgnoringClosedPort } from "../messaging/send.ts";

const parentId = "ravenpass";
const uploadId = "upload-identity-file";

/** `all` would also cover the toolbar button's and tab strip's menus, which have no frame. */
const pageContexts: chrome.contextMenus.CreateProperties["contexts"] = [
  "page",
  "frame",
  "selection",
  "link",
  "editable",
  "image",
  "video",
  "audio",
];

/** Chrome builds the menu before the right-click and reports no target; the clicked frame decides the file window. */
export class UploadMenu {
  private readonly language: StoredLanguage;

  constructor(language: StoredLanguage) {
    this.language = language;
  }

  async create(): Promise<void> {
    await chrome.contextMenus.removeAll();
    const title = await this.title();
    chrome.contextMenus.create({
      id: parentId,
      title: "Ravenpass",
      contexts: pageContexts,
    });
    chrome.contextMenus.create({
      id: uploadId,
      parentId,
      title,
      contexts: pageContexts,
    });
  }

  async retitle(): Promise<void> {
    await chrome.contextMenus.update(uploadId, { title: await this.title() });
  }

  async clicked(
    info: chrome.contextMenus.OnClickData,
    tab: chrome.tabs.Tab | undefined,
  ): Promise<void> {
    if (info.menuItemId !== uploadId || tab?.id === undefined) return;
    const message: FileMenuOpen = { kind: "file-menu-open" };
    await sendIgnoringClosedPort(
      chrome.tabs.sendMessage(tab.id, message, { frameId: info.frameId ?? 0 }),
    );
  }

  private async title(): Promise<string> {
    const { language } = await this.language.getLanguage();
    return catalogs[matchLanguage(language)]["extension.upload.menu"];
  }
}
