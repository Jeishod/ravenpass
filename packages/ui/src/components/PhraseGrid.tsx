import { cn } from "cn";
import { motion } from "motion/react";
import type { ReactNode } from "react";
import type { Presence } from "../motion/timings.ts";
import { phraseColumns } from "../seeds/phrase.ts";

/** PhraseGrid lays a phrase out in numbered cells in reading order. */
export function PhraseGrid({
  label,
  count,
  className,
  children,
}: {
  label: string;
  /** How many words the grid holds. */
  count: number;
  className?: string;
  children: ReactNode;
}) {
  return (
    <ol
      aria-label={label}
      className={cn(
        "grid gap-1",
        phraseColumns(count) === 4
          ? "grid-cols-4 max-sm:grid-cols-3"
          : "grid-cols-3",
        className,
      )}
    >
      {children}
    </ol>
  );
}

/** PhraseCell is one word of a PhraseGrid, after the number of its place in the phrase. */
export function PhraseCell({
  position,
  presence,
  className,
  children,
}: {
  /** The word's place in the phrase, counting from one. */
  position: number;
  /** How the cell arrives; without it the cell is simply there. */
  presence?: Presence;
  className?: string;
  children: ReactNode;
}) {
  return (
    <motion.li
      initial={presence?.initial}
      animate={presence?.animate}
      className={cn(
        "flex min-w-0 items-baseline gap-2 rounded-[10px] bg-field px-2.5 py-1.5",
        className,
      )}
    >
      <span className="w-4 shrink-0 text-right font-mono text-[11px] text-faint tabular-nums">
        {position}
      </span>
      {children}
    </motion.li>
  );
}
