import { cn } from "cn";
import { Copy } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useCallback, useEffect, useState } from "react";
import {
  type CodeTimeLeft,
  closing,
  groupedCode,
  periodLeft,
  timeLeft,
} from "../../credentials/one-time-code.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import { roll } from "../../motion/timings.ts";
import type { OneTimeCode as Code } from "../../vault-api.ts";
import { CodeRenewal } from "../../workspace/code-renewal.ts";
import { ProgressRing } from "./ProgressRing.tsx";

const digitSizes = {
  detail: "text-[19px] leading-tight",
  row: "text-[13px]",
};

/** `code` is undefined until the first one arrives and null when none could be had. */
interface CurrentCode {
  readonly code: Code | null | undefined;
  readonly left: CodeTimeLeft;
}

/** useClock is the time in Unix milliseconds, read every second and on `renew`. */
function useClock(): { now: number; renew: () => void } {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    const ticker = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(ticker);
  }, []);

  const renew = useCallback(() => setNow(Date.now()), []);
  return { now, renew };
}

/** usePeriodLeft counts down a `period`-second code from the clock alone. */
export function usePeriodLeft(period: number): CodeTimeLeft {
  return periodLeft(period, useClock().now);
}

/** useCurrentCode asks `generate` for a code, renews it on expiry, and asks at once for a new `generate`. */
function useCurrentCode(generate: () => Promise<Code>): CurrentCode {
  const [code, setCode] = useState<Code | null | undefined>(undefined);
  const { now, renew } = useClock();

  useEffect(() => {
    const renewal = new CodeRenewal(generate, (next) => {
      setCode(next);
      if (next) renew();
    });
    renewal.start();
    return () => renewal.stop();
  }, [generate, renew]);

  return { code, left: code ? timeLeft(code, now) : { seconds: 0, share: 0 } };
}

/** CodeTimer rings the time a code has left; `left` is null while there is none to count. */
export function CodeTimer({
  left,
  warn = false,
}: {
  left: CodeTimeLeft | null;
  warn?: boolean;
}) {
  return (
    <ProgressRing
      share={left?.share ?? 0}
      tone={warn && left && closing(left) ? "warning" : "default"}
    >
      <span
        role="timer"
        className="text-[11px] text-muted-foreground tabular-nums"
      >
        {left ? left.seconds : ""}
      </span>
    </ProgressRing>
  );
}

/** CodeDigits prints a grouped code and rolls to the next one when it changes. */
export function CodeDigits({
  code,
  size,
}: {
  code: string;
  size: keyof typeof digitSizes;
}) {
  return (
    <span className="relative block">
      <AnimatePresence mode="popLayout" initial={false}>
        <motion.span
          key={code}
          initial={roll.initial}
          animate={roll.animate}
          exit={roll.exit}
          className={cn(
            "block truncate font-mono tracking-[0.18em] tabular-nums",
            digitSizes[size],
          )}
        >
          {groupedCode(code)}
        </motion.span>
      </AnimatePresence>
    </span>
  );
}

/** OneTimeCode shows a credential's current code and copies it on click. */
export function OneTimeCode({
  setup,
  generate,
  onCopy,
  busy,
}: {
  setup: string;
  generate: (setup: string) => Promise<Code>;
  onCopy: () => void;
  busy: boolean;
}) {
  const { t } = useTranslator();
  const generateCode = useCallback(() => generate(setup), [generate, setup]);
  const current = useCurrentCode(generateCode);
  const { code } = current;

  return (
    <button
      type="button"
      aria-label={t("credential.copy.totp")}
      title={t("credential.copy.totp")}
      disabled={busy || !code}
      onClick={onCopy}
      className="flex shrink-0 items-center gap-3 rounded-row bg-field px-[13px] py-[11px] text-left outline-none hover:bg-field-hover focus-visible:bg-field-hover disabled:pointer-events-none disabled:opacity-50"
    >
      <CodeTimer left={code ? current.left : null} />
      <span className="min-w-0">
        <span className="block text-[11px] text-muted-foreground">
          {t("credential.field.totp")}
        </span>
        {code ? (
          <CodeDigits code={code.code} size="detail" />
        ) : (
          <span className="block truncate text-[13px] text-muted-foreground">
            {code === null ? t("credential.totp.unavailable") : ""}
          </span>
        )}
      </span>
      <Copy className="ml-auto size-4 shrink-0 text-muted-foreground" />
    </button>
  );
}
