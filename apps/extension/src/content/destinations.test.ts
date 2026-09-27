import assert from "node:assert/strict";
import test from "node:test";
import {
  accepts,
  type DestinationReader,
  resolveDestination,
} from "./destinations.ts";

interface Node {
  readonly name: string;
  /** The file input the element is, or the one it labels. */
  readonly fileInput?: string;
  /** The enabled file input inside it. */
  readonly holds?: string;
  readonly takesDrop?: boolean;
  readonly control?: boolean;
  /** The file input the page opens a chooser for when the control is clicked. */
  readonly opens?: string;
}

class Page implements DestinationReader<Node, string> {
  readonly probed: string[] = [];
  readonly clicked: string[] = [];

  fileInputFor(node: Node): string | null {
    return node.fileInput ?? null;
  }

  enabledFileInputIn(node: Node): string | null {
    return node.holds ?? null;
  }

  takesDrop(node: Node): boolean {
    this.probed.push(node.name);
    return node.takesDrop ?? false;
  }

  isControl(node: Node): boolean {
    return node.control ?? false;
  }

  chooserOpenedBy(node: Node): string | null {
    this.clicked.push(node.name);
    return node.opens ?? null;
  }
}

const body: Node = { name: "body", takesDrop: true };

test("a file input right-clicked is the destination, and nothing is probed", () => {
  const page = new Page();
  const input = { name: "input", fileInput: "avatar" };

  assert.deepEqual(
    resolveDestination([input, { name: "form", holds: "other" }, body], page),
    { kind: "input", input: "avatar" },
  );
  assert.deepEqual(page.probed, []);
});

test("a label for a file input, or text inside one, leads to its input", () => {
  const label = { name: "label", fileInput: "passport" };
  for (const path of [
    [label, body],
    [{ name: "span" }, { name: "strong" }, label, body],
  ]) {
    assert.deepEqual(resolveDestination(path, new Page()), {
      kind: "input",
      input: "passport",
    });
  }
});

test("a file input hidden inside a drop zone up to three levels up is the destination", () => {
  const page = new Page();
  const path = [
    { name: "icon" },
    { name: "hint" },
    { name: "zone", holds: "hidden", takesDrop: true },
    body,
  ];

  assert.deepEqual(resolveDestination(path, page), {
    kind: "input",
    input: "hidden",
  });
  assert.deepEqual(page.probed, []);
});

test("a file input four levels up is too far away", () => {
  const page = new Page();
  const path = [
    { name: "a" },
    { name: "b" },
    { name: "c" },
    { name: "d" },
    { name: "e", holds: "far" },
  ];

  assert.equal(resolveDestination(path, page), null);
  assert.deepEqual(page.probed, ["a", "b", "c", "d"]);
});

test("a drop zone without an input is found by probing the target, then its ancestors", () => {
  const page = new Page();
  const zone = { name: "zone", takesDrop: true };

  assert.deepEqual(resolveDestination([{ name: "text" }, zone, body], page), {
    kind: "drop-zone",
    zone,
  });
  assert.deepEqual(page.probed, ["text", "zone"]);
});

test("plain text takes no file", () => {
  const page = new Page();
  const path = [
    { name: "p" },
    { name: "article" },
    { name: "main" },
    { name: "div" },
    body,
  ];

  assert.equal(resolveDestination(path, page), null);
  assert.deepEqual(page.probed, ["p", "article", "main", "div"]);
  assert.deepEqual(page.clicked, []);
  assert.equal(resolveDestination([], page), null);
});

test("a button that opens the file chooser leads to the input it opens, after the drag probe", () => {
  const page = new Page();
  const path = [
    { name: "label" },
    { name: "menu-item", control: true, opens: "image" },
    { name: "menu", control: true, opens: "other" },
    { name: "body" },
  ];

  assert.deepEqual(resolveDestination(path, page), {
    kind: "input",
    input: "image",
  });
  assert.deepEqual(page.probed, ["label", "menu-item", "menu", "body"]);
  assert.deepEqual(page.clicked, ["menu-item"]);
});

test("only the first control is clicked, and one that opens no chooser takes no file", () => {
  const page = new Page();
  const path = [
    { name: "add-link", control: true },
    { name: "toolbar", control: true, opens: "image" },
    { name: "body" },
  ];

  assert.equal(resolveDestination(path, page), null);
  assert.deepEqual(page.clicked, ["add-link"]);
});

test("a control is never clicked when a nearer rule finds the destination", () => {
  const page = new Page();
  const zone = { name: "zone", control: true, takesDrop: true, opens: "x" };

  assert.deepEqual(resolveDestination([zone, body], page), {
    kind: "drop-zone",
    zone,
  });
  assert.deepEqual(page.clicked, []);
});

const jpeg = { name: "Passport.JPG", mediaType: "image/jpeg" };
const pdf = { name: "passport.pdf", mediaType: "application/pdf" };

test("no accept attribute lets every file through", () => {
  for (const accept of ["", "  ", ",", "image", "text/plain;charset=utf-8"]) {
    assert.ok(accepts(accept, jpeg), accept);
    assert.ok(accepts(accept, pdf), accept);
  }
});

test("media types in accept match exactly, whatever their case", () => {
  assert.ok(accepts("image/jpeg", jpeg));
  assert.ok(accepts("IMAGE/JPEG", jpeg));
  assert.ok(accepts("image/png, application/pdf", pdf));
  assert.ok(!accepts("image/png", jpeg));
  assert.ok(!accepts("image/jpeg", pdf));
});

test("a type wildcard in accept matches every subtype of the type", () => {
  assert.ok(accepts("image/*", jpeg));
  assert.ok(accepts("Image/*", jpeg));
  assert.ok(!accepts("image/*", pdf));
  assert.ok(accepts("image/*,.pdf", pdf));
});

test("an extension in accept matches the end of the file's name, whatever its case", () => {
  assert.ok(accepts(".jpg", jpeg));
  assert.ok(accepts(".PDF", pdf));
  assert.ok(accepts(" .png , .jpg ", jpeg));
  assert.ok(!accepts(".jpeg", jpeg));
  assert.ok(!accepts(".png,.gif", pdf));
});
