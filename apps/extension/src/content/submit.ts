import {
  boxInputs,
  boxOf,
  type FormBox,
  isVisible,
  mentions,
  wordsIn,
} from "./fields.ts";
import { signInTerms, signUpTerms } from "./form-intent.ts";

export interface ControlDescription {
  /** Whether it submits a `<form>` element: a submit button or an image input of one. */
  readonly submits: boolean;
  /** Its text or value, its `aria-label` and its title. */
  readonly text: string;
  /** Whether it takes space on the page and its style does not hide it. */
  readonly visible: boolean;
  /** Whether it is neither disabled nor marked disabled for assistive technology. */
  readonly enabled: boolean;
  /** Whether it follows the form's last visible input in document order. */
  readonly afterFields: boolean;
}

// Terms are word prefixes, matched by `mentions` in fields.ts.
const sendTerms = [
  ...signInTerms,
  "next",
  "continue",
  "submit",
  "verif",
  "confirm",
  "далее",
  "дальше",
  "продолж",
  "подтверд",
  "отправ",
];
const otherWayTerms = [
  "forgot",
  "reset",
  "with",
  "passkey",
  "sso",
  "забыл",
  "восстанов",
  "через",
  "помощью",
];
const savePasswordTerms = [
  "save",
  "change",
  "update",
  "сохран",
  "измен",
  "смен",
  "обнов",
];
const dismissTerms = [
  "cancel",
  "close",
  "back",
  "dismiss",
  "отмен",
  "закры",
  "назад",
  "вернут",
];

/** `may-send`: a script handling any click can send the form. */
export type ControlEffect = "sends" | "may-send" | "unsent";

export function controlEffect(
  controls: readonly ControlDescription[],
  clicked: number,
): ControlEffect {
  const control = controls[clicked];
  if (!control) return "unsent";
  if (control.submits) return "sends";
  const words = wordsIn(control.text);
  if (
    !controls.some(({ submits }) => submits) &&
    (clicked === sendingControl(controls) ||
      mentions(words, signUpTerms) ||
      mentions(words, savePasswordTerms))
  ) {
    return "sends";
  }
  return mentions(words, dismissTerms) || mentions(words, otherWayTerms)
    ? "unsent"
    : "may-send";
}

/** The submit control Enter would click, else the first control after the fields whose text sends. */
export function sendingControl(
  controls: readonly ControlDescription[],
): number {
  const usable = (control: ControlDescription) =>
    control.visible &&
    control.enabled &&
    !mentions(wordsIn(control.text), otherWayTerms);
  const submit = controls.findIndex(
    (control) => control.submits && usable(control),
  );
  if (submit >= 0) return submit;
  return controls.findIndex((control) => {
    const words = wordsIn(control.text);
    return (
      control.afterFields &&
      usable(control) &&
      (mentions(words, sendTerms) || mentions(words, signUpTerms))
    );
  });
}

/** The control a fill clicks to send a sign-in; never one that signs up. */
export function submitControl(controls: readonly ControlDescription[]): number {
  const index = sendingControl(controls);
  const control = controls[index];
  return control && !mentions(wordsIn(control.text), signUpTerms) ? index : -1;
}

export const controlSelector =
  "button, input[type=submit], input[type=image], input[type=button], [role=button]";

/** False when there is nothing to click or the sending control signs up. */
export function submitForm(field: HTMLInputElement): boolean {
  const box = boxOf(field);
  const { controls, descriptions } = readControls(box, field);
  const chosen = controls[submitControl(descriptions)];
  if (chosen) {
    chosen.click();
    return true;
  }
  if (box instanceof HTMLFormElement && sendingControl(descriptions) < 0) {
    box.requestSubmit();
    return true;
  }
  return false;
}

export function sendingControlText(box: FormBox): string {
  const { descriptions } = readControls(box);
  return descriptions[sendingControl(descriptions)]?.text ?? "";
}

/** `field` stands in for the box's last visible input while it has none. */
export function readControls(
  box: FormBox,
  field?: HTMLInputElement,
): { controls: HTMLElement[]; descriptions: ControlDescription[] } {
  const last = boxInputs(box).filter(isVisible).at(-1) ?? field;
  const controls = controlsOf(box);
  return {
    controls,
    descriptions: controls.map((control) => describeControl(control, last)),
  };
}

/** Includes controls associated with a form through their `form` attribute. */
function controlsOf(box: FormBox): HTMLElement[] {
  const scope = box instanceof HTMLFormElement ? box.getRootNode() : box;
  if (!(scope instanceof Document || scope instanceof ShadowRoot)) return [];
  return Array.from(
    scope.querySelectorAll<HTMLElement>(controlSelector),
  ).filter((control) => boxOf(control) === box);
}

function describeControl(
  control: HTMLElement,
  lastField: HTMLInputElement | undefined,
): ControlDescription {
  const isField =
    control instanceof HTMLButtonElement || control instanceof HTMLInputElement;
  return {
    submits:
      isField &&
      control.form !== null &&
      (control.type === "submit" || control.type === "image"),
    text: [
      control instanceof HTMLInputElement ? control.value : control.textContent,
      control.getAttribute("aria-label"),
      control.title,
    ].join(" "),
    visible: isVisible(control),
    enabled:
      !(isField && control.disabled) &&
      control.getAttribute("aria-disabled") !== "true",
    afterFields: Boolean(
      lastField &&
        lastField.compareDocumentPosition(control) &
          Node.DOCUMENT_POSITION_FOLLOWING,
    ),
  };
}
