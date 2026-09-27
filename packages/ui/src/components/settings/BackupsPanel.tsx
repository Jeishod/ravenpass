import { cn } from "cn";
import { useCapabilities } from "../../host/capabilities.tsx";
import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import { useHostSetting } from "../../query/host-setting.ts";
import { queryKeys } from "../../query/keys.ts";
import { vaultLocation } from "../../storage/location.ts";
import type {
  AutoBackup,
  BackupInterval,
  BackupSettings,
  ExportState,
} from "../../vault-api.ts";
import { Block, BlockNote, BlockRow, blockControl } from "../Block.tsx";
import { Button } from "../ui/button.tsx";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../ui/select.tsx";
import { Switch } from "../ui/switch.tsx";
import { backupNote } from "./backup-note.ts";
import { EncryptedCopy } from "./EncryptedCopy.tsx";

const intervalNames: Record<BackupInterval, MessageKey> = {
  daily: "settings.backup.frequency.daily",
  weekly: "settings.backup.frequency.weekly",
  monthly: "settings.backup.frequency.monthly",
};

/** BackupsPanel offers an encrypted copy saved by hand and, where the host offers them, automatic backups. */
export function BackupsPanel({
  settings,
  exportState,
  busy,
  onExport,
  onBackedUp,
}: {
  settings: BackupSettings;
  exportState: ExportState;
  busy: boolean;
  onExport: () => void;
  /** Called after a change to automatic backups, which may have saved the latest encrypted copy. */
  onBackedUp: () => void;
}) {
  const capabilities = useCapabilities();

  return (
    <>
      <EncryptedCopy state={exportState} busy={busy} onExport={onExport} />
      {capabilities.autoBackups && (
        <AutoBackups settings={settings} busy={busy} onBackedUp={onBackedUp} />
      )}
    </>
  );
}

function AutoBackups({
  settings,
  busy,
  onBackedUp,
}: {
  settings: BackupSettings;
  busy: boolean;
  onBackedUp: () => void;
}) {
  const { t, language } = useTranslator();
  const setting = useHostSetting({
    key: queryKeys.autoBackup,
    read: () => settings.autoBackup(),
    write: (action: () => Promise<unknown>) => action(),
    readFailure: "app.error.setting-read",
    writeFailure: "settings.backup.error",
  });
  const backup = setting.value;

  async function act(action: () => Promise<unknown>) {
    await setting.change(action);
    onBackedUp();
  }

  function turn(current: AutoBackup, enabled: boolean) {
    void act(async () => {
      if (
        enabled &&
        !current.folder &&
        !(await settings.chooseBackupFolder())
      ) {
        return;
      }
      await settings.setAutoBackup(enabled, current.interval, current.keep);
    });
  }

  const unknown = busy || setting.changing || !backup;
  const note = backup && backupNote(backup);

  return (
    <>
      <Block className="mt-5">
        <BlockRow title={t("settings.backup.auto")} htmlFor="backup-auto">
          <Switch
            id="backup-auto"
            checked={Boolean(backup?.enabled)}
            disabled={unknown}
            onCheckedChange={(enabled) => {
              if (backup) turn(backup, enabled);
            }}
          />
        </BlockRow>
        <BlockRow
          title={t("settings.backup.frequency")}
          htmlFor="backup-frequency"
        >
          <Select
            value={backup?.interval}
            disabled={unknown || !backup?.enabled}
            onValueChange={(next) => {
              const interval = backup?.intervals.find((item) => item === next);
              if (backup && interval) {
                void act(() =>
                  settings.setAutoBackup(backup.enabled, interval, backup.keep),
                );
              }
            }}
          >
            <SelectTrigger
              id="backup-frequency"
              size="sm"
              className={cn(blockControl, "max-w-[170px]")}
              aria-label={t("settings.backup.frequency")}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {backup?.intervals.map((interval) => (
                  <SelectItem key={interval} value={interval}>
                    {t(intervalNames[interval])}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </BlockRow>
        <BlockRow title={t("settings.backup.keep")} htmlFor="backup-keep">
          <Select
            value={backup ? String(backup.keep) : undefined}
            disabled={unknown || !backup?.enabled}
            onValueChange={(next) => {
              if (backup) {
                void act(() =>
                  settings.setAutoBackup(
                    backup.enabled,
                    backup.interval,
                    Number(next),
                  ),
                );
              }
            }}
          >
            <SelectTrigger
              id="backup-keep"
              size="sm"
              className={cn(blockControl, "max-w-[170px]")}
              aria-label={t("settings.backup.keep")}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {backup?.keeps.map((keep) => (
                  <SelectItem key={keep} value={String(keep)}>
                    {keep === 1
                      ? t("settings.backup.keep.latest")
                      : t("settings.backup.keep.count", { count: keep })}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </BlockRow>
        <BlockRow
          title={t("settings.backup.folder")}
          detail={
            backup &&
            (backup.folder
              ? vaultLocation(backup.folder)
              : t("settings.backup.folder.empty"))
          }
        >
          <Button
            type="button"
            variant="quiet"
            size="pill-sm"
            className="bg-tile font-normal hover:bg-field-hover"
            disabled={unknown}
            onClick={() => {
              void act(() => settings.chooseBackupFolder());
            }}
          >
            {t(
              backup?.folder
                ? "settings.backup.folder.change"
                : "settings.backup.folder.choose",
            )}
          </Button>
        </BlockRow>
      </Block>
      {note && (
        <BlockNote>
          {"at" in note
            ? t(note.key, {
                date: new Intl.DateTimeFormat(language, {
                  dateStyle: "medium",
                  timeStyle: "short",
                }).format(note.at),
              })
            : t(note.key)}
        </BlockNote>
      )}
    </>
  );
}
