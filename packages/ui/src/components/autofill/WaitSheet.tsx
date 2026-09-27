import { cn } from "cn";
import { CircleAlert, LoaderCircle } from "lucide-react";
import type {
  AutofillFailure,
  AutofillWait,
} from "../../autofill/autofill-api.ts";
import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import { ResponsiveDialogFooter } from "../ResponsiveDialog.tsx";
import { Button } from "../ui/button.tsx";
import { AutofillSheet } from "./AutofillSheet.tsx";

const steps: Record<
  Exclude<AutofillWait["kind"], "failed">,
  { title: MessageKey; detail?: MessageKey }
> = {
  opening: { title: "autofill.wait.opening" },
  unlocking: {
    title: "confirmation.unlock.title",
    detail: "autofill.wait.unlock",
  },
  verifying: {
    title: "autofill.wait.verify.title",
    detail: "autofill.wait.verify",
  },
};

const failures: Record<AutofillFailure, MessageKey> = {
  unreachable: "autofill.failed.unreachable",
  outdated: "autofill.failed.outdated",
  "fill-failed": "fill.error",
  "code-failed": "fill-code.error",
  "sign-in-failed": "autofill.failed.sign-in",
  "save-failed": "autofill.failed.save-passkey",
  unverifiable: "autofill.failed.unverifiable",
  unsupported: "autofill.failed.unsupported",
  failed: "autofill.failed.other",
};

/** WaitSheet shows what Ravenpass does outside the page for the request; cancelling ends the request. */
export function WaitSheet({
  wait,
  onCancel,
}: {
  wait: AutofillWait;
  onCancel: () => void;
}) {
  const { t } = useTranslator();
  const failed = wait.kind === "failed";
  const shown: { title: MessageKey; detail?: MessageKey } =
    wait.kind === "failed"
      ? { title: "autofill.failed.title", detail: failures[wait.failure] }
      : steps[wait.kind];
  const Icon = failed ? CircleAlert : LoaderCircle;

  return (
    <AutofillSheet
      title={t(shown.title)}
      description={shown.detail && t(shown.detail)}
      busy={false}
      onClose={onCancel}
    >
      <div className="flex justify-center py-6">
        <Icon
          className={cn(
            "size-6 text-muted-foreground",
            !failed && "animate-spin",
          )}
          aria-hidden="true"
        />
      </div>
      <ResponsiveDialogFooter>
        <Button type="button" variant="quiet" size="pill" onClick={onCancel}>
          {t(failed ? "autofill.close" : "autofill.cancel")}
        </Button>
      </ResponsiveDialogFooter>
    </AutofillSheet>
  );
}
