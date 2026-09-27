import {
  type AnimationPlaybackControls,
  type AnimationSequence,
  type BezierDefinition,
  type MotionValue,
  motionValue,
  useAnimate,
} from "motion/react";
import { useCallback, useEffect, useRef, useState } from "react";
import { vaultPart } from "../components/VaultArt.tsx";

export type IntroCard = "password" | "code" | "card" | "phrase" | "identity";

interface Place {
  readonly x: number;
  readonly y: number;
}

/** Where each item card waits around the open vault, in pixels from the stage's centre. */
export const cardPlaces: Record<
  "wide" | "compact",
  Readonly<Record<IntroCard, Place>>
> = {
  wide: {
    password: { x: -235, y: -118 },
    code: { x: 222, y: -108 },
    card: { x: -250, y: 78 },
    phrase: { x: 236, y: 92 },
    identity: { x: -10, y: -172 },
  },
  compact: {
    password: { x: -96, y: -196 },
    code: { x: 96, y: -146 },
    card: { x: -96, y: 146 },
    phrase: { x: 96, y: 196 },
    identity: { x: 0, y: -266 },
  },
};

// Android's WebView skips frames of an accelerated `clip-path` animation; the timeline writes these each frame.
export const revealEnd = "--reveal-end";
export const revealX = "--reveal-x";
export const revealY = "--reveal-y";

/** A number the timeline animates and writes to one of an element's CSS variables, in `unit`. */
function cssVariable(
  element: HTMLElement,
  name: string,
  from: number,
  unit: string,
): MotionValue<number> {
  const value = motionValue(from);
  const write = (current: number) =>
    element.style.setProperty(name, `${current}${unit}`);
  write(from);
  value.on("change", write);
  return value;
}

/** The vault's opening, from the vault's centre, where the cards disappear. */
const opening = { x: -6, y: -1 };

/** The vault is drawn at 176 px and settles into a 92 px place. */
const settledScale = 92 / 176;

const arrive: BezierDefinition = [0.22, 1, 0.36, 1];
const accelerate: BezierDefinition = [0.55, 0, 1, 0.45];
const sweep: BezierDefinition = [0.65, 0, 0.35, 1];
const overshoot: BezierDefinition = [0.34, 1.4, 0.64, 1];
const snap: BezierDefinition = [0.34, 1.8, 0.64, 1];
const drift: BezierDefinition = [0.25, 1.15, 0.4, 1];
const plunge: BezierDefinition = [0.55, 0, 0.75, 0.2];

/** Moments of the timeline, in seconds from its start. */
const beat = {
  doorOpens: 0.55,
  cardsRise: 0.85,
  cardsFly: 2,
  doorShuts: 2.95,
  impact: 3.37,
  settle: 3.97,
  promise: 4.87,
  spark: 5.67,
  line: 6.17,
  button: 6.59,
};

interface IntroParts {
  stage: HTMLElement;
  slot: HTMLElement;
  vault: HTMLElement;
  halo: HTMLElement;
  door: HTMLElement;
  seam: HTMLElement;
  handle: HTMLElement;
  cards: HTMLElement[];
  letters: HTMLElement[];
  tagline: HTMLElement;
  spark: HTMLElement;
  glow: HTMLElement;
  ping: HTMLElement;
  create: HTMLElement;
  label: HTMLElement;
  arrow: HTMLElement;
  /** The line offering to open a vault that exists; none where the host cannot open one. */
  alternatives: HTMLElement[];
}

function all(root: HTMLElement, name: string): HTMLElement[] {
  return [...root.querySelectorAll<HTMLElement>(`[data-intro="${name}"]`)];
}

function one(root: HTMLElement, name: string): HTMLElement {
  const [part] = all(root, name);
  if (!part) throw new Error(`The introduction has no ${name}.`);
  return part;
}

function readParts(root: HTMLElement): IntroParts {
  return {
    stage: one(root, "stage"),
    slot: vaultPart(root, "slot"),
    vault: vaultPart(root, "body"),
    halo: vaultPart(root, "halo"),
    door: vaultPart(root, "door"),
    seam: vaultPart(root, "seam"),
    handle: vaultPart(root, "handle"),
    cards: all(root, "card"),
    letters: all(root, "letter"),
    tagline: one(root, "tagline"),
    spark: one(root, "spark"),
    glow: one(root, "glow"),
    ping: one(root, "ping"),
    create: one(root, "create"),
    label: one(root, "label"),
    arrow: one(root, "arrow"),
    alternatives: all(root, "alternative"),
  };
}

