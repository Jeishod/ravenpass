import type { ReactNode } from "react";
import { appKey } from "../../credentials/apps.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import type { LinkedApp } from "../../vault-api.ts";
import { Block, BlockRow } from "../Block.tsx";
import { TitledBlock } from "./Fields.tsx";

/** LinkedApps lists a credential's apps by the names the device shows, else by package; `action` adds a control at the end of each row. */
export function LinkedApps({
  apps,
  action,
}: {
  apps: LinkedApp[];
  action?: (app: LinkedApp) => ReactNode;
}) {
  const { t } = useTranslator();

  return (
    <TitledBlock title={t("credential.apps")}>
      <Block>
        {apps.map((app) => (
          <BlockRow
            key={appKey(app)}
            title={app.name || app.package}
            detail={app.name ? app.package : undefined}
          >
            {action?.(app)}
          </BlockRow>
        ))}
      </Block>
    </TitledBlock>
  );
}
