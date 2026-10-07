import { useQuery } from "@tanstack/react-query";
import { cn } from "cn";
import { RefreshCw } from "lucide-react";
import { type ReactNode, useId, useMemo, useState } from "react";
import {
  characterRuns,
  fitGeneratorOptions,
  type GeneratedPassword,
  type GeneratorMode,
  type GeneratorOptions,
  generatorBounds,
  PasswordGenerator,
  readGeneratorOptions,
  type Strength,
  type WordSeparator,
  wordSeparators,
} from "../../credentials/password-generator.ts";
import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import { queryKeys } from "../../query/keys.ts";
import type { VaultApi } from "../../vault-api.ts";
import { quietStorage } from "../../workspace/browser-storage.ts";
import { blockControl } from "../Block.tsx";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../ui/select.tsx";
import { Slider } from "../ui/slider.tsx";
import { Switch } from "../ui/switch.tsx";
import { RowButton } from "../workspace/Fields.tsx";
import { Segmented } from "../workspace/Segmented.tsx";
import { type Tone, toneText } from "../workspace/tones.ts";

/** Storage key of the generator's options on the device; they hold no password. */
const optionsKey = "ravenpass.generator";

const strengths: Record<Strength, { label: MessageKey; tone: Tone }> = {
  weak: { label: "generator.strength.weak", tone: "destructive" },
  fair: { label: "generator.strength.fair", tone: "warning" },
  strong: { label: "generator.strength.strong", tone: "default" },
  "very-strong": { label: "generator.strength.very-strong", tone: "default" },
};

const separatorNames: Record<WordSeparator, MessageKey> = {
  "-": "generator.separator.hyphen",
  ".": "generator.separator.dot",
  _: "generator.separator.underscore",
  " ": "generator.separator.space",
};

/** usePasswordGenerator loads the word list once `enabled` and returns the generator over it. */
export function usePasswordGenerator(
  host: Pick<VaultApi, "seedWordlist">,
  enabled = true,
): PasswordGenerator | null {
  const wordlist = useQuery({
    queryKey: queryKeys.seedWordlist,
    queryFn: () => host.seedWordlist(),
    enabled,
    meta: { failure: "generator.error.words" },
  }).data;
  return useMemo(
    () => (wordlist ? new PasswordGenerator(wordlist) : null),
    [wordlist],
  );
}

/**
 * GeneratorForm shows a new password, its strength and its options, kept on the device; `actions` acts on the
 * password shown.
 */
