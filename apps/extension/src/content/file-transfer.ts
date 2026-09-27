import type { Destination, DestinationReader } from "./destinations.ts";
import { announceChange } from "./fill.ts";

/** A point in the viewport, as a mouse event reports it. */
export interface Point {
  readonly x: number;
  readonly y: number;
}

export type PageDestination = Destination<Element, HTMLInputElement>;

const controlSelector = [
  "button",
  "a[href]",
  "summary",
  "input[type=button]",
  "input[type=submit]",
  "input[type=image]",
  "[role=button]",
  "[role=link]",
  "[role=menuitem]",
].join(",");

/** Misses a chooser opened after the click handler, through `showPicker()`, or on a detached input. */
export class PageFiles implements DestinationReader<Element, HTMLInputElement> {
  private readonly point: Point;

  constructor(point: Point) {
    this.point = point;
  }

  fileInputFor(element: Element): HTMLInputElement | null {
    if (isFileInput(element)) return element;
    if (element instanceof HTMLLabelElement && isFileInput(element.control)) {
      return element.control;
    }
    return null;
  }

  enabledFileInputIn(element: Element): HTMLInputElement | null {
    return element.querySelector<HTMLInputElement>("input[type=file]:enabled");
  }

  takesDrop(element: Element): boolean {
    const transfer = transferOf(new File([], ""));
    const taken = !element.dispatchEvent(
      dragEvent("dragover", transfer, this.point),
    );
    element.dispatchEvent(dragEvent("dragleave", transfer, this.point));
    return taken;
  }

  isControl(element: Element): boolean {
    return element instanceof HTMLElement && element.matches(controlSelector);
  }

  chooserOpenedBy(control: Element): HTMLInputElement | null {
    const opened: { input: HTMLInputElement | null } = { input: null };
    const intercept = (event: Event): void => {
      const target = event.composedPath()[0];
      if (!isFileInput(target)) return;
      event.preventDefault();
      opened.input = target;
    };
    addEventListener("click", intercept, true);
    try {
      if (control instanceof HTMLElement) control.click();
    } finally {
      removeEventListener("click", intercept, true);
    }
    return opened.input;
  }
}

export function placeFile(
  destination: PageDestination,
  file: File,
  point: Point,
): boolean {
  const transfer = transferOf(file);
  if (destination.kind === "input") {
    return chooseFile(destination.input, file, transfer);
  }
  const { zone } = destination;
  if (!zone.isConnected) return false;
  zone.dispatchEvent(dragEvent("dragenter", transfer, point));
  zone.dispatchEvent(dragEvent("dragover", transfer, point));
  return !zone.dispatchEvent(dragEvent("drop", transfer, point));
}

/** Reads `files` back before the events: pages often empty the input once they read it. */
function chooseFile(
  input: HTMLInputElement,
  file: File,
  transfer: DataTransfer,
): boolean {
  if (!input.isConnected) return false;
  input.files = transfer.files;
  const held = Array.from(input.files ?? []).some(
    (chosen) =>
      chosen.name === file.name &&
      chosen.size === file.size &&
      chosen.type === file.type,
  );
  if (held) announceChange(input);
  return held;
}

function isFileInput(element: unknown): element is HTMLInputElement {
  return element instanceof HTMLInputElement && element.type === "file";
}

function transferOf(file: File): DataTransfer {
  const transfer = new DataTransfer();
  transfer.items.add(file);
  return transfer;
}

function dragEvent(
  type: "dragenter" | "dragover" | "dragleave" | "drop",
  dataTransfer: DataTransfer,
  point: Point,
): DragEvent {
  return new DragEvent(type, {
    bubbles: true,
    cancelable: type !== "dragleave",
    composed: true,
    dataTransfer,
    clientX: point.x,
    clientY: point.y,
  });
}
