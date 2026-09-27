import { cn } from "cn";
import { Globe } from "lucide-react";
import { useState } from "react";
import { appKey } from "../credentials/apps.ts";
import {
  type CredentialDraft,
  emptyCredential,
  readyToSave,
} from "../credentials/credential.ts";
import { useTranslator } from "../i18n/translator.tsx";
import type {
  Credential,
  CredentialInput,
  CredentialLimits,
  Group,
  LinkedApp,
} from "../vault-api.ts";
import {
  bareField,
  EditorHeader,
  EditorRow,
  GroupField,
  NotesField,
  RemoveButton,
  roomFor,
  toggled,
  useRemaining,
  useRows,
} from "./editor/EditorFields.tsx";
import { Button } from "./ui/button.tsx";
import { Input } from "./ui/input.tsx";
import { ScrollArea } from "./ui/scroll-area.tsx";
import { RevealButton } from "./workspace/Fields.tsx";
import { LinkedApps } from "./workspace/LinkedApps.tsx";
import { Passkeys } from "./workspace/Passkeys.tsx";

const formID = "credential-editor";

export function CredentialEditor({
  initial,
  initialGroups,
  groups,
  limits,
  busy,
  onSave,
  onCancel,
}: {
  initial?: Credential;
  /** The groups the credential starts in, which for a new one is the chosen default. */
  initialGroups: string[];
  groups: Group[];
  limits: CredentialLimits | null;
  busy: boolean;
  onSave: (draft: CredentialDraft, groups: string[]) => void;
  onCancel: () => void;
}) {
  const { t } = useTranslator();
  const counter = useRemaining();
  const [input, setInput] = useState<CredentialInput>(
    initial ?? emptyCredential,
  );
  const websites = useRows(initial?.websites.length ? initial.websites : [""]);
  const [membership, setMembership] = useState<string[]>(initialGroups);
  const [showPassword, setShowPassword] = useState(false);
  const [removedPasskeys, setRemovedPasskeys] = useState<string[]>([]);
  const passkeys = (initial?.passkeys ?? []).filter(
    (passkey) => !removedPasskeys.includes(passkey.id),
  );
  const [apps, setApps] = useState<LinkedApp[]>(initial?.apps ?? []);

  function update(
    field: Exclude<keyof CredentialInput, "websites" | "apps">,
    value: string,
  ) {
    setInput((current) => ({ ...current, [field]: value }));
  }

  function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onSave(
      {
        input: readyToSave({ ...input, websites: websites.values, apps }),
        removedPasskeys,
      },
      membership,
    );
  }

  const name = input.label.trim();
  const holdsPasskeys = passkeys.length > 0;

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-[11px]">
      <EditorHeader
        title={t(
          initial
            ? "credential.editor.edit.title"
            : "credential.editor.new.title",
        )}
        name={name}
        form={formID}
        submit={t(
          initial ? "workspace.editor.update" : "credential.editor.create",
        )}
        canSubmit={Boolean(
          name && (input.password || input.totp.trim() || holdsPasskeys),
        )}
        busy={busy}
        onCancel={onCancel}
      />

      <ScrollArea className="min-h-0 flex-1">
        <form
          id={formID}
          onSubmit={submit}
          className="flex flex-1 flex-col gap-2"
        >
          <div className="shrink-0 overflow-hidden rounded-row bg-field">
            <EditorRow
              label={t("credential.field.label")}
              htmlFor="credential-name"
              counter={counter(input.label, limits?.label, 20)}
            >
              <Input
                id="credential-name"
                className={bareField}
                value={input.label}
                onChange={(event) => update("label", event.target.value)}
                placeholder="GitHub"
                maxLength={limits?.label}
                disabled={busy}
                required
              />
            </EditorRow>
            {websites.rows.map((row, index) => (
              <EditorRow
                key={row.key}
                label={t("credential.field.website")}
                htmlFor={`credential-website-${row.key}`}
                counter={counter(row.value, limits?.website, 20)}
              >
                <Input
                  id={`credential-website-${row.key}`}
                  className={bareField}
                  value={row.value}
                  onChange={(event) =>
                    websites.change(row.key, event.target.value)
                  }
                  placeholder={index === 0 ? "github.com" : undefined}
                  maxLength={limits?.website}
                  autoCapitalize="none"
                  spellCheck={false}
                  disabled={busy}
                />
                {websites.rows.length > 1 && (
                  <RemoveButton
                    label={t("credential.remove.website")}
                    busy={busy}
                    onRemove={() => websites.remove(row.key)}
                  />
                )}
              </EditorRow>
            ))}
            <EditorRow
              label={t("credential.field.login")}
              htmlFor="credential-username"
              counter={counter(input.login, limits?.login, 20)}
            >
              <Input
                id="credential-username"
                className={bareField}
                value={input.login}
                onChange={(event) => update("login", event.target.value)}
                maxLength={limits?.login}
                autoCapitalize="none"
                spellCheck={false}
                disabled={busy}
              />
            </EditorRow>
            <EditorRow
              label={t("credential.field.email")}
              htmlFor="credential-email"
              counter={counter(input.email, limits?.email, 20)}
            >
              <Input
                id="credential-email"
                type="email"
                className={bareField}
                value={input.email}
                onChange={(event) => update("email", event.target.value)}
                maxLength={limits?.email}
                autoCapitalize="none"
                spellCheck={false}
                disabled={busy}
              />
            </EditorRow>
            <EditorRow
              label={t("credential.field.password")}
              htmlFor="credential-password"
            >
              <Input
                id="credential-password"
                className={cn(bareField, "font-mono")}
                type={showPassword ? "text" : "password"}
                value={input.password}
                onChange={(event) => update("password", event.target.value)}
                maxLength={limits?.password}
                autoComplete="new-password"
                spellCheck={false}
                disabled={busy}
                required={!holdsPasskeys}
              />
              <RevealButton
                shown={showPassword}
                revealLabel={t("credential.password.reveal")}
                concealLabel={t("credential.password.conceal")}
                busy={busy}
                onToggle={() => setShowPassword((visible) => !visible)}
              />
            </EditorRow>
            {groups.length > 0 && (
              <GroupField
                groups={groups}
                membership={membership}
                busy={busy}
                onToggle={(id) =>
                  setMembership((current) => toggled(current, id))
                }
              />
            )}
            <EditorRow
              label={t("credential.field.totp")}
              htmlFor="credential-totp"
            >
              <Input
                id="credential-totp"
                className={cn(bareField, "font-mono text-xs")}
                value={input.totp}
                onChange={(event) => update("totp", event.target.value)}
                placeholder={t("credential.totp.placeholder")}
                maxLength={limits?.totp}
                autoCapitalize="none"
                autoComplete="off"
                spellCheck={false}
                disabled={busy}
              />
            </EditorRow>
          </div>

          <fieldset className="flex shrink-0 flex-wrap items-center gap-1.5 px-0.5">
            <legend className="sr-only">{t("credential.editor.add")}</legend>
            {roomFor(websites.rows.length, limits?.websites) && (
              <Button
                type="button"
                variant="quiet"
                size="pill-sm"
                disabled={busy}
                onClick={() => websites.add("")}
              >
                <Globe data-icon="inline-start" />
                {t("credential.field.website")}
              </Button>
            )}
          </fieldset>

          {holdsPasskeys && (
            <Passkeys
              passkeys={passkeys}
              action={(passkey) => (
                <RemoveButton
                  label={t("credential.remove.passkey")}
                  busy={busy}
                  onRemove={() =>
                    setRemovedPasskeys((current) => [...current, passkey.id])
                  }
                />
              )}
            />
          )}

          {apps.length > 0 && (
            <LinkedApps
              apps={apps}
              action={(app) => (
                <RemoveButton
                  label={t("credential.remove.app")}
                  busy={busy}
                  onRemove={() =>
                    setApps((current) =>
                      current.filter((kept) => appKey(kept) !== appKey(app)),
                    )
                  }
                />
              )}
            />
          )}

          <NotesField
            id="credential-notes"
            value={input.notes}
            limit={limits?.notes}
            busy={busy}
            onChange={(value) => update("notes", value)}
          />

          <p className="shrink-0 px-1 text-[11px] text-faint">
            {t(
              holdsPasskeys
                ? "credential.editor.requirement.passkeys"
                : "credential.editor.requirement",
            )}
          </p>
        </form>
      </ScrollArea>
    </div>
  );
}
