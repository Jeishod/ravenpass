import assert from "node:assert/strict";
import test from "node:test";
import { backupNote } from "./backup-note.ts";

const lastBackupAt = Date.UTC(2026, 8, 25, 12, 30);

test("a failed attempt is reported before the last backup", () => {
  assert.deepEqual(backupNote({ enabled: true, failed: true, lastBackupAt }), {
    key: "settings.backup.note.failed",
  });
});

test("a failed attempt is not reported while backups are off", () => {
  assert.deepEqual(backupNote({ enabled: false, failed: true, lastBackupAt }), {
    key: "settings.backup.note.last",
    at: lastBackupAt,
  });
});

test("the last backup is reported with the moment it was saved", () => {
  assert.deepEqual(backupNote({ enabled: true, failed: false, lastBackupAt }), {
    key: "settings.backup.note.last",
    at: lastBackupAt,
  });
});

test("without a backup the note says when backups are saved", () => {
  assert.deepEqual(
    backupNote({ enabled: true, failed: false, lastBackupAt: 0 }),
    { key: "settings.backup.note.open" },
  );
  assert.deepEqual(
    backupNote({ enabled: false, failed: true, lastBackupAt: 0 }),
    { key: "settings.backup.note.open" },
  );
});
