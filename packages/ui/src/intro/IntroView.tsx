import { cn } from "cn";
import {
  CreditCard,
  IdCard,
  KeyRound,
  type LucideIcon,
  Sprout,
  Timer,
} from "lucide-react";
import { useReducedMotionConfig } from "motion/react";
import { type CSSProperties, useEffect, useEffectEvent, useState } from "react";
import { toast } from "sonner";
import { AccessHeader } from "../components/AccessShell.tsx";
import { Button } from "../components/ui/button.tsx";
import { VaultMark } from "../components/VaultArt.tsx";
import { useCapabilities } from "../host/capabilities.tsx";
import { useCompactLayout } from "../host/compact.ts";
import type { MessageKey } from "../i18n/messages.ts";
import { useTranslator } from "../i18n/translator.tsx";
import { ForwardArrow } from "../motion/ForwardArrow.tsx";
import type { VaultApi, VaultState } from "../vault-api.ts";
import {
  cardPlaces,
  type IntroCard,
  type IntroMode,
  revealEnd,
  revealX,
  revealY,
  useIntroChoreography,
} from "./choreography.ts";

type Label = { text: string } | { message: MessageKey };

/** Staged, the tagline and the button are clipped to nothing until the choreography opens them. */
const taglineReveal = {
  clipPath: `inset(0 var(${revealEnd}) 0 0)`,
  [revealEnd]: "100%",
} as CSSProperties;

const buttonReveal = {
  clipPath: `inset(var(${revealY}) var(${revealX}) round 999px)`,
  [revealX]: "50%",
  [revealY]: "50%",
  // A transition on the clip would trail each frame of the reveal.
  transitionProperty: "scale, box-shadow, opacity",
} as CSSProperties;

const cards: {
  id: IntroCard;
  icon: LucideIcon;
  title: Label;
  detail: Label;
}[] = [
  {
    id: "password",
    icon: KeyRound,
    title: { text: "github.com" },
    detail: { text: "••••••••••" },
  },
  {
    id: "code",
    icon: Timer,
    title: { message: "intro.card.code" },
    detail: { text: "482 915" },
  },
  {
    id: "card",
    icon: CreditCard,
    title: { message: "intro.card.card" },
    detail: { text: "•••• 4242" },
  },
  {
    id: "phrase",
    icon: Sprout,
    title: { message: "intro.card.phrase" },
    detail: { message: "intro.card.phrase-words" },
  },
  {
    id: "identity",
    icon: IdCard,
    title: { message: "intro.card.identity" },
    detail: { text: "•••• 7781" },
  },
];

