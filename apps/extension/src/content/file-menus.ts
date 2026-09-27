import { base64 } from "@scure/base";
import {
  ask,
  isFileMenuOpen,
  isFrameMessage,
  menuPage,
  type Placement,
} from "../messages.ts";
import { sendIgnoringClosedPort } from "../messaging/send.ts";
import { resolveDestination } from "./destinations.ts";
import {
  type PageDestination,
  PageFiles,
  type Point,
  placeFile,
} from "./file-transfer.ts";
import { MenuFrame, PointAnchor } from "./menu-frame.ts";

/** A right-click in this frame: the elements of its composed path, target first, and its point. */
interface RightClick {
  readonly path: readonly Element[];
  readonly point: Point;
}

interface OpenWindow {
  readonly token: string;
  /** Where a shared file goes, or null for a window saying the spot takes no file. */
  readonly destination: PageDestination | null;
  readonly point: Point;
  readonly frame: MenuFrame;
}

/** The page never sees what the window lists, and no file is kept once placed. */
export class FileMenus {
  private readonly document: Document;
  private rightClick: RightClick | null = null;
  private open: OpenWindow | null = null;
  /** Counts the windows asked for; an answer to any but the latest is dropped. */
  private asked = 0;

  constructor(document: Document) {
    this.document = document;
  }

  start(): void {
    this.document.addEventListener("contextmenu", this.onContextMenu, true);
    this.document.addEventListener("pointerdown", this.onPointerDown, true);
    addEventListener("pagehide", this.onPageHide);
    chrome.runtime.onMessage.addListener(this.onMessage);
  }

  stop(): void {
    this.document.removeEventListener("contextmenu", this.onContextMenu, true);
    this.document.removeEventListener("pointerdown", this.onPointerDown, true);
    removeEventListener("pagehide", this.onPageHide);
    chrome.runtime.onMessage.removeListener(this.onMessage);
    this.close({ tell: true });
  }

  private readonly onContextMenu = (event: MouseEvent): void => {
    this.rightClick = {
      path: event.composedPath().filter((target) => target instanceof Element),
      point: { x: event.clientX, y: event.clientY },
    };
  };

  private readonly onPointerDown = (event: PointerEvent): void => {
    if (this.open && event.composedPath()[0] !== this.open.frame.host) {
      this.close({ tell: true });
    }
  };

  private readonly onPageHide = (): void => {
    this.close({ tell: true });
  };

  private readonly onMessage = (
    message: unknown,
    sender: chrome.runtime.MessageSender,
    respond: (placement: Placement) => void,
  ): undefined => {
    if (sender.id !== chrome.runtime.id) return;
    if (isFileMenuOpen(message)) {
      void this.openWindow();
      return;
    }
    const open = this.open;
    if (!open || !isFrameMessage(message, open.token)) return;
    switch (message.kind) {
      case "menu-size":
        open.frame.resize(message.height);
        break;
      case "menu-close":
        this.close({ tell: false });
        break;
      case "place-file": {
        const file = new File(
          [Uint8Array.from(base64.decode(message.content))],
          message.name,
          { type: message.mediaType },
        );
        const placed =
          open.destination !== null &&
          placeFile(open.destination, file, open.point);
        respond({ placed });
        if (placed) this.close({ tell: false });
        break;
      }
    }
  };

  private async openWindow(): Promise<void> {
    const rightClick = this.rightClick;
    this.rightClick = null;
    this.close({ tell: true });
    if (!rightClick) return;
    this.asked += 1;
    const asked = this.asked;
    const destination = resolveDestination(
      rightClick.path,
      new PageFiles(rightClick.point),
    );
    const answer = await sendIgnoringClosedPort(
      ask({
        kind: "file-menu",
        destination: destination && {
          accept: destination.kind === "input" ? destination.input.accept : "",
        },
      }),
    );
    const token = answer?.token;
    if (!token) return;
    if (asked !== this.asked) {
      void sendIgnoringClosedPort(ask({ kind: "menu-close", token }));
      return;
    }
    const frame = new MenuFrame(
      new PointAnchor(this.document, rightClick.point),
      `${chrome.runtime.getURL(menuPage)}#${token}`,
      () => this.close({ tell: true }),
    );
    this.open = { token, destination, point: rightClick.point, frame };
    frame.focus();
  }

  /** `tell` is false when the service worker already ended the token. */
  private close({ tell }: { tell: boolean }): void {
    const open = this.open;
    if (!open) return;
    this.open = null;
    open.frame.remove();
    if (tell) {
      void sendIgnoringClosedPort(
        ask({ kind: "menu-close", token: open.token }),
      );
    }
  }
}
