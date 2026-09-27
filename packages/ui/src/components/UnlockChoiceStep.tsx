import { useQuery } from "@tanstack/react-query";
import { useTranslator } from "../i18n/translator.tsx";
import { unlockMethodsQuery } from "../query/unlock-methods.ts";
import type { UnlockChoice, VaultApi } from "../vault-api.ts";
import { ActionBar } from "./AccessShell.tsx";
import { ForwardButton } from "./ForwardButton.tsx";
import { UnlockOptions } from "./UnlockOptions.tsx";
import { Button } from "./ui/button.tsx";

/**
 * UnlockChoiceStep collects how a new or recovered vault will open; nothing reaches the device until the caller commits.
 * The caller holds the choice, so it survives the step giving way to the progress of a commit that fails.
 */
export function UnlockChoiceStep({
  api,
  choice,
  onChoice,
  backLabel,
  submitLabel,
  onBack,
  onChosen,
}: {
  api: VaultApi;
  choice: UnlockChoice;
  onChoice: (choice: UnlockChoice) => void;
  backLabel: string;
  submitLabel: string;
  onBack: () => void;
  onChosen: (choice: UnlockChoice) => void;
}) {
  const { t } = useTranslator();
  const { data: offered } = useQuery(unlockMethodsQuery(api));

  const chosen = choice.biometry || choice.pin !== "";
  const methods = offered
    ? {
        ...offered,
        biometryEnabled: choice.biometry,
        pinSet: choice.pin !== "",
        pinAttemptsLeft: 0,
      }
    : null;

  return (
    <div className="flex flex-col gap-3">
      <UnlockOptions
        methods={methods}
        busy={false}
        keepOne={false}
        onSetPin={async (pin) => {
          onChoice({ ...choice, pin });
          return true;
        }}
        onRemovePin={async () => {
          onChoice({ ...choice, pin: "" });
          return true;
        }}
        onBiometry={async (biometry) => {
          onChoice({ ...choice, biometry });
          return true;
        }}
      />
      <p className="px-1 text-[11px] text-faint">
        {t("unlock-methods.choose")}
      </p>

      <ActionBar>
        <Button type="button" variant="ghost" size="pill" onClick={onBack}>
          {backLabel}
        </Button>
        <ForwardButton
          type="button"
          onClick={() => onChosen(choice)}
          disabled={!chosen}
        >
          {submitLabel}
        </ForwardButton>
      </ActionBar>
    </div>
  );
}
