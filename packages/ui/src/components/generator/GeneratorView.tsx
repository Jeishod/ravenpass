import { Copy, RefreshCw, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import type {
  GeneratedPassword,
  GeneratorKind,
  GeneratorOptions,
  VaultApi,
} from "../../vault-api.ts";
import {
  Block,
  BlockHeading,
  BlockNote,
  BlockRow,
  WrapBlockText,
} from "../Block.tsx";
import { ConfirmDialog } from "../ConfirmDialog.tsx";
import { Button } from "../ui/button.tsx";
import { Input } from "../ui/input.tsx";
import { ScrollArea } from "../ui/scroll-area.tsx";
import { Slider } from "../ui/slider.tsx";
import { Switch } from "../ui/switch.tsx";
import { ToggleGroup, ToggleGroupItem } from "../ui/toggle-group.tsx";
import {
  clampOption,
  generatorLimits,
  type NumericOption,
  styledCharacters,
} from "./options.ts";

/** Rows of history shown before the owner asks for more. */
const historyPage = 100;

type GeneratorApi = Pick<
  VaultApi,
  | "generatorState"
  | "generatePassword"
  | "clearGeneratorHistory"
  | "copyGeneratedPassword"
>;

const kindLabels: Record<GeneratorKind, MessageKey> = {
  password: "generator.place.kind.password",
  passphrase: "generator.place.kind.passphrase",
};

/**
 * GeneratorView makes passwords and passphrases and lists every one made. Generated values live only in this view's
 * state, never in the query cache, so they go with the view.
 */
export function GeneratorView({ api }: { api: GeneratorApi }) {
  const { t, failure, language } = useTranslator();
  const [options, setOptions] = useState<GeneratorOptions | null>(null);
  const [current, setCurrent] = useState<GeneratedPassword | null>(null);
  const [history, setHistory] = useState<GeneratedPassword[]>([]);
  const [shown, setShown] = useState(historyPage);
  const [working, setWorking] = useState(false);
  const [clearing, setClearing] = useState(false);
  // The slider's value while it is dragged; a value is made once it is let go.
  const [draft, setDraft] = useState<{
    option: NumericOption;
    value: number;
  } | null>(null);

  function report(cause: unknown, message: MessageKey) {
    toast.error(failure(cause, message));
  }

  async function generate(next: GeneratorOptions) {
    setOptions(next);
    setWorking(true);
    try {
      const entry = await api.generatePassword(next);
      setCurrent(entry);
      setHistory((earlier) => [entry, ...earlier]);
    } catch (cause) {
      report(cause, "generator.place.error.generate");
    } finally {
      setWorking(false);
    }
  }

  // biome-ignore lint/correctness/useExhaustiveDependencies: the generator opens once per view.
  useEffect(() => {
    let live = true;
    api
      .generatorState()
      .then((state) => {
        if (!live) return;
        setHistory(state.history);
        void generate(state.options);
      })
      .catch((cause: unknown) => {
        if (live) report(cause, "generator.place.error.load");
      });
    return () => {
      live = false;
    };
  }, [api]);

  function change(patch: Partial<GeneratorOptions>) {
    if (!options) return;
    const next = { ...options, ...patch };
    // Turning a set on can ask for more guaranteed characters than the length holds.
    void generate(clampOption(next, "minNumbers", next.minNumbers));
  }

  function changeNumber(option: NumericOption, value: number) {
    if (!options || Number.isNaN(value)) return;
    const next = clampOption(options, option, value);
    if (next[option] === options[option]) return;
    void generate(next);
  }

  function copy(value: string) {
    api
      .copyGeneratedPassword(value)
      .then(() => toast.success(t("generator.place.copied")))
      .catch((cause: unknown) => report(cause, "generator.place.error.copy"));
  }

  async function clearHistory() {
    setWorking(true);
    try {
      await api.clearGeneratorHistory();
      setHistory([]);
      setShown(historyPage);
      toast.success(t("generator.place.history.cleared"));
    } catch (cause) {
      report(cause, "generator.place.error.clear");
    } finally {
      setWorking(false);
      setClearing(false);
    }
  }

  const when = new Intl.DateTimeFormat(language, {
    dateStyle: "medium",
    timeStyle: "short",
  });
  const numberOf = (option: NumericOption) =>
    draft?.option === option ? draft.value : (options?.[option] ?? 0);

  function numberRow(option: NumericOption, title: MessageKey) {
    const limits = generatorLimits[option];
    const id = `generator-${option}`;
    return (
      <BlockRow title={t(title)} htmlFor={id}>
        <Slider
          aria-label={t(title)}
          className="w-36"
          min={limits.min}
          max={limits.max}
          step={1}
          value={[numberOf(option)]}
          disabled={!options}
          onValueChange={([value]) => {
            if (value !== undefined) setDraft({ option, value });
          }}
          onValueCommit={([value]) => {
            setDraft(null);
            if (value !== undefined) changeNumber(option, value);
          }}
        />
        <Input
          id={id}
          type="number"
          inputMode="numeric"
          className="h-7 w-16 text-xs"
          min={limits.min}
          max={limits.max}
          value={numberOf(option)}
          disabled={!options}
          onChange={(event) =>
            changeNumber(option, event.currentTarget.valueAsNumber)
          }
        />
      </BlockRow>
    );
  }

  function switchRow(
    option:
      | "uppercase"
      | "lowercase"
      | "numbers"
      | "symbols"
      | "avoidAmbiguous"
      | "capitalize"
      | "includeNumber",
    title: MessageKey,
    detail?: string,
  ) {
    const id = `generator-${option}`;
    const sets = ["uppercase", "lowercase", "numbers", "symbols"] as const;
    // The last character set left on cannot be turned off.
    const lastSet =
      options !== null &&
      (sets as readonly string[]).includes(option) &&
      sets.filter((set) => options[set]).length === 1 &&
      options[option];
    return (
      <BlockRow title={t(title)} detail={detail} htmlFor={id}>
        <Switch
          id={id}
          checked={Boolean(options?.[option])}
          disabled={!options || lastSet}
          onCheckedChange={(checked) =>
            change(
              checked
                ? { [option]: true }
                : option === "numbers"
                  ? { numbers: false, minNumbers: 0 }
                  : option === "symbols"
                    ? { symbols: false, minSymbols: 0 }
                    : { [option]: false },
            )
          }
        />
      </BlockRow>
    );
  }

  return (
    <ScrollArea className="min-h-0 flex-1">
      <WrapBlockText>
        <div className="max-w-[560px] p-5 max-sm:px-4">
          <h2 className="mb-[21px] text-[17px] font-medium leading-[1.35]">
            {t("generator.place.heading")}
          </h2>

          <ToggleGroup
            type="single"
            variant="outline"
            className="mb-4 w-full"
            value={options?.kind ?? "password"}
            onValueChange={(value) => {
              if (value === "password" || value === "passphrase") {
                change({ kind: value });
              }
            }}
            aria-label={t("generator.place.kind.label")}
          >
            {(["password", "passphrase"] as const).map((kind) => (
              <ToggleGroupItem
                key={kind}
                value={kind}
                className="flex-1 text-xs"
                disabled={!options}
              >
                {t(kindLabels[kind])}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>

          <Block className="mb-6">
            <div className="flex min-h-[72px] items-center gap-2 px-[13px] py-3">
              <output
                aria-live="polite"
                aria-label={t("generator.place.value.label")}
                className="min-w-0 flex-1 font-mono text-[15px] leading-relaxed break-all select-all"
              >
                {current
                  ? styledCharacters(current.value).map((part, index) => (
                      <span
                        // biome-ignore lint/suspicious/noArrayIndexKey: the parts of one value never reorder.
                        key={index}
                        className={
                          part.style === "number"
                            ? "text-(--generated-number)"
                            : part.style === "symbol"
                              ? "text-(--generated-symbol)"
                              : undefined
                        }
                      >
                        {part.text}
                      </span>
                    ))
                  : "…"}
              </output>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-label={t("generator.place.regenerate")}
                title={t("generator.place.regenerate")}
                disabled={!options || working}
                onClick={() => options && void generate(options)}
              >
                <RefreshCw />
              </Button>
              <Button
                type="button"
                size="sm"
                disabled={!current}
                onClick={() => current && copy(current.value)}
              >
                <Copy />
                {t("generator.place.copy")}
              </Button>
            </div>
          </Block>

          <BlockHeading>{t("generator.place.options")}</BlockHeading>
          {options?.kind === "passphrase" ? (
            <Block className="mb-6">
              {numberRow("words", "generator.place.words")}
              <BlockRow
                title={t("generator.place.separator")}
                htmlFor="generator-separator"
              >
                <Input
                  id="generator-separator"
                  className="h-7 w-16 text-center font-mono text-xs"
                  maxLength={1}
                  value={options.separator}
                  onChange={(event) =>
                    change({ separator: event.currentTarget.value.slice(-1) })
                  }
                />
              </BlockRow>
              {switchRow("capitalize", "generator.place.capitalize")}
              {switchRow("includeNumber", "generator.place.include-number")}
            </Block>
          ) : (
            <Block className="mb-6">
              {numberRow("length", "generator.place.length")}
              {switchRow("uppercase", "generator.place.uppercase", "A-Z")}
              {switchRow("lowercase", "generator.place.lowercase", "a-z")}
              {switchRow("numbers", "generator.place.numbers", "0-9")}
              {switchRow("symbols", "generator.place.symbols", "!@#$%^&*")}
              {options?.numbers &&
                numberRow("minNumbers", "generator.place.min-numbers")}
              {options?.symbols &&
                numberRow("minSymbols", "generator.place.min-symbols")}
              {switchRow(
                "avoidAmbiguous",
                "generator.place.avoid-ambiguous",
                t("generator.place.avoid-ambiguous.detail"),
              )}
            </Block>
          )}

          <div className="flex items-center justify-between gap-3">
            <BlockHeading>
              {t("generator.place.history", { count: history.length })}
            </BlockHeading>
            {history.length > 0 && (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="-mt-1 text-xs text-muted-foreground"
                disabled={working}
                onClick={() => setClearing(true)}
              >
                <Trash2 />
                {t("generator.place.history.clear")}
              </Button>
            )}
          </div>
          {history.length === 0 ? (
            <BlockNote>{t("generator.place.history.empty")}</BlockNote>
          ) : (
            <>
              <Block>
                <ul aria-label={t("generator.place.history.label")}>
                  {history.slice(0, shown).map((entry) => (
                    <li
                      key={`${entry.at}-${entry.value}`}
                      className="flex min-h-11 items-center gap-2.5 border-b px-[13px] py-1.5 last:border-b-0"
                    >
                      <span className="min-w-0 flex-1">
                        <span className="block font-mono text-[13px] break-all">
                          {entry.value}
                        </span>
                        <span className="block text-[11px] text-muted-foreground">
                          {t(kindLabels[entry.kind])} ·{" "}
                          {when.format(new Date(entry.at))}
                        </span>
                      </span>
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="size-7"
                        aria-label={t("generator.place.copy")}
                        title={t("generator.place.copy")}
                        onClick={() => copy(entry.value)}
                      >
                        <Copy />
                      </Button>
                    </li>
                  ))}
                </ul>
              </Block>
              {history.length > shown && (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="mt-2 w-full text-xs"
                  onClick={() => setShown((count) => count + historyPage)}
                >
                  {t("generator.place.history.more", {
                    count: history.length - shown,
                  })}
                </Button>
              )}
              <BlockNote>{t("generator.place.history.note")}</BlockNote>
            </>
          )}
        </div>
      </WrapBlockText>
      <ConfirmDialog
        open={clearing}
        title={t("generator.place.history.clear.title")}
        detail={t("generator.place.history.clear.detail", {
          count: history.length,
        })}
        confirm={t("generator.place.history.clear.confirm")}
        cancel={t("generator.place.history.clear.cancel")}
        destructive
        busy={working}
        onConfirm={() => void clearHistory()}
        onCancel={() => setClearing(false)}
      />
    </ScrollArea>
  );
}
