import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import type { ExportState } from "../../vault-api.ts";
import { Block, BlockNote, BlockRow } from "../Block.tsx";
import { Button } from "../ui/button.tsx";

const exportSummaries: Record<ExportState, MessageKey> = {
  current: "settings.backup.state.current",
  stale: "settings.backup.state.stale",
  unknown: "settings.backup.state.unknown",
};

export function EncryptedCopy({
  state,
  busy,
  onExport,
}: {
  state: ExportState;
  busy: boolean;
  onExport: () => void;
}) {
  const { t } = useTranslator();

  return (
    <>
      <Block>
        <BlockRow title={t("settings.backup.copy")}>
          <Button
            type="button"
            variant="quiet"
            size="pill-sm"
            className="bg-tile font-normal hover:bg-field-hover"
            onClick={onExport}
            disabled={busy}
          >
            {t(busy ? "settings.backup.exporting" : "settings.backup.export")}
          </Button>
        </BlockRow>
      </Block>
      <BlockNote>{t(exportSummaries[state])}</BlockNote>
    </>
  );
}