export function GeneratorForm({
  generator,
  actions,
}: {
  generator: PasswordGenerator;
  actions: (generated: GeneratedPassword) => ReactNode;
}) {
  const { t } = useTranslator();
  const id = useId();
  const [options, setOptions] = useState(() =>
    readGeneratorOptions(quietStorage.getItem(optionsKey)),
  );
  // A new draw for each change of options, and on request.
  const [generated, setGenerated] = useState<GeneratedPassword>(() =>
    generator.generate(options),
  );
  const strength = strengths[PasswordGenerator.strength(generated.bits)];

  function change(next: Partial<GeneratorOptions>) {
    const changed = { ...options, ...next };
    setOptions(changed);
    setGenerated(generator.generate(changed));
    quietStorage.setItem(optionsKey, JSON.stringify(changed));
  }

  function changeCount(
    option: "length" | "minDigits" | "minSymbols",
    value: number,
  ) {
    change(fitGeneratorOptions({ ...options, [option]: value }, option));
  }

  return (
    <div className="grid min-w-0 gap-4">
      <div className="flex items-start gap-2 rounded-row bg-field px-[13px] py-3">
        <output
          className="min-w-0 flex-1 font-mono text-[15px] leading-[1.45] break-all"
          aria-label={t("generator.value")}
          aria-live="polite"
        >
          {characterRuns(generated.password).map((run, index) => (
            <span
              // biome-ignore lint/suspicious/noArrayIndexKey: the runs of one password never reorder.
              key={index}
              className={cn(
                run.kind === "digit" && "text-(--generated-digit)",
                run.kind === "symbol" && "text-(--generated-symbol)",
              )}
            >
              {run.text}
            </span>
          ))}
        </output>
        <RowButton
          label={t("generator.again")}
          icon={RefreshCw}
          busy={false}
          onClick={() => setGenerated(generator.generate(options))}
        />
      </div>
      <p className={cn("-mt-2 px-[3px] text-xs", toneText[strength.tone])}>
        {t("generator.strength", {
          strength: t(strength.label),
          bits: Math.floor(generated.bits),
        })}
      </p>

      <Segmented<GeneratorMode>
        id={`${id}-mode`}
        legend={t("generator.mode")}
        options={[
          { value: "words", label: t("generator.mode.words") },
          { value: "characters", label: t("generator.mode.characters") },
        ]}
        value={options.mode}
        stretch
        onChange={(mode) => change({ mode })}
      />

      {options.mode === "words" ? (
        <div className="grid gap-3">
          <Count
            label={t("generator.words", { count: options.words })}
            value={options.words}
            bounds={generatorBounds.words}
            onChange={(words) => change({ words })}
          />
          <OptionRow label={t("generator.separator")} htmlFor={`${id}-sep`}>
            <Select
              value={options.separator}
              onValueChange={(next) => {
                const separator = wordSeparators.find((item) => item === next);
                if (separator) change({ separator });
              }}
            >
              <SelectTrigger
                id={`${id}-sep`}
                size="sm"
                className={blockControl}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {wordSeparators.map((separator) => (
                    <SelectItem key={separator} value={separator}>
                      {t(separatorNames[separator])}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </OptionRow>
          <Toggle
            id={`${id}-capitalize`}
            label={t("generator.capitalize")}
            checked={options.capitalize}
            onChange={(capitalize) => change({ capitalize })}
          />
          <Toggle
            id={`${id}-digit`}
            label={t("generator.digit")}
            checked={options.digit}
            onChange={(digit) => change({ digit })}
          />
        </div>
      ) : (
        <div className="grid gap-3">
          <Count
            label={t("generator.length", { count: options.length })}
            value={options.length}
            bounds={generatorBounds.length}
            onChange={(length) => changeCount("length", length)}
          />
          <Toggle
            id={`${id}-uppercase`}
            label={t("generator.uppercase")}
            checked={options.uppercase}
            onChange={(uppercase) => change({ uppercase })}
          />
          <Toggle
            id={`${id}-digits`}
            label={t("generator.digits")}
            checked={options.digits}
            onChange={(digits) => change({ digits })}
          />
          {options.digits && (
            <Count
              label={t("generator.digits.minimum", {
                count: options.minDigits,
              })}
              value={options.minDigits}
              bounds={generatorBounds.minimum}
              onChange={(minDigits) => changeCount("minDigits", minDigits)}
            />
          )}
          <Toggle
            id={`${id}-symbols`}
            label={t("generator.symbols")}
            checked={options.symbols}
            onChange={(symbols) => change({ symbols })}
          />
          {options.symbols && (
            <Count
              label={t("generator.symbols.minimum", {
                count: options.minSymbols,
              })}
              value={options.minSymbols}
              bounds={generatorBounds.minimum}
              onChange={(minSymbols) => changeCount("minSymbols", minSymbols)}
            />
          )}
          <Toggle
            id={`${id}-ambiguous`}
            label={t("generator.avoid-ambiguous")}
            detail={t("generator.avoid-ambiguous.detail")}
            checked={options.avoidAmbiguous}
            onChange={(avoidAmbiguous) => change({ avoidAmbiguous })}
          />
        </div>
      )}

      {actions(generated)}
    </div>
  );
}

function OptionRow({
  label,
  detail,
  htmlFor,
  children,
}: {
  label: string;
  detail?: string;
  htmlFor: string;
  children: ReactNode;
}) {
  return (
    <div className="flex min-h-7 items-center justify-between gap-3">
      <span className="grid gap-0.5">
        <label htmlFor={htmlFor} className="text-[13px]">
          {label}
        </label>
        {detail && (
          <span className="text-[11px] text-muted-foreground">{detail}</span>
        )}
      </span>
      {children}
    </div>
  );
}

function Toggle({
  id,
  label,
  detail,
  checked,
  onChange,
}: {
  id: string;
  label: string;
  detail?: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <OptionRow label={label} detail={detail} htmlFor={id}>
      <Switch id={id} checked={checked} onCheckedChange={onChange} />
    </OptionRow>
  );
}

function Count({
  label,
  value,
  bounds,
  onChange,
}: {
  label: string;
  value: number;
  bounds: { min: number; max: number };
  onChange: (value: number) => void;
}) {
  return (
    <div className="grid gap-2">
      <span className="text-[13px]">{label}</span>
      <Slider
        value={[value]}
        min={bounds.min}
        max={bounds.max}
        step={1}
        aria-label={label}
        onValueChange={([next]) => {
          if (next !== undefined) onChange(next);
        }}
      />
    </div>
  );
}
