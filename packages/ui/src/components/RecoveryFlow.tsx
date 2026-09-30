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

/** What the person has accepted of the warnings a preview gives. */
interface Acceptance {
  loss: boolean;
  replacedKey: boolean;
}

const nothingAccepted: Acceptance = { loss: false, replacedKey: false };

/** Whether every warning `preview` gives is accepted; one without warnings needs nothing. */
function warningsAccepted(
  preview: RecoveryPreview,
  accepted: Acceptance,
): boolean {
  return (
    (!preview.mayLoseNewerCredentials || accepted.loss) &&
    (!preview.keyReplaced || accepted.replacedKey)
  );
}

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
  const [accepted, setAccepted] = useState(nothingAccepted);
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
    setAccepted(nothingAccepted);
    setBusy(false);
    if (warningsAccepted(preview, nothingAccepted)) proceed(preview);
  }

  // A copy that warns goes on only once the person accepts each warning.
  function proceed(preview: RecoveryPreview) {
    if (!warningsAccepted(preview, accepted)) return;
    if (preview.needsWayIn) {
      setStage("unlock");
      return;
    }
    void restore(preview, noUnlockChoice);
  }

  async function restore(preview: RecoveryPreview, chosen: UnlockChoice) {
    setBusy(true);
    setRestoring(chosen);
    try {
      await api.confirmRecovery(warningsAccepted(preview, accepted), chosen);
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
          onChosen={(chosen) => {
            if (staged) void restore(staged, chosen);
          }}
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

          {staged?.keyReplaced && (
            <RecoveryWarning
              id="accept-replaced-key"
              title={t("recovery.preview.replaced-key.title")}
              description={t("recovery.preview.replaced-key.description")}
              accept={t("recovery.preview.accept-replaced-key")}
              checked={accepted.replacedKey}
              onChecked={(replacedKey) =>
                setAccepted((held) => ({ ...held, replacedKey }))
              }
              disabled={busy}
            />
          )}
          {staged?.mayLoseNewerCredentials && (
            <RecoveryWarning
              id="accept-older-copy"
              title={t("recovery.preview.older.title")}
              description={t("recovery.preview.older.description")}
              accept={t("recovery.preview.accept-loss")}
              checked={accepted.loss}
              onChecked={(loss) => setAccepted((held) => ({ ...held, loss }))}
              disabled={busy}
            />
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
                busy || (staged !== null && !warningsAccepted(staged, accepted))
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

/** RecoveryWarning states one risk of the staged copy and takes the person's acceptance of it. */
function RecoveryWarning({
  id,
  title,
  description,
  accept,
  checked,
  onChecked,
  disabled,
}: {
  id: string;
  title: string;
  description: string;
  accept: string;
  checked: boolean;
  onChecked: (checked: boolean) => void;
  disabled: boolean;
}) {
  return (
    <>
      <Alert>
        <CircleAlert />
        <AlertTitle>{title}</AlertTitle>
        <AlertDescription>{description}</AlertDescription>
      </Alert>
      <div className="flex items-start gap-2.5">
        <Checkbox
          id={id}
          checked={checked}
          onCheckedChange={(next) => onChecked(next === true)}
          disabled={disabled}
        />
        <label htmlFor={id} className="text-[13px]">
          {accept}
        </label>
      </div>
    </>
  );
}
