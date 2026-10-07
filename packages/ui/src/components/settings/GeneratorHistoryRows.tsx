import { useState } from "react";
import { toast } from "sonner";
import { useTranslator } from "../../i18n/translator.tsx";
import type { GeneratorHistorySetting, VaultApi } from "../../vault-api.ts";
import { BlockRow, blockControl } from "../Block.tsx";
import { ConfirmDialog } from "../ConfirmDialog.tsx";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../ui/select.tsx";
import { Switch } from "../ui/switch.tsx";

/** A change that deletes passwords from the history at once, waiting for confirmation. */
interface Pending {
  enabled: boolean;
  days: number;
  count: number;
}

/**
 * GeneratorHistoryRows keep the generator's history on or off and its period, confirming a change that deletes
 * passwords already kept.
 */
export function GeneratorHistoryRows({
  setting,
  busy,
  history,
  onChange,
}: {
  setting: GeneratorHistorySetting | null;
  busy: boolean;
  history: Pick<VaultApi, "countGeneratorHistoryPast">;
  onChange: (enabled: boolean, days: number) => void;
}) {
  const { t, failure } = useTranslator();
  const [pending, setPending] = useState<Pending | null>(null);
  const unknown = busy || !setting;

  // Turning the history off deletes all of it; a shorter period deletes what it no longer keeps.
  async function choose(enabled: boolean, days: number) {
    if (!setting) return;
    const deletes = !enabled || (setting.enabled && days < setting.days);
    if (!deletes) {
      onChange(enabled, days);
      return;
    }
    try {
      const count = await history.countGeneratorHistoryPast(enabled ? days : 0);
      if (count > 0) setPending({ enabled, days, count });
      else onChange(enabled, days);
    } catch (cause) {
      toast.error(failure(cause, "settings.generator-history.error"));
    }
  }

  return (
    <>
      <BlockRow
        title={t("settings.generator-history")}
        detail={t("settings.generator-history.detail")}
        htmlFor="generator-history"
        wrap
      >
        <Switch
          id="generator-history"
          checked={Boolean(setting?.enabled)}
          disabled={unknown}
          onCheckedChange={(enabled) => {
            if (setting) void choose(enabled, setting.days);
          }}
        />
      </BlockRow>
      <BlockRow
        title={t("settings.generator-history.period")}
        htmlFor="generator-history-period"
      >
        <Select
          value={setting ? String(setting.days) : undefined}
          onValueChange={(next) => {
            if (setting) void choose(setting.enabled, Number(next));
          }}
          disabled={unknown || !setting?.enabled}
        >
          <SelectTrigger
            id="generator-history-period"
            size="sm"
            className={blockControl}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              {(setting?.offered ?? []).map((days) => (
                <SelectItem key={days} value={String(days)}>
                  {t("settings.generator-history.days", { count: days })}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
      </BlockRow>
      <ConfirmDialog
        open={pending !== null}
        title={t(
          pending?.enabled
            ? "settings.generator-history.shorten.title"
            : "settings.generator-history.off.title",
          { count: pending?.count ?? 0 },
        )}
        detail={t("generator.history.clear.detail")}
        confirm={t(
          pending?.enabled
            ? "settings.generator-history.shorten.confirm"
            : "settings.generator-history.off.confirm",
        )}
        cancel={t("settings.generator-history.cancel")}
        destructive
        busy={busy}
        onConfirm={() => {
          if (pending) onChange(pending.enabled, pending.days);
          setPending(null);
        }}
        onCancel={() => setPending(null)}
      />
    </>
  );
}
