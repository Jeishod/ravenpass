import { useTranslator } from "../../i18n/translator.tsx";
import { useHostSetting } from "../../query/host-setting.ts";
import { queryKeys } from "../../query/keys.ts";
import type { ScreenshotSettings } from "../../vault-api.ts";
import { Block, BlockRow } from "../Block.tsx";
import { Switch } from "../ui/switch.tsx";

/** ScreenshotsSetting allows screenshots, screen recordings and the recent apps preview of the vault. */
export function ScreenshotsSetting({
  settings,
  busy,
}: {
  settings: ScreenshotSettings;
  busy: boolean;
}) {
  const { t } = useTranslator();
  const allowed = useHostSetting({
    key: queryKeys.screenshots,
    read: () => settings.screenshotsAllowed(),
    write: (next: boolean) => settings.allowScreenshots(next),
    readFailure: "app.error.setting-read",
    writeFailure: "settings.screenshots.error",
  });

  return (
    <Block className="mt-3.5">
      <BlockRow
        title={t("settings.screenshots")}
        detail={t("settings.screenshots.detail")}
        htmlFor="screenshots"
      >
        <Switch
          id="screenshots"
          checked={Boolean(allowed.value)}
          disabled={busy || allowed.changing || allowed.value === null}
          onCheckedChange={(next) => {
            void allowed.change(next);
          }}
        />
      </BlockRow>
    </Block>
  );
}
