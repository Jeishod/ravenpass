import { useQuery } from "@tanstack/react-query";
import { cn } from "cn";
import { Copy } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import type { GeneratedPassword } from "../../credentials/password-generator.ts";
import { useCompactLayout } from "../../host/compact.ts";
import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import { queryKeys } from "../../query/keys.ts";
import type { GeneratorHistoryEntry, VaultApi } from "../../vault-api.ts";
import { BlockHeading, BlockNote, WrapBlockText } from "../Block.tsx";
import { ConfirmDialog } from "../ConfirmDialog.tsx";
import { Button } from "../ui/button.tsx";
import { ScrollArea } from "../ui/scroll-area.tsx";
import { FieldBlock, SecretRow } from "../workspace/Fields.tsx";
import { GeneratorForm, usePasswordGenerator } from "./GeneratorForm.tsx";

/** Passwords of the history shown before the owner asks for more. */
const historyPage = 100;

export type GeneratorPlaceApi = Pick<
  VaultApi,
  | "seedWordlist"
  | "generatorHistory"
  | "generatorHistorySetting"
  | "recordGeneratedPassword"
  | "clearGeneratorHistory"
  | "copyGeneratedPassword"
>;

/**
 * GeneratorView makes passwords to copy and lists those copied or used. The history lives only in this view's state,
 * never in the query cache, so it goes with the view.
 */
export function GeneratorView({ api }: { api: GeneratorPlaceApi }) {
  const { t, failure, language } = useTranslator();
  const compact = useCompactLayout();
  const generator = usePasswordGenerator(api);
  const setting = useQuery({
    queryKey: queryKeys.generatorHistorySetting,
    queryFn: () => api.generatorHistorySetting(),
    meta: { failure: "app.error.setting-read" },
  }).data;
  const [history, setHistory] = useState<GeneratorHistoryEntry[]>([]);
  const [shown, setShown] = useState(historyPage);
  const [revealed, setRevealed] = useState<ReadonlySet<string>>(new Set());
  const [clearing, setClearing] = useState(false);
  const [busy, setBusy] = useState(false);

  function report(cause: unknown, message: MessageKey) {
    toast.error(failure(cause, message));
  }

  function reload() {
    api
      .generatorHistory()
      .then(setHistory)
      .catch((cause: unknown) => report(cause, "generator.error.history"));
  }

  // biome-ignore lint/correctness/useExhaustiveDependencies: the history is read once per view and after each copy.
  useEffect(reload, [api]);

  /** Resolves whether the password reached the clipboard. */
  async function copy(value: string): Promise<boolean> {
    try {
      await api.copyGeneratedPassword(value);
      toast.success(t("generator.copied"));
      return true;
    } catch (cause) {
      report(cause, "generator.error.copy");
      return false;
    }
  }

  async function copyGenerated(generated: GeneratedPassword) {
    if (!(await copy(generated.password))) return;
    try {
      await api.recordGeneratedPassword(generated.password, generated.mode);
      reload();
    } catch (cause) {
      report(cause, "generator.error.record");
    }
  }

  async function clearHistory() {
    setBusy(true);
    try {
      await api.clearGeneratorHistory();
      setHistory([]);
      setShown(historyPage);
      setRevealed(new Set());
      setClearing(false);
      toast.success(t("generator.history.cleared"));
    } catch (cause) {
      report(cause, "generator.error.clear");
    } finally {
      setBusy(false);
    }
  }

  function toggleRevealed(key: string) {
    setRevealed((current) => {
      const next = new Set(current);
      if (!next.delete(key)) next.add(key);
      return next;
    });
  }

  const when = new Intl.DateTimeFormat(language, {
    dateStyle: "short",
    timeStyle: "short",
  });

  return (
    <ScrollArea className="min-h-0 flex-1">
      <WrapBlockText>
        <div className="max-w-[524px] p-5 max-sm:px-4">
          <h2
            className={cn(
              "mb-[21px] font-medium leading-[1.35]",
              compact ? "text-[22px]" : "text-[17px]",
            )}
          >
            {t("workspace.rail.generator")}
          </h2>
          {generator && (
            <GeneratorForm
              generator={generator}
              actions={(generated) => (
                <div className="flex justify-end">
                  <Button
                    type="button"
                    variant="raised"
                    size="pill"
                    onClick={() => void copyGenerated(generated)}
                  >
                    <Copy data-icon="inline-start" />
                    {t("generator.copy")}
                  </Button>
                </div>
              )}
            />
          )}

          <BlockHeading>{t("generator.history")}</BlockHeading>
          {history.length === 0 ? (
            setting && (
              <BlockNote>
                {setting.enabled
                  ? t("generator.history.empty", { count: setting.days })
                  : t("generator.history.off")}
              </BlockNote>
            )
          ) : (
            <>
              <FieldBlock>
                {history.slice(0, shown).map((entry, index) => {
                  const key = `${entry.at}-${index}`;
                  return (
                    <SecretRow
                      key={key}
                      label={when.format(new Date(entry.at))}
                      value={entry.value}
                      revealed={revealed.has(key)}
                      wrap
                      revealLabel={t("credential.password.reveal")}
                      concealLabel={t("credential.password.conceal")}
                      copyLabel={t("generator.copy")}
                      busy={busy}
                      onReveal={() => toggleRevealed(key)}
                      onCopy={() => void copy(entry.value)}
                    />
                  );
                })}
              </FieldBlock>
              <div className="mt-3 flex flex-wrap gap-2">
                {history.length > shown && (
                  <Button
                    type="button"
                    variant="quiet"
                    size="pill-sm"
                    className="bg-tile font-normal hover:bg-field-hover"
                    onClick={() => setShown((count) => count + historyPage)}
                  >
                    {t("generator.history.more", {
                      count: history.length - shown,
                    })}
                  </Button>
                )}
                <Button
                  type="button"
                  variant="quiet"
                  size="pill-sm"
                  className="bg-tile font-normal text-destructive hover:bg-field-hover"
                  disabled={busy}
                  onClick={() => setClearing(true)}
                >
                  {t("generator.history.clear")}
                </Button>
              </div>
            </>
          )}
        </div>
      </WrapBlockText>
      <ConfirmDialog
        open={clearing}
        title={t("generator.history.clear.title", { count: history.length })}
        detail={t("generator.history.clear.detail")}
        confirm={t("generator.history.clear.confirm")}
        cancel={t("generator.cancel")}
        destructive
        busy={busy}
        onConfirm={() => void clearHistory()}
        onCancel={() => setClearing(false)}
      />
    </ScrollArea>
  );
}
