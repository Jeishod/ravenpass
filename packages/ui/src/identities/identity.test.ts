import assert from "node:assert/strict";
import test from "node:test";
import type { IdentityInput, ScanSummary } from "../vault-api.ts";
import { emptyIdentity } from "./identity.test-support.ts";
import {
  addressName,
  blankAddress,
  emptyAddress,
  emptyDocument,
  isDocumentType,
  isScanMediaType,
  labelled,
  readyToSave,
  scansOf,
  stagedScan,
  storedScans,
  unnamedDocuments,
} from "./identity.ts";

test("rows left blank are dropped before saving", () => {
  const input: IdentityInput = {
    ...emptyIdentity,
    label: "Me",
    emails: ["", "me@example.com", "  "],
    phones: [" "],
    addresses: [emptyAddress, { ...emptyAddress, city: "Almaty" }],
    documents: [
      emptyDocument("passport"),
      { ...emptyDocument("id-card"), number: "N123" },
    ],
  };
  assert.deepEqual(readyToSave(input), {
    ...input,
    emails: ["me@example.com"],
    phones: [],
    addresses: [{ ...emptyAddress, city: "Almaty" }],
    documents: [{ ...emptyDocument("id-card"), number: "N123" }],
  });
});

test("an address keeps its id and is blank when nothing typed is in it", () => {
  const stored = { ...emptyAddress, id: "a1b2c3d4e5f60718" };
  assert.ok(blankAddress(stored));
  assert.ok(!blankAddress({ ...stored, country: "Kazakhstan" }));
  const input: IdentityInput = {
    ...emptyIdentity,
    label: "Me",
    addresses: [stored, { ...stored, city: "Almaty" }],
  };
  assert.deepEqual(readyToSave(input).addresses, [
    { ...stored, city: "Almaty" },
  ]);
});

test("an address is named by its label, or else its first filled part", () => {
  assert.equal(addressName({ ...emptyAddress, label: " Home " }), "Home");
  assert.equal(
    addressName({ ...emptyAddress, city: "Almaty", country: "Kazakhstan" }),
    "Almaty",
  );
  assert.equal(addressName(emptyAddress), "");
});

test("a row holding anything is kept as typed", () => {
  const input: IdentityInput = {
    ...emptyIdentity,
    label: "Me",
    emails: [" me@example.com "],
    addresses: [{ ...emptyAddress, label: "Home" }],
    documents: [{ ...emptyDocument("other"), expiresOn: "2030-01-01" }],
  };
  assert.deepEqual(readyToSave(input), input);
});

test("a typed document is saved without a label", () => {
  const input: IdentityInput = {
    ...emptyIdentity,
    label: "Me",
    documents: [
      { ...emptyDocument("passport"), label: "Old", number: "P1" },
      { ...emptyDocument("tax-number"), label: "Only a label" },
      { ...emptyDocument("other"), label: "Library card", number: "L1" },
    ],
  };
  assert.deepEqual(readyToSave(input).documents, [
    { ...emptyDocument("passport"), number: "P1" },
    { ...emptyDocument("other"), label: "Library card", number: "L1" },
  ]);
});

test("only another kind of document needs a name", () => {
  assert.ok(labelled("other"));
  assert.ok(!labelled("passport"));
  const named: IdentityInput = {
    ...emptyIdentity,
    documents: [
      { ...emptyDocument("other"), number: "1" },
      { ...emptyDocument("other"), label: "Card", number: "2" },
      emptyDocument("other"),
      { ...emptyDocument("passport"), number: "3" },
    ],
  };
  assert.equal(unnamedDocuments(named), 1);
});

const scan = (id: string): ScanSummary => ({
  id,
  name: `${id}.jpg`,
  mediaType: "image/jpeg",
  thumbnail: "",
});

test("a document lists its scans in its own order", () => {
  const document = { ...emptyDocument("passport"), scans: ["b", "gone", "a"] };
  assert.deepEqual(
    scansOf(document, [scan("a"), scan("b"), scan("c")]).map((item) => item.id),
    ["b", "a"],
  );
  assert.deepEqual(scansOf(emptyDocument("passport"), [scan("a")]), []);
});

test("a document holding only scans is kept", () => {
  const input: IdentityInput = {
    ...emptyIdentity,
    label: "Me",
    documents: [{ ...emptyDocument("passport"), scans: ["a"] }],
  };
  assert.deepEqual(readyToSave(input).documents, input.documents);
});

test("a chosen scan is listed by its staging token until it is saved", () => {
  const staged = stagedScan({
    chosen: true,
    token: "t1",
    name: "passport.pdf",
    mediaType: "application/pdf",
    thumbnail: "",
  });
  assert.deepEqual(staged, {
    id: "new:t1",
    name: "passport.pdf",
    mediaType: "application/pdf",
    thumbnail: "",
  });
  const document = { ...emptyDocument("passport"), scans: ["a", staged.id] };
  assert.deepEqual(
    scansOf(document, [scan("a"), staged]).map((item) => item.id),
    ["a", "new:t1"],
  );
});

test("only the scans the vault already holds count as stored", () => {
  const document = {
    ...emptyDocument("passport"),
    scans: ["a", "new:t1", "b"],
  };
  assert.deepEqual(storedScans(document), ["a", "b"]);
  assert.deepEqual(storedScans(emptyDocument("passport")), []);
});

test("only a picture or a PDF is a scan", () => {
  assert.ok(isScanMediaType("image/jpeg"));
  assert.ok(isScanMediaType("application/pdf"));
  assert.ok(!isScanMediaType("image/png"));
});

test("only the known document types are accepted", () => {
  assert.ok(isDocumentType("drivers-license"));
  assert.ok(!isDocumentType("visa"));
  assert.ok(!isDocumentType(""));
});
