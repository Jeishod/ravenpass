import { cn } from "cn";
import type { ReactNode } from "react";
import type { Tone } from "./tones.ts";

const ringRadius = 14.5;
const ringLength = 2 * Math.PI * ringRadius;

const tones: Record<Tone, string> = {
  default: "",
  warning: "text-warning",
  destructive: "text-destructive",
};

/** ProgressRing draws the share of time left as an arc around its children. */
export function ProgressRing({
  share,
  tone = "default",
  children,
}: {
  /** From 1, a full ring, down to 0. */
  share: number;
  tone?: Tone;
  children?: ReactNode;
}) {
  const left = Math.min(Math.max(share, 0), 1);

  return (
    <span className={cn("relative size-8 shrink-0", tones[tone])}>
      <svg viewBox="0 0 32 32" className="size-8 -rotate-90" aria-hidden="true">
        <circle
          cx="16"
          cy="16"
          r={ringRadius}
          fill="none"
          stroke="var(--tile)"
          strokeWidth="3"
        />
        <circle
          cx="16"
          cy="16"
          r={ringRadius}
          fill="none"
          stroke="currentColor"
          strokeWidth="3"
          strokeLinecap="round"
          strokeDasharray={ringLength}
          strokeDashoffset={ringLength * (1 - left)}
          className="transition-[stroke-dashoffset] duration-500 motion-reduce:transition-none"
        />
      </svg>
      <span className="absolute inset-0 grid place-items-center">
        {children}
      </span>
    </span>
  );
}
