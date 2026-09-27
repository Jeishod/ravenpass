import { cn } from "cn";
import { LayoutGroup, motion } from "motion/react";
import type { ReactNode } from "react";
import { fadeAway, glide } from "./timings.ts";

/** The class a choice's content carries to paint above the indicator. */
export const aboveIndicator = "relative z-[2]";

/** SelectionGroup scopes one switch, so its indicator travels only between its own choices. */
export function SelectionGroup({
  id,
  children,
}: {
  id: string;
  children: ReactNode;
}) {
  return <LayoutGroup id={id}>{children}</LayoutGroup>;
}

/** SelectionIndicator travels between choices; render it only inside the chosen one, which must be `relative`. */
export function SelectionIndicator({
  appear = false,
  className,
}: {
  /** Whether it grows into place, for a choice it did not travel from. */
  appear?: boolean;
  className?: string;
}) {
  return (
    <motion.span
      layoutId="selection"
      aria-hidden="true"
      initial={appear ? { opacity: 0, scale: 0.8 } : false}
      animate={{ opacity: 1, scale: 1 }}
      exit={fadeAway}
      transition={glide}
      className={cn("pointer-events-none absolute inset-0 z-[1]", className)}
    />
  );
}
