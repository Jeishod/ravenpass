import type { TargetAndTransition, Transition, Variants } from "motion/react";

/** An indicator travelling between choices. */
export const glide: Transition = {
  type: "spring",
  stiffness: 520,
  damping: 42,
};

const enterSeconds = 0.22;
const enterCurve = [0.22, 1, 0.36, 1] as const;
const leaveSeconds = 0.12;

/** Content arriving. */
export const enter: Transition = {
  duration: enterSeconds,
  ease: [...enterCurve],
};

/** Content leaving; shorter than arriving so a swap never waits on the old content. */
export const leave: Transition = { duration: leaveSeconds, ease: "easeIn" };

/** `enter` as the Web Animations API takes it, for what animates outside React. */
export const enterTiming = {
  duration: enterSeconds * 1000,
  easing: `cubic-bezier(${enterCurve.join(", ")})`,
} as const;

/** `leave` as the Web Animations API takes it, for what animates outside React. */
export const leaveTiming = {
  duration: leaveSeconds * 1000,
  easing: "ease-in",
} as const;

/** How swapped content comes and goes. */
export interface Presence {
  initial: TargetAndTransition;
  animate: TargetAndTransition;
  exit: TargetAndTransition;
}

// Leaving content stops taking pointer input at once, so a click lands on what is arriving.
const gone: TargetAndTransition = {
  opacity: 0,
  pointerEvents: "none",
  transition: leave,
};

/** A highlight the pointer left with nowhere to go: it fades where it stands. */
export const fadeAway: TargetAndTransition = {
  opacity: 0,
  transition: { duration: 0.2, ease: "easeOut" },
};

/** Content leaving a view it can come back to, such as a row scrolled away: it goes at once. */
export const vanish: TargetAndTransition = {
  opacity: 0,
  transition: { duration: 0 },
};

/** Content inside a pane: it rises a few pixels into place. */
export const settle: Presence = {
  initial: { opacity: 0, y: 6 },
  animate: { opacity: 1, y: 0, transition: enter },
  exit: gone,
};

/** A whole screen: it grows from just under its size. */
export const rise: Presence = {
  initial: { opacity: 0, scale: 0.985 },
  animate: { opacity: 1, scale: 1, transition: enter },
  exit: gone,
};

/** A walk's step change; the custom value is the direction, 1 forward and -1 back. */
export const turn: Variants = {
  enter: (direction: number) => ({
    opacity: 0,
    x: 18 * direction,
    filter: "blur(4px)",
  }),
  center: {
    opacity: 1,
    x: 0,
    filter: "blur(0px)",
    transition: { duration: 0.32, ease: [...enterCurve] },
  },
  leave: (direction: number) => ({
    opacity: 0,
    x: -12 * direction,
    filter: "blur(4px)",
    pointerEvents: "none",
    transition: leave,
  }),
};

/** Pieces of one whole arriving in reading order, such as the words of a recovery key. */
export function cascade(order: number): Presence {
  return {
    initial: { opacity: 0, y: 6, filter: "blur(6px)" },
    animate: {
      opacity: 1,
      y: 0,
      filter: "blur(0px)",
      transition: {
        duration: 0.35,
        ease: [...enterCurve],
        delay: order * 0.025,
      },
    },
    exit: gone,
  };
}

/** A value replaced by its successor: the old one rolls up and out as the new one rolls in. */
export const roll: Presence = {
  initial: { opacity: 0, y: "45%" },
  animate: { opacity: 1, y: 0, transition: enter },
  exit: { opacity: 0, y: "-45%", transition: leave },
};

/** Rows arriving together follow one another, up to this many before they arrive at once. */
const staggeredRows = 8;

/** A list row coming into view, staggered by its place in the frame. */
export function rowPresence(order: number): Presence {
  return {
    initial: { opacity: 0, y: 4 },
    animate: {
      opacity: 1,
      y: 0,
      transition: { ...enter, delay: Math.min(order, staggeredRows) * 0.025 },
    },
    exit: gone,
  };
}
