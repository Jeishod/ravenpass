import { Countdown } from "../extensions/countdown.ts";
import type { OneTimeCode } from "../vault-api.ts";

/** `share` is the part of the period left, from 1 down to 0. */
export interface CodeTimeLeft {
  readonly seconds: number;
  readonly share: number;
}

const closingSeconds = 5;

export function groupedCode(code: string): string {
  const half = Math.ceil(code.length / 2);
  return `${code.slice(0, half)} ${code.slice(half)}`;
}

/** One dot per digit, which `groupedCode` groups as it groups digits. */
export function maskedCode(digits: number): string {
  return "•".repeat(digits);
}

/** Periods run from the Unix epoch (RFC 6238), so a period's end is known without the code. */
export function periodEnd(period: number, now: number): number {
  return (Math.floor(Math.floor(now / 1000) / period) + 1) * period * 1000;
}

export function periodLeft(period: number, now: number): CodeTimeLeft {
  return timeLeft({ period, expiresAt: periodEnd(period, now) }, now);
}

/** A closing code is about to be refused, so its ring warns and a fill waits for the next one. */
export function closing(left: CodeTimeLeft): boolean {
  return left.seconds <= closingSeconds;
}

/** A code without a period has no share. */
export function timeLeft(
  code: Pick<OneTimeCode, "period" | "expiresAt">,
  now: number,
): CodeTimeLeft {
  const seconds = new Countdown(code.expiresAt).secondsLeft(now);
  return {
    seconds,
    share: code.period ? Math.min(seconds / code.period, 1) : 0,
  };
}
