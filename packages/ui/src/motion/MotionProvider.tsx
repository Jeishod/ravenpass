import { MotionConfig } from "motion/react";
import type { ReactNode } from "react";

/** Under reduced motion, Motion keeps opacity and drops every transform and layout animation. */
export function MotionProvider({ children }: { children: ReactNode }) {
  return <MotionConfig reducedMotion="user">{children}</MotionConfig>;
}