/** IntroView welcomes a device that knows no vault and offers to create or open one. */
export function IntroView({
  api,
  played = false,
  onPhase,
}: {
  api: VaultApi;
  /** Whether the introduction has already played, so it returns settled. */
  played?: boolean;
  onPhase: (phase: VaultState["phase"]) => void;
}) {
  const { t, failure } = useTranslator();
  const { storageLocations } = useCapabilities();
  const reduced = useReducedMotionConfig() === true;
  const compact = useCompactLayout();
  // The cards keep the places the screen opened with, so turning a phone mid-flight moves none.
  const [places] = useState(() => cardPlaces[compact ? "compact" : "wide"]);
  const mode: IntroMode = reduced ? "still" : played ? "settled" : "play";
  const { scope, settled, finish } = useIntroChoreography(mode);
  const [opening, setOpening] = useState(false);
  // Staged elements start where the timeline's first keyframes put them.
  const staged = mode === "play";

  function label(value: Label) {
    return "text" in value ? value.text : t(value.message);
  }

  function create() {
    onPhase("setup");
  }

  async function open() {
    setOpening(true);
    try {
      const change = await api.openVault();
      if (change.changed) onPhase((await api.getState()).phase);
    } catch (reason) {
      toast.error(failure(reason, "intro.errors.open-failed"));
    } finally {
      setOpening(false);
    }
  }

  // A key finishes the introduction; once settled, Return on the page body creates a vault.
  const onKey = useEffectEvent((event: KeyboardEvent) => {
    if (!settled) {
      finish();
      return;
    }
    if (event.key === "Enter" && event.target === document.body && !opening) {
      create();
    }
  });

  useEffect(() => {
    const listener = (event: KeyboardEvent) => onKey(event);
    window.addEventListener("keydown", listener);
    return () => window.removeEventListener("keydown", listener);
  }, []);

  return (
    <main
      ref={scope}
      onPointerDown={settled ? undefined : finish}
      className="relative h-dvh min-h-0 min-w-0 overflow-hidden bg-background text-foreground"
    >
      <AccessHeader className="absolute inset-x-0 top-0 z-20" />

      <div
        data-intro="stage"
        className="absolute inset-0 flex flex-col items-center justify-center max-sm:px-6"
      >
        <VaultMark staged={staged} className="mb-6" />

        <h1 className="flex overflow-hidden px-0.5 pb-1 text-[46px] leading-[1.1] font-semibold tracking-[-0.035em] max-sm:text-[38px]">
          <span className="sr-only">Ravenpass</span>
          {Array.from("Ravenpass", (letter, index) => (
            <span
              // biome-ignore lint/suspicious/noArrayIndexKey: the wordmark's letters never change.
              key={index}
              data-intro="letter"
              aria-hidden="true"
              className={cn(
                "inline-block",
                staged && "[transform:translateY(110%)]",
              )}
            >
              {letter}
            </span>
          ))}
        </h1>

        <p
          data-intro="tagline"
          className="mt-3 text-[15px] text-muted-foreground max-sm:text-center max-sm:text-sm max-sm:text-balance"
          style={staged ? taglineReveal : undefined}
        >
          {t("intro.tagline")}
        </p>

        <div className="mt-8 flex flex-col items-center gap-[18px] max-sm:absolute max-sm:inset-x-5 max-sm:bottom-[calc(env(safe-area-inset-bottom)+1.5rem)] max-sm:mt-0">
          <div className="group/cta relative h-13 w-70 max-sm:h-12 max-sm:w-full">
            <div
              aria-hidden="true"
              className="pointer-events-none absolute top-1/2 left-1/2 -mt-11 -ml-[220px] h-[130px] w-[440px] opacity-85 transition-[opacity,scale] duration-300 group-hover/cta:scale-110 group-hover/cta:opacity-100 motion-reduce:transition-none"
            >
              <div
                data-intro="glow"
                className={cn(
                  "size-full bg-radial from-foreground/17 to-transparent to-70%",
                  staged ? "opacity-0" : "opacity-85",
                )}
              />
            </div>
            <div
              data-intro="ping"
              aria-hidden="true"
              className="pointer-events-none absolute inset-0 rounded-full border border-foreground/55 opacity-0"
            />
            <div
              data-intro="spark"
              aria-hidden="true"
              className="pointer-events-none absolute top-1/2 left-1/2 -mt-[3px] -ml-[3px] size-1.5 rounded-full bg-foreground opacity-0 shadow-[0_0_14px_4px_color-mix(in_oklab,var(--color-foreground)_55%,transparent)]"
            />
            <Button
              data-intro="create"
              type="button"
              variant="raised"
              onClick={create}
              disabled={opening}
              className={cn(
                "group relative size-full gap-2.5 rounded-full text-[15px] tracking-[-0.01em] shadow-[inset_0_1px_0_rgb(255_255_255/0.16),inset_0_-1px_0_rgb(0_0_0/0.3)] dark:shadow-[inset_0_1px_0_#fff,inset_0_-1px_0_rgb(0_0_0/0.12)] motion-safe:hover:scale-[1.025]",
                staged && "opacity-0",
              )}
              style={staged ? buttonReveal : undefined}
            >
              <span data-intro="label" className={cn(staged && "opacity-0")}>
                {t("intro.create")}
              </span>
              <ForwardArrow
                data-intro="arrow"
                className={cn("size-[18px]", staged && "opacity-0")}
              />
            </Button>
          </div>

          {storageLocations && (
            <p
              data-intro="alternative"
              className={cn("text-[13px] text-faint", staged && "opacity-0")}
            >
              {t("intro.open-prompt")}{" "}
              <button
                type="button"
                onClick={open}
                disabled={opening}
                className="font-medium text-foreground underline-offset-3 outline-none hover:underline focus-visible:underline disabled:opacity-50"
              >
                {opening ? t("intro.open-busy") : t("intro.open")}
              </button>
            </p>
          )}
        </div>
      </div>

      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 z-10 perspective-[1100px]"
      >
        {cards.map(({ id, icon: Icon, title, detail }) => (
          <div
            key={id}
            data-intro="card"
            style={{
              left: `calc(50% + ${places[id].x}px)`,
              top: `calc(50% + ${places[id].y}px)`,
            }}
            className="absolute -mt-[27px] -ml-20 flex w-40 items-center gap-2.5 rounded-[14px] border border-input bg-muted px-3 py-2.5 opacity-0"
          >
            <Icon className="size-[18px] shrink-0 text-muted-foreground" />
            <div className="min-w-0">
              <p className="truncate text-xs font-medium">{label(title)}</p>
              <p className="truncate font-mono text-[11px] text-muted-foreground tabular-nums">
                {label(detail)}
              </p>
            </div>
          </div>
        ))}
      </div>
    </main>
  );
}
