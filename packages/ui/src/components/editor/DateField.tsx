import { maskitoDate } from "@maskito/kit";
import { useMaskito } from "@maskito/react";
import { cn } from "cn";
import { CalendarDays } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useTranslator } from "../../i18n/translator.tsx";
import {
  CalendarDate,
  DateEntryFormat,
  dateLocale,
  earliestYear,
} from "../../identities/dates.ts";
import { Button } from "../ui/button.tsx";
import { Calendar } from "../ui/calendar.tsx";
import { Input } from "../ui/input.tsx";
import { Popover, PopoverContent, PopoverTrigger } from "../ui/popover.tsx";
import { bareField } from "./EditorFields.tsx";

/** How far past today the calendar offers years for a date that lies ahead, such as an expiry. */
const yearsAhead = 30;

/** Which way from today a date may lie. */
export type DateSpan = "past" | "any";

/** DateField edits a stored `YYYY-MM-DD` date, typed in the interface language's order or picked. */
export function DateField({
  id,
  value,
  span,
  disabled,
  onChange,
  onValidity,
}: {
  id: string;
  value: string;
  span: DateSpan;
  disabled: boolean;
  onChange: (value: string) => void;
  /** False while the typed text is not a whole date; true once it is or the field unmounts. */
  onValidity: (id: string, complete: boolean) => void;
}) {
  const { t, language } = useTranslator();
  const entry = useMemo(() => DateEntryFormat.for(language), [language]);
  const [typed, setTyped] = useState(() => entry.display(value));
  const [typedIn, setTypedIn] = useState(language);
  const [open, setOpen] = useState(false);

  // A language change rewrites the date in the new order.
  if (typedIn !== language) {
    setTypedIn(language);
    setTyped(entry.display(value));
  }

  const today = useMemo(() => CalendarDate.of(new Date()).day, []);
  const mask = useMemo(
    () =>
      maskitoDate({
        mode: entry.mode,
        separator: entry.separator,
        min: new Date(earliestYear, 0, 1),
        max: span === "past" ? today : undefined,
      }),
    [entry, span, today],
  );
  const maskRef = useMaskito({ options: mask });
  const complete = entry.read(typed) !== null;

  useEffect(() => {
    onValidity(id, complete);
  }, [id, complete, onValidity]);

  useEffect(() => () => onValidity(id, true), [id, onValidity]);

  function type(text: string) {
    setTyped(text);
    const stored = entry.read(text);
    if (stored !== null) onChange(stored);
  }

  function pick(day: Date | undefined) {
    if (!day) return;
    const stored = CalendarDate.of(day).stored();
    setTyped(entry.display(stored));
    onChange(stored);
    setOpen(false);
  }

  const selected = CalendarDate.parse(value)?.day;

  return (
    <>
      <Input
        id={id}
        ref={maskRef}
        className={cn(bareField, "tabular-nums aria-invalid:text-destructive")}
        value={typed}
        onInput={(event) => type(event.currentTarget.value)}
        placeholder={t("workspace.date.placeholder")}
        inputMode="numeric"
        autoComplete="off"
        aria-invalid={complete ? undefined : true}
        disabled={disabled}
      />
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            className="size-7 shrink-0 rounded-md text-muted-foreground hover:text-foreground"
            aria-label={t("workspace.date.choose")}
            title={t("workspace.date.choose")}
            disabled={disabled}
          >
            <CalendarDays className="size-4" />
          </Button>
        </PopoverTrigger>
        <PopoverContent align="end" className="w-auto p-0">
          <Calendar
            mode="single"
            selected={selected}
            defaultMonth={selected}
            onSelect={pick}
            captionLayout="dropdown"
            startMonth={new Date(earliestYear, 0)}
            endMonth={
              span === "past"
                ? today
                : new Date(today.getFullYear() + yearsAhead, 11)
            }
            disabled={span === "past" ? { after: today } : undefined}
            locale={dateLocale(language)}
            formatters={{
              formatMonthDropdown: (month) =>
                month.toLocaleString(language, { month: "short" }),
            }}
          />
        </PopoverContent>
      </Popover>
    </>
  );
}
