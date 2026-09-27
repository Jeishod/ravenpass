import { cn } from "cn";
import { useId } from "react";

/** The parts of the drawn vault that animations move. */
export type VaultPart = "slot" | "halo" | "body" | "door" | "seam" | "handle";

/** vaultPart finds a part of the vault drawn inside root. */
export function vaultPart(root: ParentNode, part: VaultPart): HTMLElement {
  const found = root.querySelector<HTMLElement>(`[data-vault="${part}"]`);
  if (!found) throw new Error(`The vault has no ${part}.`);
  return found;
}

const opening = "absolute top-[34px] left-[29px] size-[106px] rounded-[26px]";

/** VaultArt draws the Ravenpass icon as a vault whose door swings on its left hinge. */
export function VaultArt({ className }: { className?: string }) {
  const keyFill = useId();

  return (
    <div
      data-vault="body"
      aria-hidden="true"
      className={cn(
        "size-44 rounded-[42px] border border-white/8 bg-linear-to-b from-[#25262b] to-[#111215] perspective-[700px]",
        className,
      )}
    >
      <div className={cn(opening, "border border-white/6 bg-[#030304]")} />
      <div
        data-vault="door"
        className={cn(
          opening,
          "origin-left bg-linear-to-b from-[#3b3c43] to-[#1f2025]",
        )}
      >
        <svg viewBox="22 25 70 70" aria-hidden="true" className="size-full">
          <defs>
            <linearGradient id={keyFill} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0" stopColor="#eeeff2" />
              <stop offset="1" stopColor="#a7a9b0" />
            </linearGradient>
          </defs>
          <g transform="rotate(-45 57 60)" fill={`url(#${keyFill})`}>
            <path
              fillRule="evenodd"
              d="M83 60a13 13 0 1 1-26 0a13 13 0 1 1 26 0ZM75 60a5 5 0 1 0-10 0a5 5 0 1 0 10 0Z"
            />
            <rect x="30" y="56.5" width="30" height="7" rx="2" />
            <rect x="34" y="62" width="6" height="9" rx="1.5" />
            <rect x="44" y="62" width="5" height="7" rx="1.5" />
          </g>
        </svg>
      </div>
      <div
        data-vault="seam"
        className={cn(opening, "border-[1.5px] border-foreground opacity-0")}
      />
      <div
        data-vault="handle"
        className="absolute top-[66px] right-[14px] h-11 w-[9px] rounded-full bg-[#44454c]"
      />
    </div>
  );
}

/** VaultMark is the settled vault; `staged` hides it for a timeline that brings it in. */
export function VaultMark({
  staged = false,
  className,
}: {
  staged?: boolean;
  className?: string;
}) {
  return (
    <div data-vault="slot" className={cn("relative size-23", className)}>
      <div
        data-vault="halo"
        aria-hidden="true"
        className={cn(
          "pointer-events-none absolute top-1/2 left-1/2 -mt-[210px] -ml-[210px] size-[420px] rounded-full bg-radial from-foreground/8 to-transparent to-60%",
          staged ? "opacity-0" : "opacity-50",
        )}
      />
      <VaultArt
        className={cn(
          "absolute top-1/2 left-1/2 -mt-22 -ml-22",
          staged ? "opacity-0" : "scale-[0.523]",
        )}
      />
    </div>
  );
}
