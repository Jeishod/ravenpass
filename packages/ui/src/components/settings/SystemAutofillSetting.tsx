import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { usePageVisible } from "../../host/visibility.ts";
import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import { queryKeys } from "../../query/keys.ts";
import type { AutofillSettings } from "../../vault-api.ts";
import { Block, BlockNote, BlockRow } from "../Block.tsx";
import { Button } from "../ui/button.tsx";
import {
  type SystemAutofillRole,
  systemAutofillRoles,
} from "./system-autofill.ts";

const roleNames: Record<SystemAutofillRole, MessageKey> = {
  autofill: "settings.autofill.system.autofill",
  passkeys: "settings.autofill.system.passkeys",
};

const roleDetails: Record<SystemAutofillRole, MessageKey> = {
  autofill: "settings.autofill.system.autofill.detail",
  passkeys: "settings.autofill.system.passkeys.detail",
};

/** SystemAutofillSetting says whether the device fills in through Ravenpass and opens the system screen. */
export function SystemAutofillSetting({
  settings,
  busy,
}: {
  settings: AutofillSettings;
  busy: boolean;
}) {
  const { t } = useTranslator();
  const client = useQueryClient();
  const visible = usePageVisible();
  // The owner can change it in the system's settings at any time; each showing reads it again.
  const status =
    useQuery({
      queryKey: queryKeys.systemAutofill,
      queryFn: () => settings.systemAutofill(),
      enabled: visible,
      staleTime: 0,
      meta: { failure: "app.error.setting-read" },
    }).data ?? null;
  const choose = useMutation({
    mutationFn: () => settings.chooseSystemAutofill(),
    meta: { failure: "settings.autofill.system.error" },
    onSettled: () =>
      client.invalidateQueries({ queryKey: queryKeys.systemAutofill }),
  });

  return (
    <>
      {status && (
        <Block>
          {systemAutofillRoles(status).map(({ role, on }) => (
            <BlockRow
              key={role}
              title={t(roleNames[role])}
              detail={t(roleDetails[role])}
            >
              <span className="rounded-full bg-tile px-2 py-px text-[11px] text-secondary-foreground">
                {t(
                  on
                    ? "settings.autofill.system.on"
                    : "settings.autofill.system.off",
                )}
              </span>
            </BlockRow>
          ))}
        </Block>
      )}
      <Button
        type="button"
        variant="raised"
        size="pill"
        className="mt-3.5 w-full first:mt-0"
        disabled={busy || choose.isPending}
        onClick={() => choose.mutate()}
      >
        {t("settings.autofill.system.open")}
      </Button>
      <BlockNote>{t("settings.autofill.system.chrome")}</BlockNote>
    </>
  );
}
