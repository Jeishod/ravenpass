import { useState } from "react";
import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import { type StorageWarning, storageWarnings } from "../../storage/kinds.ts";
import { vaultLocation } from "../../storage/location.ts";
import type { StorageKind, StorageStatus } from "../../vault-api.ts";
import { Block, BlockNote } from "../Block.tsx";
import { ConfirmDialog } from "../ConfirmDialog.tsx";
import { StorageFileRow, StorageTypeRow } from "../StorageChoice.tsx";

const warnings: Record<StorageWarning, MessageKey> = {
  unrestricted: "settings.storage.unrestricted",
  concurrent: "storage.type.document.concurrent",
};

/** VaultStorage shows where the open vault is kept, moves it, and says what that place cannot promise. */
export function VaultStorage({
  storage,
  busy,
  moving,
  onMove,
}: {
  storage: StorageStatus | null;
  busy: boolean;
  moving: boolean;
  /** Moves the open vault to another type, or to another file of the current one. */
  onMove: (kind: StorageKind) => void;
}) {
  const { t } = useTranslator();
  const [confirming, setConfirming] = useState<StorageKind | null>(null);
  const kind = storage?.kind || null;
  const halted = busy || moving || !storage?.available;
  const notices = storage ? storageWarnings(storage) : [];

  function move(next: StorageKind) {
    if (storage?.shared) {
      setConfirming(next);
      return;
    }
    onMove(next);
  }

  return (
    <>
      <Block>
        <StorageTypeRow
          id="storage-type"
          label={t("settings.storage.type")}
          status={storage}
          disabled={halted}
          onChange={move}
        />
        <StorageFileRow
          label={t("settings.storage.file")}
          status={storage}
          actionLabel={t("settings.storage.move")}
          busyLabel={t("settings.storage.moving")}
          emptyLabel={t("settings.storage.checking")}
          onAction={
            kind && storage?.chosen.includes(kind)
              ? () => move(kind)
              : undefined
          }
          busy={halted}
        />
      </Block>
      {notices.length > 0 && (
        <BlockNote>
          {notices.map((warning) => t(warnings[warning])).join(" ")}
        </BlockNote>
      )}
      <ConfirmDialog
        open={confirming !== null}
        title={t("settings.storage.shared.title")}
        detail={
          storage
            ? t("settings.storage.shared.detail", {
                location: vaultLocation(storage),
              })
            : ""
        }
        confirm={t("settings.storage.shared.confirm")}
        cancel={t("settings.storage.shared.cancel")}
        busy={halted}
        onConfirm={() => {
          if (confirming) onMove(confirming);
          setConfirming(null);
        }}
        onCancel={() => setConfirming(null)}
      />
    </>
  );
}
