import { Copy, ExternalLink, KeyRound } from "lucide-react";
import { useState } from "react";
import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import type {
  OneTimeCode as Code,
  Credential,
  CredentialField,
} from "../../vault-api.ts";
import { Button } from "../ui/button.tsx";
import { ScrollArea } from "../ui/scroll-area.tsx";
import { type DetailControls, DetailHeader } from "./DetailHeader.tsx";
import {
  CopyButton,
  FieldBlock,
  FieldRow,
  NotesBlock,
  SecretRow,
} from "./Fields.tsx";
import { LinkedApps } from "./LinkedApps.tsx";
import { OneTimeCode } from "./OneTimeCode.tsx";
import { Passkeys } from "./Passkeys.tsx";
import { useSiteIcon } from "./SiteIcons.tsx";

/** CredentialDetail owns the password reveal, so it ends with this pane. */
export function CredentialDetail({
  credential,
  controls,
  onCopy,
  onGenerateCode,
  onOpenWebsite,
}: {
  credential: Credential;
  controls: DetailControls;
  /** Copies one value, and names the notice that says what was copied. */
  onCopy: (field: CredentialField, notice: MessageKey) => void;
  onGenerateCode: (setup: string) => Promise<Code>;
  onOpenWebsite: (address: string) => void;
}) {
  const { t } = useTranslator();
  const [revealPassword, setRevealPassword] = useState(false);
  const { busy } = controls;
  const logo = useSiteIcon(credential.site);

  return (
    <ScrollArea className="min-h-0 flex-1">
      <article className="flex flex-1 flex-col gap-[11px]">
        <DetailHeader
          label={credential.label}
          logo={logo}
          title={credential.label || t("credential.untitled")}
          icon={KeyRound}
          subtitle={credential.websites[0] || t("credential.website.empty")}
          deleteTitle={t("credential.delete.title")}
          controls={controls}
        />

        <div className="shrink-0">
          <FieldBlock>
            {credential.login && (
              <FieldRow
                label={t("credential.field.login")}
                action={t("credential.copy.login")}
                icon={Copy}
                disabled={busy}
                onAction={() => onCopy("login", "workspace.copy.login")}
              >
                <span className="min-w-0 flex-1 truncate text-[13px]">
                  {credential.login}
                </span>
              </FieldRow>
            )}
            {credential.email && (
              <FieldRow
                label={t("credential.field.email")}
                action={t("credential.copy.email")}
                icon={Copy}
                disabled={busy}
                onAction={() => onCopy("email", "workspace.copy.email")}
              >
                <span className="min-w-0 flex-1 truncate text-[13px]">
                  {credential.email}
                </span>
              </FieldRow>
            )}
            {credential.password && (
              <SecretRow
                label={t("credential.field.password")}
                value={credential.password}
                revealed={revealPassword}
                revealLabel={t("credential.password.reveal")}
                concealLabel={t("credential.password.conceal")}
                copyLabel={t("credential.copy.password")}
                busy={busy}
                onReveal={() => setRevealPassword((shown) => !shown)}
                onCopy={() => onCopy("password", "workspace.copy.password")}
              />
            )}
            {credential.websites.map((address, index) => (
              <FieldRow
                // biome-ignore lint/suspicious/noArrayIndexKey: websites carry no id and may repeat, and the list never reorders while shown.
                key={index}
                label={t("credential.field.website")}
                action={t("credential.website.open")}
                icon={ExternalLink}
                disabled={busy}
                onAction={() => onOpenWebsite(address)}
                accessory={
                  <CopyButton
                    label={t("credential.copy.website")}
                    busy={busy}
                    onCopy={() =>
                      onCopy(`website:${index}`, "workspace.copy.website")
                    }
                  />
                }
              >
                <span className="min-w-0 flex-1 truncate text-[13px]">
                  {address}
                </span>
              </FieldRow>
            ))}
          </FieldBlock>
        </div>

        {credential.totp && (
          <OneTimeCode
            setup={credential.totp}
            generate={onGenerateCode}
            onCopy={() => onCopy("totp", "workspace.copy.totp")}
            busy={busy}
          />
        )}

        {credential.passkeys.length > 0 && (
          <Passkeys passkeys={credential.passkeys} />
        )}

        {credential.apps.length > 0 && <LinkedApps apps={credential.apps} />}

        {credential.notes && (
          <NotesBlock
            label={t("workspace.field.notes")}
            notes={credential.notes}
            copyLabel={t("workspace.notes.copy")}
            busy={busy}
            onCopy={() => onCopy("notes", "workspace.copy.notes")}
          />
        )}

        {credential.password && (
          <Button
            type="button"
            variant="raised"
            size="pill"
            className="mt-auto shrink-0 text-[13px]"
            disabled={busy}
            onClick={() => onCopy("password", "workspace.copy.password")}
          >
            <Copy data-icon="inline-start" />
            {t("credential.copy.password")}
          </Button>
        )}
      </article>
    </ScrollArea>
  );
}
