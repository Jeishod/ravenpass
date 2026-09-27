import { animate } from "motion";

/** The longest a filled value takes to appear, in seconds, and the time each character adds. */
const longest = 0.8;
const perCharacter = 0.035;

/** typeIn writes a value a few characters at a time and resolves once it is whole; `instant` skips the animation. */
export function typeIn(
  value: string,
  write: (shown: string) => void,
  instant: boolean,
): Promise<void> {
  const characters = Array.from(value);
  if (instant || characters.length < 2) {
    write(value);
    return Promise.resolve();
  }
  return new Promise((resolve) => {
    animate(0, characters.length, {
      duration: Math.min(longest, perCharacter * characters.length),
      ease: "easeOut",
      onUpdate: (count) =>
        write(characters.slice(0, Math.round(count)).join("")),
      onComplete: () => {
        write(value);
        resolve();
      },
    });
  });
}
