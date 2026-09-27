import { useState } from "react";
import {
  codeSetupChoices,
  namesCredential,
} from "../../credentials/code-setup.ts";
import { accountOf } from "../../credentials/credential.ts";
import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import type { CodeSetup, CredentialSummary } from "../../vault-api.ts";
import { ConfirmDialog } from "../ConfirmDialog.tsx";
import {
  ResponsiveDialog,
  ResponsiveDialogContent,
  ResponsiveDialogDescription,
  ResponsiveDialogFooter,
  ResponsiveDialogHeader,
  ResponsiveDialogTitle,
} from "../ResponsiveDialog.tsx";
import { SearchField } from "../SearchField.tsx";
import { Button } from "../ui/button.tsx";
import { ChoiceRow } from "./ChoiceRow.tsx";

/** The dialog's description, naming what the setup names. */
function detailOf(setup: CodeSetup): MessageKey {
  if (setup.issuer && setup.account) return "code-setup.detail.issuer-account";
  if (setup.issuer) return "code-setup.detail.issuer";
  if (setup.account) return "code-setup.detail.account";
  return "code-setup.detail.unnamed";
}

/** CodeSetupDialog adds a received code setup to a credential; replacing an existing code asks first. */
export function CodeSetupDialog({
  setup,
  credentials,
  busy,
  adding,
  onAdd,
  onCreate,
  onDismiss,
}: {
  setup: CodeSetup;
  credentials: CredentialSummary[];
  busy: boolean;
  adding: string | null;
  onAdd: (credentialId: string) => void;
  onCreate: () => void;
  onDismiss: () => void;
}) {
  const { t } = useTranslator();
  const [query, setQuery] = useState("");
  const [replacing, setReplacing] = useState<CredentialSummary | null>(null);
  const choices = codeSetupChoices(credentials, setup, query);
  const label = (credential: CredentialSummary) =>
    credential.label || t("credential.untitled");

  function choose(credential: CredentialSummary) {
    if (credential.oneTimeCode) setReplacing(credential);
    else onAdd(credential.id);
  }

  return (
    <ResponsiveDialog
      open
      dismissible={!busy}
      onOpenChange={(open) => {
        if (!open) onDismiss();
      }}
    >
      <ResponsiveDialogContent className="bg-background sm:max-w-[400px]">
        <ResponsiveDialogHeader>
          <ResponsiveDialogTitle>{t("code-setup.title")}</ResponsiveDialogTitle>
          <ResponsiveDialogDescription>
            {t(detailOf(setup), {
              issuer: setup.issuer,
              account: setup.account,
            })}
          </ResponsiveDialogDescription>
        </ResponsiveDialogHeader>
        <SearchField
          id="code-setup-search"
          value={query}
          onChange={setQuery}
          label={t("workspace.toolbar.search")}
          placeholder={t("workspace.toolbar.search.placeholder")}
        />
        <div className="grid max-h-[min(320px,45dvh)] gap-1 overflow-y-auto">
          {choices.map((credential) => (
            <ChoiceRow
              key={credential.id}
              site={credential.site}
              title={label(credential)}
              detail={accountOf(credential) || t("credential.login.empty")}
              busy={busy}
              working={adding === credential.id}
              onChoose={() => choose(credential)}
            />
          ))}
          {!choices.length && (
            <p className="px-3 py-2 text-[13px] text-muted-foreground">
              {t(
                query.trim()
                  ? "workspace.list.empty.search"
                  : "code-setup.empty",
              )}
            </p>
          )}
        </div>
        <ResponsiveDialogFooter>
          <Button
            type="button"
            variant="quiet"
            size="pill"
            disabled={busy}
            onClick={onDismiss}
          >
            {t("code-setup.cancel")}
          </Button>
          {namesCredential(setup) && (
            <Button
              type="button"
              variant="raised"
              size="pill"
              disabled={busy}
              onClick={onCreate}
            >
              {t("code-setup.new")}
            </Button>
          )}
        </ResponsiveDialogFooter>
        <ConfirmDialog
          open={replacing !== null}
          title={t("code-setup.replace.title")}
          detail={t("code-setup.replace.detail", {
            label: replacing ? label(replacing) : "",
          })}
          confirm={t("code-setup.replace.confirm")}
          cancel={t("code-setup.cancel")}
          busy={busy}
          onConfirm={() => {
            if (replacing) onAdd(replacing.id);
            setReplacing(null);
          }}
          onCancel={() => setReplacing(null)}
        />
      </ResponsiveDialogContent>
    </ResponsiveDialog>
  );
}
