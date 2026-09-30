import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { cn } from "cn";
import { useState } from "react";
import type { SignInStyle } from "../../extensions/sign-in-style.ts";
import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import { CalendarDate } from "../../identities/dates.ts";
import { useHostSetting } from "../../query/host-setting.ts";
import { queryKeys } from "../../query/keys.ts";
import type { ExtensionLinking, LinkedExtension } from "../../vault-api.ts";
import {
  Block,
  BlockHint,
  BlockNote,
  BlockRow,
  blockControl,
} from "../Block.tsx";
import { ConfirmDialog } from "../ConfirmDialog.tsx";
import { Button } from "../ui/button.tsx";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../ui/select.tsx";
import { Switch } from "../ui/switch.tsx";
import { LinkExtensionDialog } from "./LinkExtensionDialog.tsx";
import { NameDialog } from "./NameDialog.tsx";

/** The desktop app's limit on an extension's name, in characters. */
const extensionNameLength = 64;

const signInStyleNames: Record<SignInStyle, MessageKey> = {
  card: "settings.extensions.sign-in.card",
  field: "settings.extensions.sign-in.field",
};

/** LinkedExtensions links, renames and unlinks browser extensions, and chooses how they offer sign-in and fill. */
export function LinkedExtensions({
  linking,
  verifiable,
  busy,
}: {
  linking: ExtensionLinking;
  /** Without a way to confirm, every fill would be refused, so confirmation can only be turned off. */
  verifiable: boolean;
  busy: boolean;
}) {
  const { t, language } = useTranslator();
  const client = useQueryClient();
  const [offering, setOffering] = useState(false);
  const [unlinking, setUnlinking] = useState<LinkedExtension | null>(null);
  const [renaming, setRenaming] = useState<LinkedExtension | null>(null);
  const links =
    useQuery({
      queryKey: queryKeys.extensionLinks,
      queryFn: () => linking.extensionLinks(),
      meta: { failure: "settings.extensions.error.load" },
    }).data ?? null;
  const signIn = useHostSetting({
    key: queryKeys.signInStyle,
    read: () => linking.signInStyle(),
    write: (style: SignInStyle) => linking.setSignInStyle(style),
    readFailure: "app.error.setting-read",
    writeFailure: "settings.extensions.error.sign-in",
  });
  const confirmFills = useHostSetting({
    key: queryKeys.extensionFillConfirmation,
    read: () => linking.confirmExtensionFills(),
    write: (confirm: boolean) => linking.setConfirmExtensionFills(confirm),
    readFailure: "app.error.setting-read",
    writeFailure: "settings.extensions.error.confirm-fills",
  });
  const unlink = useMutation({
    mutationFn: (extension: LinkedExtension) =>
      linking.unlinkExtension(extension.id),
    meta: { failure: "settings.extensions.error.unlink" },
    onSettled: () => refresh(),
  });
  const rename = useMutation({
    mutationFn: ({ id, name }: { id: string; name: string }) =>
      linking.renameExtension(id, name),
    meta: { failure: "settings.extensions.error.rename" },
    onSettled: () => refresh(),
  });

  function refresh() {
    return client.invalidateQueries({ queryKey: queryKeys.extensionLinks });
  }

  // Offering a key can change whether Ravenpass listens.
  function closeOffer() {
    setOffering(false);
    void refresh();
  }

  const extensions = links?.extensions ?? [];

  function linkedDate(extension: LinkedExtension) {
    return CalendarDate.of(new Date(extension.linkedAt)).format(language);
  }

  return (
    <>
      {extensions.length > 0 && (
        <Block>
          {extensions.map((extension) => (
            <BlockRow
              key={extension.id}
              title={extension.name}
              detail={t("settings.extensions.linked", {
                date: linkedDate(extension),
              })}
            >
              <Button
                type="button"
                variant="quiet"
                size="pill-sm"
                className="bg-tile font-normal hover:bg-field-hover"
                aria-label={t("settings.extensions.rename.action", {
                  name: extension.name,
                })}
                disabled={busy}
                onClick={() => setRenaming(extension)}
              >
                {t("settings.extensions.rename")}
              </Button>
              <Button
                type="button"
                variant="quiet"
                size="pill-sm"
                className="bg-tile font-normal hover:bg-field-hover"
                aria-label={t("settings.extensions.unlink.action", {
                  name: extension.name,
                  date: linkedDate(extension),
                })}
                disabled={busy}
                onClick={() => setUnlinking(extension)}
              >
                {t("settings.extensions.unlink")}
              </Button>
            </BlockRow>
          ))}
        </Block>
      )}
      {links && extensions.length === 0 && (
        <BlockNote>{t("settings.extensions.empty")}</BlockNote>
      )}
      {links && !links.reachable && (
        <BlockHint>{t("settings.extensions.unreachable")}</BlockHint>
      )}
      <Button
        type="button"
        variant="quiet"
        size="pill-sm"
        className="mt-3 mb-[18px] bg-tile font-normal hover:bg-field-hover"
        disabled={busy}
        onClick={() => setOffering(true)}
      >
        {t("settings.extensions.link")}
      </Button>

      <Block>
        <BlockRow
          title={t("settings.extensions.sign-in")}
          htmlFor="sign-in-style"
        >
          <Select
            value={signIn.value?.style}
            disabled={busy || signIn.changing || !signIn.value}
            onValueChange={(next) => {
              const style = signIn.value?.offered.find(
                (offered) => offered === next,
              );
              if (style) void signIn.change(style);
            }}
          >
            <SelectTrigger
              id="sign-in-style"
              size="sm"
              className={cn(blockControl, "max-w-[170px]")}
              aria-label={t("settings.extensions.sign-in")}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {signIn.value?.offered.map((style) => (
                  <SelectItem key={style} value={style}>
                    {t(signInStyleNames[style])}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </BlockRow>
        <BlockRow
          title={t("settings.extensions.confirm-fills")}
          detail={t(
            verifiable
              ? "settings.extensions.confirm-fills.detail"
              : "settings.extensions.confirm-fills.unavailable",
          )}
          htmlFor="confirm-extension-fills"
          wrap
        >
          <Switch
            id="confirm-extension-fills"
            checked={Boolean(confirmFills.value)}
            disabled={
              busy ||
              confirmFills.changing ||
              confirmFills.value === null ||
              (!verifiable && !confirmFills.value)
            }
            onCheckedChange={(next) => {
              void confirmFills.change(next);
            }}
          />
        </BlockRow>
      </Block>

      {offering && (
        <LinkExtensionDialog linking={linking} onClose={closeOffer} />
      )}
      <NameDialog
        key={renaming?.id ?? "rename"}
        open={renaming !== null}
        title={t("settings.extensions.rename.title")}
        label={t("settings.extensions.name")}
        maxLength={extensionNameLength}
        save={t("settings.extensions.rename.save")}
        cancel={t("settings.extensions.rename.cancel")}
        initial={renaming?.name ?? ""}
        busy={busy || rename.isPending}
        onSave={(name) =>
          renaming
            ? rename.mutateAsync({ id: renaming.id, name }).then(
                () => true,
                () => false,
              )
            : Promise.resolve(false)
        }
        onClose={() => setRenaming(null)}
      />
      <ConfirmDialog
        open={unlinking !== null}
        title={t("settings.extensions.unlink.title", {
          name: unlinking?.name ?? "",
        })}
        detail={
          unlinking
            ? t("settings.extensions.unlink.detail", {
                date: linkedDate(unlinking),
              })
            : ""
        }
        confirm={t("settings.extensions.unlink.confirm")}
        cancel={t("settings.extensions.unlink.cancel")}
        destructive
        busy={unlink.isPending}
        onConfirm={() => {
          if (unlinking) {
            unlink.mutate(unlinking, { onSettled: () => setUnlinking(null) });
          }
        }}
        onCancel={() => setUnlinking(null)}
      />
    </>
  );
}
