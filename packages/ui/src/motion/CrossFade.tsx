import { AnimatePresence, motion, PresenceContext } from "motion/react";
import type { ReactNode } from "react";
import { type Presence, settle } from "./timings.ts";

/** CrossFade fades to new content when `id` changes, once the old content's own fade ends. */
export function CrossFade({
  id,
  presence = settle,
  appear = false,
  className,
  children,
}: {
  /** What the content is; a new value replaces it. */
  id: string;
  presence?: Presence;
  /** Whether the first content animates in as well. */
  appear?: boolean;
  className?: string;
  children: ReactNode;
}) {
  return (
    <AnimatePresence mode="wait" initial={appear}>
      <motion.div
        key={id}
        initial={presence.initial}
        animate={presence.animate}
        exit={presence.exit}
        className={className}
      >
        {/* Motion 13.4 holds a leaving child forever when a motion element inside it unmounts first. */}
        <PresenceContext.Provider value={null}>
          {children}
        </PresenceContext.Provider>
      </motion.div>
    </AnimatePresence>
  );
}
