import { Fingerprint } from "lucide-react";
import { useEffect, useState, useSyncExternalStore } from "react";
import type {
  AutofillApi,
  UnlockOpening,
} from "../../autofill/autofill-api.ts";
import { type UnlockNote, UnlockScreen } from "../../autofill/unlock-screen.ts";
import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import { PinField } from "../PinField.tsx";
import { ResponsiveDialogFooter } from "../ResponsiveDialog.tsx";
import { Button } from "../ui/button.tsx";
import { AutofillNote, AutofillSheet } from "./AutofillSheet.tsx";

const notes: Record<UnlockNote, MessageKey> = {
  "wrong-pin": "failure.pin-wrong",
  "pin-removed": "failure.pin-removed",
  "too-soon": "failure.pin-too-soon",
  failed: "unlock.errors.failed",
};

/** UnlockSheet opens the vault with its PIN, and with the device's own unlock where the vault takes it. */
export function UnlockSheet({
  api,
  opening,
}: {
  api: AutofillApi;
  opening: UnlockOpening;
}) {
  const { t } = useTranslator();
  const [screen] = useState(
    () => new UnlockScreen(opening, (pin) => api.unlock(pin)),
  );
  const view = useSyncExternalStore(screen.state.subscribe, screen.state.get);
  const { methods, busy } = view;

  useEffect(() => {
    if (!opening.methods.pinSet) return;
    const frame = requestAnimationFrame(() => api.showKeyboard());
    return () => cancelAnimationFrame(frame);
  }, [api, opening.methods.pinSet]);

  return (
    <AutofillSheet
      title={t(
        opening.verify
          ? "autofill.wait.verify.title"
          : "confirmation.unlock.title",
      )}
      description={t(
        opening.verify
          ? "autofill.verify.description"
          : "autofill.unlock.description",
      )}
      busy={busy}
      onClose={() => api.cancel()}
    >
      <form
        id="autofill-unlock"
        className="grid gap-2"
        onSubmit={(event) => {
          event.preventDefault();
          void screen.submit();
        }}
      >
        {methods.pinSet && (
          <PinField
            id="autofill-pin"
            value={view.pin}
            onChange={(pin) => screen.enter(pin)}
            maxLength={methods.pinMaxLength}
            attemptsLeft={methods.pinAttemptsLeft}
            disabled={busy}
          />
        )}
        {view.note && <AutofillNote>{t(notes[view.note])}</AutofillNote>}
      </form>
      <ResponsiveDialogFooter>
        <Button
          type="button"
          variant="quiet"
          size="pill"
          disabled={busy}
          onClick={() => api.cancel()}
        >
          {t("autofill.cancel")}
        </Button>
        {screen.biometry && (
          <Button
            type="button"
            variant={methods.pinSet ? "quiet" : "raised"}
            size="pill"
            disabled={busy}
            onClick={() => void screen.unlockWithDevice()}
          >
            <Fingerprint data-icon="inline-start" />
            {t("unlock.biometry-action")}
          </Button>
        )}
        {methods.pinSet && (
          <Button
            type="submit"
            form="autofill-unlock"
            variant="raised"
            size="pill"
            disabled={busy || !screen.pinReady}
          >
            {busy
              ? t("unlock.pin.busy")
              : t(
                  opening.verify
                    ? "confirmation.change-unlock.confirm"
                    : "unlock.pin.action",
                )}
          </Button>
        )}
      </ResponsiveDialogFooter>
    </AutofillSheet>
  );
}
