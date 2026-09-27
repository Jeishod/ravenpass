import { cn } from "cn";
import { formatDelay } from "../../i18n/language.ts";
import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import { Block, BlockNote, BlockRow, blockControl } from "../Block.tsx";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../ui/select.tsx";
import { Switch } from "../ui/switch.tsx";

/** DelaySetting is a switch with the delay it applies after; a delay of zero applies at once. */
export function DelaySetting({
  id,
  setting,
  busy,
  onChange,
  title,
  detail,
  delay,
  note,
  className = "mt-3.5",
}: {
  id: string;
  setting: { enabled: boolean; seconds: number; offered: number[] } | null;
  busy: boolean;
  onChange: (enabled: boolean, seconds: number) => void;
  title: MessageKey;
  detail?: MessageKey;
  delay: MessageKey;
  note: MessageKey;
  className?: string;
}) {
  const { t, language } = useTranslator();
  const unknown = busy || !setting;

  return (
    <>
      <Block className={className}>
        <BlockRow
          title={t(title)}
          detail={detail ? t(detail) : undefined}
          htmlFor={id}
        >
          <Switch
            id={id}
            checked={Boolean(setting?.enabled)}
            disabled={unknown}
            onCheckedChange={(enabled) => {
              if (setting) onChange(enabled, setting.seconds);
            }}
          />
        </BlockRow>
        <BlockRow title={t(delay)} htmlFor={`${id}-delay`}>
          <Select
            value={setting ? String(setting.seconds) : undefined}
            disabled={unknown || !setting?.enabled}
            onValueChange={(next) => {
              if (setting) onChange(setting.enabled, Number(next));
            }}
          >
            <SelectTrigger
              id={`${id}-delay`}
              size="sm"
              className={cn(blockControl, "max-w-[170px]")}
              aria-label={t(delay)}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {setting?.offered.map((seconds) => (
                  <SelectItem key={seconds} value={String(seconds)}>
                    {seconds === 0
                      ? t("settings.delay.immediately")
                      : formatDelay(seconds, language, "long")}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </BlockRow>
      </Block>
      <BlockNote>{t(note)}</BlockNote>
    </>
  );
}