/** The screen cross-fades in scaled down; on-screen distances divide by this to become layout pixels. */
function layoutScale(stage: HTMLElement): number {
  return stage.getBoundingClientRect().height / stage.offsetHeight || 1;
}

function centreY(element: HTMLElement): number {
  const box = element.getBoundingClientRect();
  return box.top + box.height / 2;
}

/** Where an element's centre is from the stage's centre, in layout pixels. */
function offsetFrom(stage: HTMLElement, element: HTMLElement, scale: number) {
  const from = stage.getBoundingClientRect();
  const box = element.getBoundingClientRect();
  return {
    x: (box.left + box.width / 2 - (from.left + from.width / 2)) / scale,
    y: (box.top + box.height / 2 - (from.top + from.height / 2)) / scale,
  };
}

function introSequence(parts: IntroParts): AnimationSequence {
  const scale = layoutScale(parts.stage);
  const lift = (centreY(parts.stage) - centreY(parts.slot)) / scale;
  const fall =
    (parts.tagline.getBoundingClientRect().bottom +
      10 -
      centreY(parts.create)) /
    scale;
  // The button opens from a two-pixel point, through a line, to its full height.
  const pointX = parts.create.offsetWidth / 2 - 1;
  const lineY = parts.create.offsetHeight / 2 - 1;
  const width = cssVariable(parts.create, revealX, pointX, "px");
  const height = cssVariable(parts.create, revealY, lineY, "px");
  const hidden = cssVariable(parts.tagline, revealEnd, 100, "%");

  const cards: AnimationSequence = parts.cards.flatMap((card, index) => {
    const place = offsetFrom(parts.stage, card, scale);
    const side = index % 2 ? -1 : 1;
    const rise = beat.cardsRise + index * 0.08;
    const fly = beat.cardsFly + index * 0.09;
    return [
      [
        card,
        {
          opacity: [0, 1],
          z: [-320, 0],
          rotateX: [28, 6],
          rotateY: [side * 24, side * 8],
        },
        { duration: 0.8, ease: arrive, at: rise },
      ],
      [
        card,
        {
          x: [0, opening.x - place.x],
          y: [0, opening.y - place.y],
          z: [0, -140],
          scale: [1, 0.18],
          rotateX: [6, 0],
          rotateY: [side * 8, 0],
        },
        { duration: 0.62, ease: plunge, at: fly },
      ],
      [card, { opacity: [1, 0] }, { duration: 0.16, at: fly + 0.46 }],
    ];
  });

  const letters: AnimationSequence = parts.letters.map((letter, index) => [
    letter,
    { y: ["110%", "0%"] },
    { duration: 0.8, ease: arrive, at: beat.settle + 0.2 + index * 0.06 },
  ]);

  const alternatives: AnimationSequence = parts.alternatives.map((line) => [
    line,
    { opacity: [0, 1], y: [6, 0] },
    { duration: 0.5, ease: arrive, at: beat.button + 0.5 },
  ]);

  // A sequence holds each value at its first keyframe from time 0, before its segment starts.
  return [
    [parts.vault, { opacity: [0, 1] }, { duration: 0.5, at: 0 }],
    [
      parts.vault,
      { scale: [0.86, 1] },
      { duration: 0.7, ease: overshoot, at: 0 },
    ],
    [parts.halo, { opacity: [0, 1] }, { duration: 1, at: 0 }],
    [
      parts.door,
      { rotateY: [0, -76] },
      { duration: 0.9, ease: arrive, at: beat.doorOpens },
    ],
    ...cards,
    [
      parts.door,
      { rotateY: [-76, 0] },
      { duration: 0.42, ease: accelerate, at: beat.doorShuts },
    ],
    [
      parts.door,
      { rotateY: [0, -5, 0] },
      { duration: 0.32, ease: "easeOut", at: beat.impact },
    ],
    [
      parts.vault,
      { scale: [1, 0.965, 1] },
      { duration: 0.3, ease: "easeOut", at: beat.impact },
    ],
    [
      parts.seam,
      { opacity: [0, 0.85, 0] },
      { duration: 0.9, times: [0, 0.15, 1], at: beat.impact },
    ],
    [
      parts.handle,
      { y: [-8, 0] },
      { duration: 0.35, ease: snap, at: beat.impact + 0.18 },
    ],
    [
      parts.vault,
      { y: [lift, 0], scale: [1, settledScale] },
      { duration: 0.85, ease: drift, at: beat.settle },
    ],
    [
      parts.halo,
      { y: [lift, 0], opacity: [1, 0.5] },
      { duration: 0.85, ease: arrive, at: beat.settle },
    ],
    ...letters,
    [hidden, [100, 0], { duration: 0.8, ease: sweep, at: beat.promise }],
    [parts.spark, { opacity: [0, 1, 1] }, { duration: 0.5, at: beat.spark }],
    [
      parts.spark,
      { y: [fall, 0] },
      { duration: 0.5, ease: accelerate, at: beat.spark },
    ],
    [
      parts.spark,
      { opacity: [1, 0], scaleX: [1, 6] },
      { duration: 0.3, at: beat.line },
    ],
    [parts.glow, { opacity: [0, 0.5] }, { duration: 0.4, at: beat.line }],
    // The two-pixel point would show before the spark lands on it.
    [parts.create, { opacity: [0, 1] }, { duration: 0.01, at: beat.line }],
    [width, [pointX, 0], { duration: 0.42, ease: sweep, at: beat.line }],
    [height, [lineY, 0], { duration: 0.5, ease: arrive, at: beat.button }],
    [parts.glow, { opacity: [0.5, 0.85] }, { duration: 0.5, at: beat.button }],
    [
      parts.label,
      { opacity: [0, 1], y: [6, 0] },
      { duration: 0.45, ease: arrive, at: beat.button + 0.12 },
    ],
    [
      parts.arrow,
      { opacity: [0, 1], y: [6, 0] },
      { duration: 0.45, ease: arrive, at: beat.button + 0.19 },
    ],
    ...alternatives,
  ];
}

