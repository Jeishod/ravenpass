import { cn } from "cn";
import type { ComponentType, ReactNode } from "react";
import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import type { Validity } from "../../identities/dates.ts";
import { actionRow } from "./Fields.tsx";
import { ProgressRing } from "./ProgressRing.tsx";
import { toneText, validityTone } from "./tones.ts";

/** How a heading states its item's expiry, in the words of that kind of item. */
export interface ValidityMessages {
  /** Takes `{date}` and `{left}`. */
  expires: MessageKey;
  /** Takes `{date}`. */
  expired: MessageKey;
  none: MessageKey;
}

/** What the whole heading does when selected, as a field row does. */
export interface HeadingAction {
  label: string;
  icon: ComponentType<{ className?: string }>;
  disabled: boolean;
  run: () => void;
}

const heading =
  "flex min-h-[54px] w-full items-center gap-3 border-b px-[13px] py-[11px] text-left last:border-b-0";

/** ValidityHeading states an item's expiry; with `action` the whole heading is that button, named by what it shows. */
export function ValidityHeading({
  validity,
  today,
  title,
  messages,
  children,
  action,
}: {
  validity: Validity | null;
  today: Date;
  title: string;
  messages: ValidityMessages;
  /** Further lines under the expiry. */
  children?: ReactNode;
  action?: HeadingAction;
}) {
  const { t, language } = useTranslator();
  const tone = validityTone(validity, today);

  function expiry(): string {
    if (!validity) return t(messages.none);
    const date = validity.expires.format(language);
    if (tone === "destructive") return t(messages.expired, { date });
    return t(messages.expires, {
      date,
      left: validity.timeLeftInWords(today, language),
    });
  }

  const content = (
    <>
      {validity && (
        <ProgressRing share={validity.share(today)} tone={tone}>
          <span
            className={cn(
              "text-[10px] tabular-nums",
              tone === "default" && "text-muted-foreground",
            )}
            aria-hidden="true"
          >
            {validity.timeLeft(today, language)}
          </span>
        </ProgressRing>
      )}
      <span className="min-w-0 flex-1">
        <span className="block truncate text-[13px]">{title}</span>
        <span className={cn("block truncate text-[11px]", toneText[tone])}>
          {expiry()}
        </span>
        {children}
      </span>
      {action && (
        <action.icon className="size-4 shrink-0 text-muted-foreground" />
      )}
    </>
  );

  if (!action) return <div className={heading}>{content}</div>;

  return (
    <button
      type="button"
      title={action.label}
      disabled={action.disabled}
      onClick={action.run}
      className={`${heading} ${actionRow}`}
    >
      {content}
    </button>
  );
}
