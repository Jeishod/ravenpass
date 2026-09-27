import { useQueryClient } from "@tanstack/react-query";
import {
  Eye,
  EyeOff,
  KeyRound,
  ListChecks,
  type LucideIcon,
  ShieldCheck,
} from "lucide-react";
import {
  type FormEvent,
  useEffect,
  useEffectEvent,
  useId,
  useRef,
  useState,
} from "react";
import { toast } from "sonner";
import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import { queryKeys } from "../../query/keys.ts";
import type { UnlockMethods, VaultApi } from "../../vault-api.ts";
import { AccessShell, ActionBar } from "../AccessShell.tsx";
import { ForwardButton } from "../ForwardButton.tsx";
import { PhraseFields, type PhraseFieldsHandle } from "../PhraseFields.tsx";
import { PinField } from "../PinField.tsx";
import { PhraseEntry } from "../phrase-entry.ts";
import {
  ResponsiveDialog,
  ResponsiveDialogContent,
  ResponsiveDialogTitle,
} from "../ResponsiveDialog.tsx";
import { recoveryKeyWords } from "../recovery-challenge.ts";
import { ConfirmStep } from "../setup/ConfirmStep.tsx";
import { PhraseStep } from "../setup/PhraseStep.tsx";
import { SetupProgress } from "../setup/SetupProgress.tsx";
import { Button } from "../ui/button.tsx";
import { ownerCheck } from "../unlock-change.ts";

export type RecoveryKeyChangeApi = Pick<
  VaultApi,
  | "beginRecoveryPhraseChange"
  | "confirmRecoveryPhraseChange"
  | "cancelRecoveryPhraseChange"
  | "copyRecoveryKey"
  | "saveRecoveryKey"
  | "printRecoveryKey"
>;

type Step = "verify" | "phrase" | "confirm";

const noWords = PhraseEntry.empty(recoveryKeyWords);

const steps: readonly Step[] = ["verify", "phrase", "confirm"];

const stepLooks: Record<
  Step,
  {
    name: MessageKey;
    title: MessageKey;
    description: MessageKey;
    icon: LucideIcon;
  }
> = {
  verify: {
    name: "recovery-key-change.steps.verify",
    title: "recovery-key-change.verify.title",
    description: "recovery-key-change.verify.description",
    icon: ShieldCheck,
  },
  phrase: {
    name: "recovery-key-change.steps.phrase",
    title: "recovery-key-change.phrase.title",
    description: "recovery-key-change.phrase.description",
    icon: KeyRound,
  },
  confirm: {
    name: "recovery-key-change.steps.confirm",
    title: "recovery-key-change.confirm.title",
    description: "recovery-key-change.confirm.description",
    icon: ListChecks,
  },
};

/** RecoveryKeyChange walks the owner through replacing the open vault's recovery key; nothing changes until the last step. */
export function RecoveryKeyChange({
  api,
  methods,
  open,
  onClose,
}: {
  api: RecoveryKeyChangeApi;
  methods: UnlockMethods | null;
  open: boolean;
  onClose: () => void;
}) {
  const { t, failure } = useTranslator();
  const client = useQueryClient();
  const [step, setStep] = useState<Step>("verify");
  const [phrase, setPhrase] = useState("");
  const [busy, setBusy] = useState(false);
  // True while the host holds a new phrase that is neither confirmed nor discarded.
  const staged = useRef(false);
  const mounted = useRef(false);

  function discardStaged() {
    if (!staged.current) return;
    staged.current = false;
    api
      .cancelRecoveryPhraseChange()
      .catch((reason: unknown) =>
        toast.error(failure(reason, "wizard.errors.cancel-failed")),
      );
  }

  // The dialog can go without its cancel, such as when the panel under it closes.
  const discardOnUnmount = useEffectEvent(discardStaged);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      discardOnUnmount();
    };
  }, []);

  function close() {
    setPhrase("");
    setStep("verify");
    onClose();
  }

  function cancel() {
    discardStaged();
    close();
  }

  async function begin(pin: string, current: string) {
    setBusy(true);
    try {
      const next = await api.beginRecoveryPhraseChange(pin, current);
      staged.current = true;
      if (!mounted.current) {
        discardStaged();
        return;
      }
      setPhrase(next);
      setStep("phrase");
    } catch (reason) {
      toast.error(failure(reason, "recovery-key-change.errors.begin-failed"));
    } finally {
      setBusy(false);
      void client.invalidateQueries({ queryKey: queryKeys.unlockMethods });
    }
  }

  async function finish() {
    if (busy) return;
    setBusy(true);
    try {
      await api.confirmRecoveryPhraseChange(phrase);
      staged.current = false;
      toast.success(t("recovery-key-change.done"));
      close();
    } catch (reason) {
      toast.error(failure(reason, "recovery-key-change.errors.finish-failed"));
    } finally {
      setBusy(false);
      void client.invalidateQueries({ queryKey: queryKeys.unlockMethods });
    }
  }

  const look = stepLooks[step];

  return (
    <ResponsiveDialog
      open={open}
      dismissible={!busy}
      onOpenChange={(next) => {
        if (!next) cancel();
      }}
    >
      <ResponsiveDialogContent
        showCloseButton={false}
        className="bg-background sm:max-w-[480px]"
      >
        <ResponsiveDialogTitle className="sr-only">
          {t(look.title)}
        </ResponsiveDialogTitle>
        <AccessShell
          title={t(look.title)}
          description={t(look.description)}
          icon={look.icon}
          steps={steps.map((each) => t(stepLooks[each].name))}
          activeStep={steps.indexOf(step)}
          layout="dialog"
          languageMenu={false}
        >
          {step === "verify" && (
            <VerifyStep
              methods={methods}
              busy={busy}
              onCancel={cancel}
              onContinue={(pin, current) => void begin(pin, current)}
            />
          )}
          {step === "phrase" && (
            <PhraseStep
              api={api}
              phrase={phrase}
              busy={busy}
              onCancel={cancel}
              onContinue={() => setStep("confirm")}
            />
          )}
          {step === "confirm" &&
            (busy ? (
              <SetupProgress
                title={t("recovery-key-change.saving")}
                biometry={Boolean(methods?.biometryEnabled)}
              />
            ) : (
              <ConfirmStep
                phrase={phrase}
                onBack={() => setStep("phrase")}
                onConfirmed={() => void finish()}
              />
            ))}
        </AccessShell>
      </ResponsiveDialogContent>
    </ResponsiveDialog>
  );
}

