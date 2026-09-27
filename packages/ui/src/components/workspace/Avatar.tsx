import { cn } from "cn";
import { LoaderCircle } from "lucide-react";
import {
  type ComponentType,
  type CSSProperties,
  useRef,
  useState,
} from "react";
import { photoSource } from "../../identities/photo.ts";
import { initial } from "../../workspace/initials.ts";
import type { SiteIconState } from "../../workspace/site-icons.ts";

const sizes = {
  compact: "size-6 text-[11px]",
  row: "size-[30px] text-[13px]",
  header: "size-[42px] text-[17px]",
};

const iconSizes = {
  compact: "size-3.5",
  row: "size-4",
  header: "size-5",
};

/** Avatar shows an item's photo, else its site logo, else its initial, else its kind's icon. */
export function Avatar({
  label,
  mark,
  photo = "",
  logo,
  icon: Icon,
  size,
  shape,
  emphasis,
}: {
  label: string;
  /** What the avatar shows in place of the name's letter, such as a count; empty shows the icon. */
  mark?: string;
  /** In base64, empty for none. */
  photo?: string;
  /** What is known of the item's logo, for a kind that has one. */
  logo?: SiteIconState;
  icon: ComponentType<{ className?: string }>;
  size: keyof typeof sizes;
  shape: "tile" | "circle";
  emphasis: "none" | "fill" | "selected";
}) {
  const source = photoSource(photo);
  const logoSource = logo?.kind === "icon" ? photoSource(logo.image) : null;
  const tint = logo?.kind === "icon" ? logo.tint : "";
  const [shownSource, setShownSource] = useState<string | null>(null);
  // Only a logo that differs from the one present at mount animates in.
  const firstSource = useRef(logoSource);
  const letter = mark ?? initial(label);
  const radius =
    shape === "circle"
      ? "rounded-full"
      : size === "compact"
        ? "rounded-[7px]"
        : "rounded-[10px]";

  if (source) {
    return (
      <img
        src={source}
        alt=""
        draggable={false}
        className={cn(
          sizes[size],
          radius,
          "shrink-0 object-cover",
          emphasis === "selected" &&
            "outline-2 -outline-offset-2 outline-foreground",
        )}
      />
    );
  }

  const covered = logoSource !== null && shownSource === logoSource;
  const animate = logoSource !== firstSource.current;
  const fade =
    animate && "transition-opacity duration-300 motion-reduce:transition-none";
  const entrance =
    animate &&
    "transition-[opacity,scale] duration-300 ease-[cubic-bezier(0.34,1.56,0.64,1)] motion-reduce:transition-none";

  return (
    <span
      className={cn(
        "relative isolate flex shrink-0 items-center justify-center overflow-hidden",
        sizes[size],
        radius,
        emphasis === "none" || covered ? "bg-tile" : "action-fill",
      )}
      aria-hidden="true"
    >
      <span
        className={cn(
          "absolute inset-0",
          fade,
          covered ? "opacity-100" : "opacity-0",
        )}
        style={washOf(tint)}
      />
      <span
        className={cn(
          "transition-[opacity,scale] duration-200 motion-reduce:transition-none",
          covered && "scale-50 opacity-0",
          logo?.kind === "pending" && "site-icon-pending",
        )}
      >
        {letter || <Icon className={iconSizes[size]} />}
      </span>
      {logo?.kind === "pending" && (
        <LoaderCircle className="site-icon-spinner absolute size-[58%]" />
      )}
      {logoSource && (
        <img
          key={logoSource}
          src={logoSource}
          alt=""
          draggable={false}
          onLoad={() => setShownSource(logoSource)}
          className={cn(
            "absolute size-[64%] rounded-[22%] object-contain drop-shadow-[0_1px_1.5px_rgb(0_0_0/0.3)]",
            entrance,
            covered ? "scale-100 opacity-100" : "scale-50 opacity-0",
          )}
        />
      )}
      <span
        className={cn(
          "pointer-events-none absolute inset-0 rounded-[inherit] inset-ring inset-ring-white/10",
          fade,
          covered ? "opacity-100" : "opacity-0",
        )}
      />
    </span>
  );
}

/** washOf returns no wash for an empty tint, which a grey logo has. */
function washOf(tint: string): CSSProperties | undefined {
  if (!tint) return undefined;
  return {
    backgroundImage: `radial-gradient(130% 130% at 25% 10%, color-mix(in oklab, ${tint} 48%, transparent), color-mix(in oklab, ${tint} 14%, transparent) 70%)`,
  };
}
