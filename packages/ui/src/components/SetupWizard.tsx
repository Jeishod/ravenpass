import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Fingerprint,
  KeyRound,
  ListChecks,
  type LucideIcon,
  Vault,
} from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { useCapabilities } from "../host/capabilities.tsx";
import type { MessageKey } from "../i18n/messages.ts";
import { useTranslator } from "../i18n/translator.tsx";
import { queryKeys } from "../query/keys.ts";
import { vaultStorageQuery } from "../query/vault-storage.ts";
import type {
  StorageKind,
  UnlockChoice,
  VaultApi,
  VaultState,
} from "../vault-api.ts";
import { AccessShell } from "./AccessShell.tsx";
import { ConfirmStep } from "./setup/ConfirmStep.tsx";
import { PhraseStep } from "./setup/PhraseStep.tsx";
import { SetupProgress } from "./setup/SetupProgress.tsx";
import { StorageStep } from "./setup/StorageStep.tsx";
import { UnlockChoiceStep } from "./UnlockChoiceStep.tsx";
import { bindsBiometry, noUnlockChoice } from "./unlock-change.ts";

type Step = "storage" | "phrase" | "confirm" | "unlock";

const steps: readonly Step[] = ["storage", "phrase", "confirm", "unlock"];

const stepLooks: Record<
  Step,
  {
    name: MessageKey;
    title: MessageKey;
    description: MessageKey;
    icon: LucideIcon;
  }
> = {
  storage: {
    name: "wizard.steps.storage",
    title: "wizard.storage.title",
    description: "wizard.storage.description",
    icon: Vault,
  },
  phrase: {
    name: "wizard.steps.phrase",
    title: "wizard.phrase.title",
    description: "wizard.phrase.description",
    icon: KeyRound,
  },
  confirm: {
    name: "wizard.steps.confirm",
    title: "wizard.confirm.title",
    description: "wizard.confirm.description",
    icon: ListChecks,
  },
  unlock: {
    name: "wizard.steps.unlock",
    title: "wizard.unlock.title",
    description: "wizard.unlock.description",
    icon: Fingerprint,
  },
};

/** SetupWizard creates a vault; nothing is written until the last step commits the unlock choice. */
export function SetupWizard({
  api,
  onPhase,
  onBack,
}: {
  api: VaultApi;
  /** Called with the phase the application moves to once the walk ends or is cancelled. */
  onPhase: (phase: VaultState["phase"]) => void;
  /** Present when the walk started somewhere it can return to before anything is staged. */
  onBack?: () => void;
}) {
  const { t, failure, language } = useTranslator();
  const { storageLocations } = useCapabilities();
  const client = useQueryClient();
  const [step, setStep] = useState<Step>("storage");
  const [phrase, setPhrase] = useState("");
  const [choice, setChoice] = useState<UnlockChoice>(noUnlockChoice);
  const [busy, setBusy] = useState(false);
  const storageKey = queryKeys.storage(language);
  const storage =
    useQuery(
      vaultStorageQuery(api, language, "wizard.errors.storage-unreadable"),
    ).data ?? null;
  // Cancelling leads to another known vault; a failed read hides the cancel.
  const otherVault =
    useQuery({
      queryKey: queryKeys.knowsVault,
      queryFn: () => api.knowsVault(),
    }).data ?? false;
  const location = useMutation({
    mutationFn: (kind: StorageKind) => api.selectStorageLocation(kind),
    onSuccess: (change) =>
      change.changed
        ? client.invalidateQueries({ queryKey: storageKey })
        : undefined,
    meta: { failure: "wizard.errors.location-rejected" },
  });

  async function cancelWalk() {
    setBusy(true);
    try {
      await api.cancelVaultCreation();
      onPhase((await api.getState()).phase);
    } catch (reason) {
      toast.error(failure(reason, "wizard.errors.leave-failed"));
      setBusy(false);
    }
  }

  async function begin() {
    setBusy(true);
    try {
      setPhrase(await api.beginCreation());
      setStep("phrase");
    } catch (reason) {
      toast.error(failure(reason, "wizard.errors.begin-failed"));
    } finally {
      setBusy(false);
    }
  }

  async function cancelCreation() {
    setBusy(true);
    try {
      await api.lock();
      setPhrase("");
      setChoice(noUnlockChoice);
      setStep("storage");
    } catch (reason) {
      toast.error(failure(reason, "wizard.errors.cancel-failed"));
    } finally {
      setBusy(false);
    }
  }

  async function create(choice: UnlockChoice) {
    setBusy(true);
    try {
      await api.confirmCreation(phrase, choice);
      setPhrase("");
      onPhase("ready");
    } catch (reason) {
      toast.error(failure(reason, "wizard.errors.finish-failed"));
      setBusy(false);
    }
  }

  const look = stepLooks[step];
  // Where the host keeps the vault file in its own storage, the first step only shows it.
  const title =
    step === "storage" && !storageLocations
      ? "wizard.storage.title-fixed"
      : look.title;
  const leave = onBack
    ? { label: t("wizard.actions.back"), run: onBack }
    : otherVault
      ? { label: t("wizard.actions.cancel"), run: () => void cancelWalk() }
      : undefined;

  return (
    <AccessShell
      title={t(title)}
      description={t(look.description)}
      icon={look.icon}
      steps={steps.map((each) => t(stepLooks[each].name))}
      activeStep={steps.indexOf(step)}
    >
      {step === "storage" && (
        <StorageStep
          storage={storage}
          choosing={location.isPending}
          busy={busy}
          onChoose={location.mutate}
          leave={leave}
          onContinue={begin}
        />
      )}

      {step === "phrase" && (
        <PhraseStep
          api={api}
          phrase={phrase}
          busy={busy}
          onCancel={cancelCreation}
          onContinue={() => setStep("confirm")}
        />
      )}

      {step === "confirm" && (
        <ConfirmStep
          phrase={phrase}
          onBack={() => setStep("phrase")}
          onConfirmed={() => setStep("unlock")}
        />
      )}

      {step === "unlock" &&
        (busy ? (
          <SetupProgress
            title={t("wizard.unlock.creating")}
            biometry={bindsBiometry(choice, null)}
          />
        ) : (
          <UnlockChoiceStep
            api={api}
            choice={choice}
            onChoice={setChoice}
            backLabel={t("wizard.actions.back")}
            submitLabel={t("wizard.unlock.create")}
            onBack={() => setStep("confirm")}
            onChosen={create}
          />
        ))}
    </AccessShell>
  );
}
