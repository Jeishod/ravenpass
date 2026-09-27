import assert from "node:assert/strict";
import test from "node:test";
import type { ImportPreview, ImportSkip } from "../vault-api.ts";
import { defaultImportOptions, ImportReview } from "./review.ts";

function preview(changes: Partial<ImportPreview> = {}): ImportPreview {
  return {
    format: "json",
    items: 16,
    kinds: [
      {
        kind: "credential",
        count: 10,
        duplicates: 3,
        oneTimeCodes: 6,
        converted: [],
      },
      {
        kind: "note",
        count: 4,
        duplicates: 1,
        oneTimeCodes: 0,
        converted: [{ from: "ssh-key", count: 2 }],
      },
    ],
    groups: { new: 5, existing: 2, dropped: 1 },
    groupsWithoutDuplicates: { new: 4, existing: 1, dropped: 0 },
    cardIssuers: [],
    skipped: [],
    attachments: 0,
    passkeys: 0,
    importedPasskeys: 5,
    ...changes,
  };
}

const keepEverything = { groups: true, skipDuplicates: false };

test("each card issuer is named the network a typed number would be", () => {
  const review = new ImportReview(
    preview({ cardIssuers: ["41111111", "55555555", "99999999"] }),
    defaultImportOptions,
  );
  assert.deepEqual(review.cardNetworks, {
    "41111111": "visa",
    "55555555": "mastercard",
  });
  assert.deepEqual(
    review.withOptions(keepEverything).cardNetworks,
    review.cardNetworks,
  );
});

test("skipping duplicates leaves them out of each kind and the total", () => {
  const review = new ImportReview(preview(), defaultImportOptions);
  assert.deepEqual(
    review.kinds.map((kind) => [kind.kind, kind.count, kind.adding]),
    [
      ["credential", 10, 7],
      ["note", 4, 3],
    ],
  );
  assert.equal(review.total, 10);
});

test("keeping duplicates adds them as new items", () => {
  const review = new ImportReview(preview(), keepEverything);
  assert.deepEqual(
    review.kinds.map((kind) => kind.adding),
    [10, 4],
  );
  assert.equal(review.total, 14);
});

test("duplicates are counted whichever the option", () => {
  assert.equal(new ImportReview(preview(), defaultImportOptions).duplicates, 4);
  assert.equal(new ImportReview(preview(), keepEverything).duplicates, 4);
});

test("one-time codes count the file whichever the duplicate option", () => {
  for (const options of [defaultImportOptions, keepEverything]) {
    assert.deepEqual(
      new ImportReview(preview(), options).kinds.map(
        (kind) => kind.oneTimeCodes,
      ),
      [6, 0],
    );
  }
});

test("imported passkeys count the file whichever the duplicate option and belong to credentials", () => {
  for (const options of [defaultImportOptions, keepEverything]) {
    assert.deepEqual(
      new ImportReview(preview(), options).kinds.map((kind) => kind.passkeys),
      [5, 0],
    );
  }
});

test("imported passkeys are not left behind", () => {
  const review = new ImportReview(
    preview({ importedPasskeys: 4 }),
    defaultImportOptions,
  );
  assert.ok(!review.leftBehind);
  assert.equal(review.kinds[0]?.passkeys, 4);
});

test("a kind keeps what it holds and where it came from", () => {
  const note = new ImportReview(preview(), defaultImportOptions).kinds[1];
  assert.equal(note?.duplicates, 1);
  assert.deepEqual(note?.converted, [{ from: "ssh-key", count: 2 }]);
});

test("new groups follow the duplicate option, and none are made with groups off", () => {
  assert.equal(new ImportReview(preview(), defaultImportOptions).newGroups, 4);
  assert.equal(new ImportReview(preview(), keepEverything).newGroups, 5);
  assert.equal(
    new ImportReview(preview(), { groups: false, skipDuplicates: true })
      .newGroups,
    0,
  );
});

test("folders count every name, and dropped names follow the options", () => {
  const review = new ImportReview(preview(), keepEverything);
  assert.equal(review.folders, 8);
  assert.equal(review.droppedFolders, 1);
  assert.equal(review.withOptions(defaultImportOptions).droppedFolders, 0);
  assert.equal(
    review.withOptions({ groups: false, skipDuplicates: false }).droppedFolders,
    0,
  );
});

test("changing the options makes a new review and leaves the first one", () => {
  const first = new ImportReview(preview(), defaultImportOptions);
  const second = first.withOptions(keepEverything);
  assert.notEqual(second, first);
  assert.deepEqual(second.options, keepEverything);
  assert.equal(second.total, 14);
  assert.deepEqual(first.options, defaultImportOptions);
  assert.equal(first.total, 10);
});

test("an import of duplicates only adds nothing until they are kept", () => {
  const onlyDuplicates = preview({
    kinds: [
      {
        kind: "card",
        count: 2,
        duplicates: 2,
        oneTimeCodes: 0,
        converted: [],
      },
    ],
  });
  const review = new ImportReview(onlyDuplicates, defaultImportOptions);
  assert.equal(review.total, 0);
  assert.ok(review.empty);
  assert.ok(!review.withOptions(keepEverything).empty);
});

test("a file with no kinds adds nothing", () => {
  const review = new ImportReview(preview({ kinds: [] }), keepEverything);
  assert.equal(review.total, 0);
  assert.equal(review.duplicates, 0);
  assert.ok(review.empty);
});

test("skipped items, attached files and passkeys are left behind", () => {
  const unnamed: ImportSkip = { label: "", origin: "login", reason: "unnamed" };
  assert.ok(!new ImportReview(preview(), defaultImportOptions).leftBehind);
  for (const changes of [
    { skipped: [unnamed] },
    { attachments: 3 },
    { passkeys: 1 },
  ]) {
    assert.ok(
      new ImportReview(preview(changes), defaultImportOptions).leftBehind,
    );
  }
});
