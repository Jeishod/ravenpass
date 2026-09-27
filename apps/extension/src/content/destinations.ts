export type Destination<Element, Input> =
  | { readonly kind: "input"; readonly input: Input }
  | { readonly kind: "drop-zone"; readonly zone: Element };

export interface DestinationReader<Element, Input> {
  /** The file input the element is, or the file input it labels when it is a label. */
  fileInputFor(element: Element): Input | null;
  enabledFileInputIn(element: Element): Input | null;
  /** Whether the page takes a file dragged over the element. */
  takesDrop(element: Element): boolean;
  isControl(element: Element): boolean;
  /** Clicks the control with the chooser kept from opening; null when the page opened none. */
  chooserOpenedBy(control: Element): Input | null;
}

/** Drop zones hide their file input inside themselves, a few wrappers up from what is shown. */
const ancestorLevels = 3;

/** `path` is the composed path, target first. Probes run last, drag before click: both run page code. */
export function resolveDestination<Element, Input>(
  path: readonly Element[],
  page: DestinationReader<Element, Input>,
): Destination<Element, Input> | null {
  for (const element of path) {
    const input = page.fileInputFor(element);
    if (input !== null) return { kind: "input", input };
  }
  const nearby = path.slice(0, ancestorLevels + 1);
  for (const element of nearby) {
    const input = page.enabledFileInputIn(element);
    if (input !== null) return { kind: "input", input };
  }
  const zone = nearby.find((element) => page.takesDrop(element));
  if (zone !== undefined) return { kind: "drop-zone", zone };
  const control = nearby.find((element) => page.isControl(element));
  const input = control === undefined ? null : page.chooserOpenedBy(control);
  return input === null ? null : { kind: "input", input };
}

export interface FileType {
  readonly name: string;
  readonly mediaType: string;
}

/** An `accept` without a valid token lets every file through, as the browser's file chooser does. */
export function accepts(accept: string, file: FileType): boolean {
  const tokens = accept
    .split(",")
    .map((token) => token.trim().toLowerCase())
    .filter(isAcceptToken);
  if (!tokens.length) return true;
  const name = file.name.toLowerCase();
  const mediaType = file.mediaType.toLowerCase();
  return tokens.some((token) => {
    if (token.startsWith(".")) return name.endsWith(token);
    if (token.endsWith("/*")) return mediaType.startsWith(token.slice(0, -1));
    return mediaType === token;
  });
}

function isAcceptToken(token: string): boolean {
  if (token.startsWith(".")) return token.length > 1;
  const [type, subtype, ...rest] = token.split("/");
  return !rest.length && !!type && !!subtype && !token.includes(";");
}