type Animate = ReturnType<typeof useAnimate>[1];

/** The loops that keep drawing the eye to the button once the introduction has settled. */
function holdAttention(
  animate: Animate,
  parts: IntroParts,
): AnimationPlaybackControls[] {
  return [
    animate(
      parts.ping,
      { scaleX: [1, 1.12], scaleY: [1, 1.6], opacity: [0.6, 0] },
      { duration: 1.4, ease: arrive, repeat: Infinity, repeatDelay: 2.6 },
    ),
    animate(
      parts.arrow,
      { x: [0, 5, 0] },
      { duration: 0.6, ease: "easeInOut", repeat: Infinity, repeatDelay: 3.4 },
    ),
    animate(
      parts.glow,
      { opacity: [0.7, 1, 0.7] },
      { duration: 4, ease: "easeInOut", repeat: Infinity },
    ),
  ];
}

/** `play` runs the timeline, `settled` starts finished, `still` starts finished with nothing moving. */
export type IntroMode = "play" | "settled" | "still";

/** useIntroChoreography animates the introduction inside `scope`; `finish` lands every element at once. */
export function useIntroChoreography(mode: IntroMode) {
  const [scope, animate] = useAnimate<HTMLElement>();
  const [played, setPlayed] = useState(false);
  const timeline = useRef<AnimationPlaybackControls | null>(null);

  useEffect(() => {
    if (mode === "still") return;
    const parts = readParts(scope.current);
    let loops: AnimationPlaybackControls[] = [];
    if (mode === "settled") {
      loops = holdAttention(animate, parts);
      return () => {
        for (const loop of loops) loop.stop();
      };
    }
    const playing = animate(introSequence(parts));
    timeline.current = playing;
    playing.then(() => {
      if (timeline.current !== playing) return;
      // The button's focus ring is drawn outside its box, where any clip would cut it off.
      parts.create.style.clipPath = "none";
      loops = holdAttention(animate, parts);
      setPlayed(true);
    });
    return () => {
      timeline.current = null;
      playing.complete();
      for (const loop of loops) loop.stop();
    };
  }, [animate, mode, scope]);

  const finish = useCallback(() => timeline.current?.complete(), []);

  return { scope, settled: mode !== "play" || played, finish };
}
