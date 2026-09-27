import { AnimatePresence, motion } from "motion/react";
import { type ReactNode, useState } from "react";
import { turn } from "./timings.ts";

/** StepTransition slides a later step in from the trailing side and an earlier one from the leading side. */
export function StepTransition({
  step,
  className,
  children,
}: {
  step: number;
  className?: string;
  children: ReactNode;
}) {
  const [shown, setShown] = useState({ step, direction: 1 });
  if (shown.step !== step) {
    setShown({ step, direction: step > shown.step ? 1 : -1 });
  }

  return (
    <AnimatePresence mode="wait" initial={false} custom={shown.direction}>
      <motion.div
        key={step}
        custom={shown.direction}
        variants={turn}
        initial="enter"
        animate="center"
        exit="leave"
        className={className}
      >
        {children}
      </motion.div>
    </AnimatePresence>
  );
}
