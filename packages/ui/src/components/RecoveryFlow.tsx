import { useQuery } from "@tanstack/react-query";
import { CircleAlert, Eye, EyeOff, Fingerprint, KeyRound } from "lucide-react";
import { type FormEvent, useId, useRef, useState } from "react";
import { toast } from "sonner";
import { useTranslator } from "../i18n/translator.tsx";
import { unlockMethodsQuery } from "../query/unlock-methods.ts";
import type { RecoveryPreview, UnlockChoice, VaultApi } from "../vault-api.ts";
import { AccessShell, ActionBar } from "./AccessShell.tsx";
import { ForwardButton } from "./ForwardButton.tsx";
import { PhraseFields, type PhraseFieldsHandle } from "./PhraseFields.tsx";
import { PhraseEntry } from "./phrase-entry.ts";
import { recoveryKeyWords } from "./recovery-challenge.ts";
import { SetupProgress } from "./setup/SetupProgress.tsx";
import { UnlockChoiceStep } from "./UnlockChoiceStep.tsx";
import { Alert, AlertDescription, AlertTitle } from "./ui/alert.tsx";
import { Button } from "./ui/button.tsx";
import { Checkbox } from "./ui/checkbox.tsx";
import { bindsBiometry, noUnlockChoice } from "./unlock-change.ts";

const noWords = PhraseEntry.empty(recoveryKeyWords);

type Stage = "phrase" | "unlock";

/** RecoveryFlow opens the current vault with its recovery key, then sets up ways in where the device holds none. */
export function RecoveryFlow({
  api,
  onRecovered,
  onCancel,
}: {
  api: VaultApi;
  onRecovered: () => void;
  onCancel: () => void;
}) {
  const { t, failure } = useTranslator();
  const formID = useId();
  const [stage, setStage] = useState<Stage>("phrase");
  const [entry, setEntry] = useState(noWords);
  const [revealPhrase, setRevealPhrase] = useState(false);
  const fields = useRef<PhraseFieldsHandle>(null);
  // The host refuses a second recovery while one is staged; going back discards the staging.
  const [staged, setStaged] = useState<RecoveryPreview | null>(null);
  const [acceptLoss, setAcceptLoss] = useState(false);
  const [choice, setChoice] = useState<UnlockChoice>(noUnlockChoice);
  const [restoring, setRestoring] = useState<UnlockChoice | null>(null);
  const [busy, setBusy] = useState(false);
  const kept = useQuery(unlockMethodsQuery(api)).data ?? null;

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (staged) {
      proceed(staged);
      return;
    }
    if (!entry.complete) {
      toast.error(t("recovery.errors.phrase-missing"));
      fields.current?.focus(entry.firstEmpty);
      return;
    }
    setBusy(true);
    let preview: RecoveryPreview;
    try {
      preview = await api.beginRecovery(entry.phrase);
    } catch (reason) {
      toast.error(failure(reason, "recovery.errors.local-failed"));
      setBusy(false);
      return;
    }
    setStaged(preview);
    setAcceptLoss(false);
    setBusy(false);
    if (!preview.mayLoseNewerCredentials) proceed(preview);
  }

  // An older copy goes on only once the person accepts losing newer items.
  function proceed(preview: RecoveryPreview) {
    if (preview.mayLoseNewerCredentials && !acceptLoss) return;
    if (preview.needsWayIn) {
      setStage("unlock");
      return;
    }
    void restore(noUnlockChoice);
  }

  async function restore(chosen: UnlockChoice) {
    setBusy(true);
    setRestoring(chosen);
    try {
      await api.confirmRecovery(acceptLoss, chosen);
      setEntry(noWords);
      onRecovered();
    } catch (reason) {
      toast.error(failure(reason, "recovery.errors.confirm-failed"));
      setStaged(null);
      setChoice(noUnlockChoice);
      setStage("phrase");
      setRestoring(null);
      setBusy(false);
    }
  }

  async function cancel() {
    setBusy(true);
    try {
      await api.lock();
      setEntry(noWords);
      onCancel();
    } catch (reason) {
      toast.error(failure(reason, "recovery.errors.cancel-failed"));
      setBusy(false);
    }
  }

  const unlocking = stage === "unlock";

  return (
    <AccessShell
      title={
        unlocking ? t("recovery.unlock.title") : t("unlock.recovery.title")
      }
      description={
        unlocking
          ? t("recovery.unlock.description")
          : t("unlock.recovery.description")
      }
      icon={unlocking ? Fingerprint : KeyRound}
      activeStep={unlocking ? 1 : 0}
    >
      {restoring ? (
        <SetupProgress
          title={t("recovery.restoring")}
          biometry={bindsBiometry(restoring, kept)}
        />
      ) : unlocking ? (
        <UnlockChoiceStep
          api={api}
          choice={choice}
          onChoice={setChoice}
          backLabel={t("recovery.actions.back")}
          submitLabel={t("recovery.actions.restore")}
          onBack={() => setStage("phrase")}
          onChosen={restore}
        />
      ) : (
        <form id={formID} className="flex flex-col gap-3" onSubmit={submit}>
          <div className="flex flex-col gap-1.5">
            <div className="flex items-center justify-between gap-2">
              <span className="text-xs text-muted-foreground">
                {t("recovery.phrase.label")}
              </span>
              <Button
                className="text-muted-foreground"
                variant="ghost"
                size="icon-sm"
                type="button"
                onClick={() => setRevealPhrase((visible) => !visible)}
                aria-label={
                  revealPhrase
                    ? t("recovery.phrase.hide")
                    : t("recovery.phrase.show")
                }
                disabled={busy}
              >
                {revealPhrase ? <EyeOff /> : <Eye />}
              </Button>
            </div>
            <PhraseFields
              ref={fields}
              label={t("recovery.phrase.label")}
              entry={entry}
              onEntry={setEntry}
              concealed={!revealPhrase}
              disabled={busy || staged !== null}
            />
          </div>

          {staged?.mayLoseNewerCredentials && (
            <>
              <Alert>
                <CircleAlert />
                <AlertTitle>{t("recovery.preview.older.title")}</AlertTitle>
                <AlertDescription>
                  {t("recovery.preview.older.description")}
                </AlertDescription>
              </Alert>
              <div className="flex items-start gap-2.5">
                <Checkbox
                  id="accept-older-copy"
                  checked={acceptLoss}
                  onCheckedChange={(checked) => setAcceptLoss(checked === true)}
                  disabled={busy}
                />
                <label htmlFor="accept-older-copy" className="text-[13px]">
                  {t("recovery.preview.accept-loss")}
                </label>
              </div>
            </>
          )}

          <ActionBar>
            <Button
              type="button"
              variant="ghost"
              size="pill"
              onClick={cancel}
              disabled={busy}
            >
              {t("recovery.actions.back")}
            </Button>
            <ForwardButton
              type="submit"
              form={formID}
              disabled={
                busy || Boolean(staged?.mayLoseNewerCredentials && !acceptLoss)
              }
            >
              {busy
                ? t("recovery.actions.checking")
                : staged && !staged.needsWayIn
                  ? t("recovery.actions.restore")
                  : t("recovery.actions.continue")}
            </ForwardButton>
          </ActionBar>
        </form>
      )}
    </AccessShell>
  );
}