/** What a new key means for this device: the ways in it keeps, or the new key where it has none. */
function thisDeviceNote(methods: UnlockMethods | null): MessageKey {
  const pin = Boolean(methods?.pinSet);
  const biometry = Boolean(methods?.biometryEnabled);
  if (pin && biometry) return "recovery-key-change.verify.this-device.both";
  if (pin) return "recovery-key-change.verify.this-device.pin";
  if (biometry) return "recovery-key-change.verify.this-device.biometry";
  return "recovery-key-change.verify.this-device.none";
}

/**
 * VerifyStep says what a new key changes and takes the PIN where the vault has one, which the new key is wrapped for
 * even where device authentication verifies the owner; a device with neither takes the current recovery key.
 */
function VerifyStep({
  methods,
  busy,
  onCancel,
  onContinue,
}: {
  methods: UnlockMethods | null;
  busy: boolean;
  onCancel: () => void;
  onContinue: (pin: string, current: string) => void;
}) {
  const { t } = useTranslator();
  const id = useId();
  const [pin, setPin] = useState("");
  const [current, setCurrent] = useState(noWords);
  const [revealCurrent, setRevealCurrent] = useState(false);
  const fields = useRef<PhraseFieldsHandle>(null);
  const pinSet = Boolean(methods?.pinSet);
  const needsCurrent = methods !== null && ownerCheck(methods) === "none";
  const ready =
    methods !== null &&
    (!pinSet || pin.length >= methods.pinMinLength) &&
    (!needsCurrent || current.complete);

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (needsCurrent && !current.complete) {
      fields.current?.focus(current.firstEmpty);
      return;
    }
    if (!ready || busy) return;
    onContinue(pinSet ? pin : "", needsCurrent ? current.phrase : "");
    setPin("");
    setCurrent(noWords);
  }

  return (
    <form id={`${id}-form`} onSubmit={submit} className="flex flex-col gap-3">
      <ul className="flex list-disc flex-col gap-1.5 pl-5 text-[13px] leading-[1.5]">
        <li>{t("recovery-key-change.verify.devices")}</li>
        <li>{t("recovery-key-change.verify.backups")}</li>
        <li>{t(thisDeviceNote(methods))}</li>
      </ul>
      {pinSet && methods && (
        <div className="flex flex-col gap-2">
          <p className="text-[13px] text-muted-foreground">
            {t(
              ownerCheck(methods) === "device"
                ? "recovery-key-change.verify.pin-then-biometry"
                : "recovery-key-change.verify.pin",
            )}
          </p>
          <PinField
            id={`${id}-pin`}
            value={pin}
            onChange={setPin}
            maxLength={methods.pinMaxLength}
            attemptsLeft={methods.pinAttemptsLeft}
            disabled={busy}
          />
        </div>
      )}
      {needsCurrent && (
        <div className="flex flex-col gap-2">
          <div className="flex items-center justify-between gap-2">
            <p className="text-[13px] text-muted-foreground">
              {t("recovery-key-change.verify.current")}
            </p>
            <Button
              className="text-muted-foreground"
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label={t(
                revealCurrent ? "recovery.phrase.hide" : "recovery.phrase.show",
              )}
              onClick={() => setRevealCurrent((shown) => !shown)}
              disabled={busy}
            >
              {revealCurrent ? <EyeOff /> : <Eye />}
            </Button>
          </div>
          <PhraseFields
            ref={fields}
            label={t("recovery.phrase.label")}
            entry={current}
            onEntry={setCurrent}
            concealed={!revealCurrent}
            disabled={busy}
          />
        </div>
      )}
      <ActionBar>
        <Button
          type="button"
          variant="ghost"
          size="pill"
          onClick={onCancel}
          disabled={busy}
        >
          {t("wizard.actions.cancel")}
        </Button>
        <ForwardButton
          type="submit"
          form={`${id}-form`}
          disabled={!ready || busy}
        >
          {t(
            busy
              ? "recovery-key-change.verify.busy"
              : "wizard.actions.continue",
          )}
        </ForwardButton>
      </ActionBar>
    </form>
  );
}
