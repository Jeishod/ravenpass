import { useTranslator } from "../../i18n/translator.tsx";
import { useHostSetting } from "../../query/host-setting.ts";
import { queryKeys } from "../../query/keys.ts";
import type { AutofillSettings } from "../../vault-api.ts";
import { Block, BlockNote, BlockRow } from "../Block.tsx";
import { Switch } from "../ui/switch.tsx";

/** AccountSuggestions chooses whether the system's AutoFill suggests the vault's accounts. */
export function AccountSuggestions({
  settings,
  busy,
}: {
  settings: AutofillSettings;
  busy: boolean;
}) {
  const { t } = useTranslator();
  const list = useHostSetting({
    key: queryKeys.identityList,
    read: () => settings.identityList(),
    write: (enabled: boolean) => settings.setIdentityList(enabled),
    readFailure: "app.error.setting-read",
    writeFailure: "settings.autofill.error.suggestions",
  });

  return (
    <>
      <Block>
        <BlockRow
          title={t("settings.autofill.suggestions")}
          detail={t("settings.autofill.suggestions.detail")}
          htmlFor="autofill-suggestions"
        >
          <Switch
            id="autofill-suggestions"
            checked={Boolean(list.value?.enabled)}
            disabled={busy || list.changing || !list.value}
            onCheckedChange={(enabled) => {
              void list.change(enabled);
            }}
          />
        </BlockRow>
      </Block>
      <BlockNote>{t("settings.autofill.suggestions.note")}</BlockNote>
    </>
  );
}
