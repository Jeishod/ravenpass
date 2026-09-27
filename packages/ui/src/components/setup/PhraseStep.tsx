import { cn } from "cn";
import { Copy, Download, Printer, ShieldAlert } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { useCapabilities } from "../../host/capabilities.tsx";
import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import { cascade } from "../../motion/timings.ts";
import { splitWords } from "../../seeds/phrase.ts";
import type { VaultApi } from "../../vault-api.ts";
import { ActionBar } from "../AccessShell.tsx";
import { ForwardButton } from "../ForwardButton.tsx";
import { PhraseCell, PhraseGrid } from "../PhraseGrid.tsx";
import { Button } from "../ui/button.tsx";
import { Checkbox } from "../ui/checkbox.tsx";

type Keeping = "copy" | "save" | "print";

/** PhraseStep shows the recovery key being set up, of a new vault or a new key, to copy, save or print. */
export function PhraseStep({
  api,
  phrase,
  busy,
  onCancel,
  onContinue,
}: {
  api: Pick<
    VaultApi,
    "copyRecoveryKey" | "saveRecoveryKey" | "printRecoveryKey"
  >;
  phrase: string;
  busy: boolean;
  onCancel: () => void;
  onContinue: () => void;
}) {
  const { t, failure } = useTranslator();
  const { saveFiles, print } = useCapabilities();
  const [acknowledged, setAcknowledged] = useState(false);
  const [keeping, setKeeping] = useState<Keeping | null>(null);
  const words = splitWords(phrase).map((word, index) => ({
    word,
    position: index + 1,
  }));

  const failed: Record<Keeping, MessageKey> = {
    copy: saveFiles
      ? "wizard.errors.copy-failed"
      : "wizard.errors.copy-failed-no-file",
    save: "wizard.errors.save-failed",
    print: "wizard.errors.print-failed",
  };

  async function keep(how: Keeping) {
    setKeeping(how);
    try {
      if (how === "copy") {
        await api.copyRecoveryKey(phrase);
        toast.success(t("wizard.phrase.copy-done"));
      } else if (how === "save") {
        await api.saveRecoveryKey(phrase);
        toast.success(t("wizard.phrase.save-done"));
      } else {
        await api.printRecoveryKey(phrase);
      }
    } catch (reason) {
      toast.error(failure(reason, failed[how]));
    } finally {
      setKeeping(null);
    }
  }

  return (
    <div className="flex flex-col gap-3">
      <PhraseGrid
        label={t("wizard.phrase.list")}
        count={words.length}
        className="select-text"
      >
        {words.map(({ word, position }) => (
          <PhraseCell
            key={position}
            position={position}
            presence={cascade(position - 1)}
          >
            <span className="truncate text-[13px] font-medium">{word}</span>
          </PhraseCell>
        ))}
      </PhraseGrid>

      <div className="flex flex-wrap items-center gap-2">
        <Button
          type="button"
          size="pill-sm"
          variant="quiet"
          onClick={() => keep("copy")}
          disabled={keeping !== null || busy}
        >
          <Copy data-icon="inline-start" />
          {keeping === "copy"
            ? t("wizard.phrase.copy-busy")
            : t("wizard.phrase.copy")}
        </Button>
        {saveFiles && (
          <Button
            type="button"
            size="pill-sm"
            variant="quiet"
            onClick={() => keep("save")}
            disabled={keeping !== null || busy}
          >
            <Download data-icon="inline-start" />
            {keeping === "save"
              ? t("wizard.phrase.save-busy")
              : t("wizard.phrase.save")}
          </Button>
        )}
        {print && (
          <Button
            type="button"
            size="pill-sm"
            variant="quiet"
            onClick={() => keep("print")}
            disabled={keeping !== null || busy}
          >
            <Printer data-icon="inline-start" />
            {keeping === "print"
              ? t("wizard.phrase.print-busy")
              : t("wizard.phrase.print")}
          </Button>
        )}
      </div>

      <label
        htmlFor="recovery-acknowledgment"
        className={cn(
          "flex cursor-pointer items-start gap-3 rounded-2xl border px-4 py-3 transition-colors duration-200 motion-reduce:transition-none",
          acknowledged
            ? "border-foreground/25 bg-field"
            : "border-input hover:bg-field/60",
        )}
      >
        <Checkbox
          id="recovery-acknowledgment"
          checked={acknowledged}
          onCheckedChange={(checked) => setAcknowledged(checked === true)}
          className="mt-0.5"
        />
        <span className="flex flex-col gap-1">
          <span className="text-[13px]">
            {t("wizard.phrase.acknowledgment")}
          </span>
          <span className="flex items-start gap-1.5 text-[11px] leading-[1.5] text-faint">
            <ShieldAlert
              className="mt-px size-3.5 shrink-0"
              aria-hidden="true"
            />
            {t(saveFiles ? "wizard.phrase.note" : "wizard.phrase.note-words")}
          </span>
        </span>
      </label>

      <ActionBar>
        <Button
          type="button"
          variant="ghost"
          size="pill"
          onClick={onCancel}
          disabled={busy || keeping !== null}
        >
          {t("wizard.actions.cancel")}
        </Button>
        <ForwardButton
          type="button"
          onClick={onContinue}
          disabled={!acknowledged || busy || keeping !== null}
        >
          {t("wizard.actions.continue")}
        </ForwardButton>
      </ActionBar>
    </div>
  );
}
