import { toast } from "sonner";
import { useTranslator } from "../../i18n/translator.tsx";
import type { VaultApi } from "../../vault-api.ts";
import {
  GeneratorForm,
  usePasswordGenerator,
} from "../generator/GeneratorForm.tsx";
import {
  ResponsiveDialog,
  ResponsiveDialogContent,
  ResponsiveDialogFooter,
  ResponsiveDialogHeader,
  ResponsiveDialogTitle,
} from "../ResponsiveDialog.tsx";
import { Button } from "../ui/button.tsx";

/** What the generator asks of the host. */
export type GeneratorHost = Pick<
  VaultApi,
  "seedWordlist" | "recordGeneratedPassword"
>;

/** GeneratorDialog shows a new password and its options; `onUse` takes the one shown, which the history records. */
export function GeneratorDialog({
  open,
  host,
  onUse,
  onClose,
}: {
  open: boolean;
  host: GeneratorHost;
  onUse: (password: string) => void;
  onClose: () => void;
}) {
  const { t, failure } = useTranslator();
  const generator = usePasswordGenerator(host, open);

  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={(next) => {
        if (!next) onClose();
      }}
    >
      <ResponsiveDialogContent className="sm:max-w-[420px]">
        <ResponsiveDialogHeader>
          <ResponsiveDialogTitle>{t("generator.title")}</ResponsiveDialogTitle>
        </ResponsiveDialogHeader>
        {generator && (
          <GeneratorForm
            generator={generator}
            actions={(generated) => (
              <ResponsiveDialogFooter>
                <Button
                  type="button"
                  variant="quiet"
                  size="pill"
                  onClick={onClose}
                >
                  {t("generator.cancel")}
                </Button>
                <Button
                  type="button"
                  variant="raised"
                  size="pill"
                  onClick={() => {
                    host
                      .recordGeneratedPassword(
                        generated.password,
                        generated.mode,
                      )
                      .catch((cause: unknown) =>
                        toast.error(failure(cause, "generator.error.record")),
                      );
                    onUse(generated.password);
                  }}
                >
                  {t("generator.use")}
                </Button>
              </ResponsiveDialogFooter>
            )}
          />
        )}
      </ResponsiveDialogContent>
    </ResponsiveDialog>
  );
}
