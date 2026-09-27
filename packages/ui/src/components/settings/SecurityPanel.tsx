import { useState } from "react";
import { useCapabilities } from "../../host/capabilities.tsx";
import { useTranslator } from "../../i18n/translator.tsx";
import type { AutoLock, UnlockMethods } from "../../vault-api.ts";
import { Block, BlockHeading, BlockRow } from "../Block.tsx";
import { ConfirmDialog } from "../ConfirmDialog.tsx";
import { UnlockOptions } from "../UnlockOptions.tsx";
import { Button } from "../ui/button.tsx";
import type { UnlockPending } from "../unlock-change.ts";
import { DelaySetting } from "./DelaySetting.tsx";
import {
  RecoveryKeyChange,
  type RecoveryKeyChangeApi,
} from "./RecoveryKeyChange.tsx";
export function SecurityPanel({
  methods,
  busy,
  pending,
  onSetPin,
  onRemovePin,
  onBiometry,
  autoLock,
  onAutoLock,
  recoveryKey,
}: {
  methods: UnlockMethods | null;
  busy: boolean;
  /** The unlock change the device is applying; only the controls over the ways in wait for it. */
  pending: UnlockPending | null;
  onSetPin: (pin: string, current: string) => Promise<boolean>;
  onRemovePin: (current: string) => Promise<boolean>;
  onBiometry: (enabled: boolean, current: string) => Promise<boolean>;
  autoLock: AutoLock | null;
  onAutoLock: (enabled: boolean, seconds: number) => void;
  recoveryKey: RecoveryKeyChangeApi;
}) {
  const { t } = useTranslator();
  const offers = useCapabilities();
  const [confirmingOff, setConfirmingOff] = useState(false);
  const [changingKey, setChangingKey] = useState(false);

  return (
    <>
      <BlockHeading>{t("settings.unlock.heading")}</BlockHeading>
      <UnlockOptions
        methods={methods}
        busy={busy}
        pending={pending}
        descriptions={false}
        confirmChanges
        onSetPin={onSetPin}
        onRemovePin={onRemovePin}
        onBiometry={onBiometry}
      />
      <BlockHeading>{t("settings.recovery-key.heading")}</BlockHeading>
      <Block>
        <BlockRow
          title={t("settings.recovery-key.title")}
          detail={t("settings.recovery-key.detail")}
          wrap
        >
          <Button
            type="button"
            variant="quiet"
            size="pill-sm"
            disabled={busy || pending !== null || !methods}
            onClick={() => setChangingKey(true)}
          >
            {t("settings.recovery-key.change")}
          </Button>
        </BlockRow>
      </Block>
      <RecoveryKeyChange
        api={recoveryKey}
        methods={methods}
        open={changingKey}
        onClose={() => setChangingKey(false)}
      />
      <BlockHeading>{t("settings.locking.heading")}</BlockHeading>
      <DelaySetting
        className=""
        id="auto-lock"
        setting={autoLock}
        busy={busy}
        onChange={(enabled, seconds) => {
          if (autoLock?.enabled && !enabled) setConfirmingOff(true);
          else onAutoLock(enabled, seconds);
        }}
        title={
          offers.lockWhenHidden
            ? "settings.auto-lock.hidden"
            : "settings.auto-lock"
        }
        delay="settings.auto-lock.delay"
        note={
          offers.lockWhenHidden
            ? "settings.auto-lock.note.hidden"
            : "settings.auto-lock.note"
        }
      />
      <ConfirmDialog
        open={confirmingOff}
        title={t("settings.auto-lock.off.title")}
        detail={t("settings.auto-lock.off.detail")}
        confirm={t("settings.auto-lock.off.confirm")}
        cancel={t("settings.auto-lock.off.cancel")}
        destructive
        busy={busy}
        onConfirm={() => {
          setConfirmingOff(false);
          if (autoLock) onAutoLock(false, autoLock.seconds);
        }}
        onCancel={() => setConfirmingOff(false)}
      />
    </>
  );
}
