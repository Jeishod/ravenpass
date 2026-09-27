import { useCapabilities } from "../../host/capabilities.tsx";
import { useTranslator } from "../../i18n/translator.tsx";
import type { AutofillSettings, ExtensionLinking } from "../../vault-api.ts";
import { BlockHeading } from "../Block.tsx";
import { AccountSuggestions } from "./AccountSuggestions.tsx";
import { LinkedExtensions } from "./LinkedExtensions.tsx";
import { SystemAutofillSetting } from "./SystemAutofillSetting.tsx";

/** AutofillPanel gathers every way of filling in that the host offers. */
export function AutofillPanel({
  settings,
  linking,
  verifiable,
  busy,
}: {
  settings: AutofillSettings;
  linking: ExtensionLinking;
  /** Whether the vault can ask the owner to confirm, by PIN or device authentication. */
  verifiable: boolean;
  busy: boolean;
}) {
  const { t } = useTranslator();
  const offers = useCapabilities();

  return (
    <>
      {offers.systemAutofill && (
        <SystemAutofillSetting settings={settings} busy={busy} />
      )}
      {offers.identityList && (
        <>
          <BlockHeading>
            {t("settings.autofill.suggestions.title")}
          </BlockHeading>
          <AccountSuggestions settings={settings} busy={busy} />
        </>
      )}
      {offers.extensions && (
        <>
          <BlockHeading>{t("settings.extensions.title")}</BlockHeading>
          <LinkedExtensions
            linking={linking}
            verifiable={verifiable}
            busy={busy}
          />
        </>
      )}
    </>
  );
}
