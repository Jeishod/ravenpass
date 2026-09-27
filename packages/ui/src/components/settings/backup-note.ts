import type { MessageKey } from "../../i18n/messages.ts";
import type { AutoBackup } from "../../vault-api.ts";

/** `at` is when the last backup was saved, in Unix milliseconds. */
export type BackupNote = { key: MessageKey } | { key: MessageKey; at: number };

/** A failed last attempt while backups are on, else the open vault's last backup, else that backups run while it is open. */
export function backupNote(
  backup: Pick<AutoBackup, "enabled" | "failed" | "lastBackupAt">,
): BackupNote {
  if (backup.enabled && backup.failed) {
    return { key: "settings.backup.note.failed" };
  }
  if (backup.lastBackupAt > 0) {
    return { key: "settings.backup.note.last", at: backup.lastBackupAt };
  }
  return { key: "settings.backup.note.open" };
}
