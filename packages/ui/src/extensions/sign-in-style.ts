/** `card` shows a card in the page's corner once the page asks to sign in; `field` a menu under the focused field. */
const signInStyles = ["card", "field"] as const;

export type SignInStyle = (typeof signInStyles)[number];

export function isSignInStyle(value: unknown): value is SignInStyle {
  return signInStyles.some((style) => style === value);
}

/** An unknown value reads as `card`. */
export function signInStyleOf(value: unknown): SignInStyle {
  return isSignInStyle(value) ? value : "card";
}
