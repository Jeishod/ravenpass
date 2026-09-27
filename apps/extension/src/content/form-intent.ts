import {
  classify,
  type FieldDescription,
  type FieldKind,
  mentions,
  pairedLogin,
  wordsIn,
} from "./fields.ts";

export type FormIntent = "sign-in" | "sign-up";

export interface FormDescription {
  /** The form's inputs in document order. */
  readonly fields: readonly FieldDescription[];
  /** The text of the control that sends the form, empty for none. */
  readonly control: string;
  /** The `<form>`'s `id`, `name`, `class`, `aria-label` and `action` path; empty without a `<form>`. */
  readonly attributes: string;
  /** The path of the page's address. */
  readonly path: string;
  readonly fragment: string;
}

// Terms are word prefixes, matched by `mentions` in fields.ts.
export const signUpTerms: readonly string[] = [
  "sign up",
  "signup",
  "regist",
  "create",
  "join",
  "get started",
  "зарегистр",
  "регистр",
  "создать",
  "присоедин",
];
export const signInTerms: readonly string[] = [
  "sign in",
  "signin",
  "log in",
  "login",
  "log on",
  "logon",
  "войти",
  "вход",
  "авториз",
];

const nameTokens: ReadonlySet<string> = new Set([
  "name",
  "given-name",
  "additional-name",
  "family-name",
  "honorific-prefix",
  "honorific-suffix",
]);
const nameTerms = [
  "name",
  "first name",
  "last name",
  "firstname",
  "lastname",
  "fullname",
  "surname",
  "имя",
  "фамил",
  "отчеств",
];
const phoneTerms = ["phone", "mobile", "телефон", "мобильн"];
const birthTerms = ["birth", "bday", "dob", "рожден"];
const termsTerms = [
  "terms",
  "agree",
  "accept",
  "consent",
  "policy",
  "privacy",
  "согла",
  "услови",
  "политик",
  "оферт",
  "принима",
];
const rememberTerms = ["remember", "stay", "keep", "запомн"];

const signUpSegments: ReadonlySet<string> = new Set([
  "signup",
  "sign-up",
  "register",
  "registration",
  "join",
  "create-account",
  "createaccount",
]);
const signInSegments: ReadonlySet<string> = new Set([
  "login",
  "log-in",
  "signin",
  "sign-in",
  "logon",
  "auth",
]);

/** Input types an account field takes, a phone number among them. */
const accountTypes: ReadonlySet<string> = new Set(["text", "email", "tel"]);

export function formIntent(form: FormDescription): FormIntent {
  return (
    declaredIntent(form.fields) ??
    wordsIntent(form.control) ??
    wordsIntent(form.attributes) ??
    (extraFields(form.fields).some(asksBeyondSignIn) ? "sign-up" : null) ??
    addressIntent(form.path, form.fragment) ??
    "sign-in"
  );
}

function declaredIntent(
  fields: readonly FieldDescription[],
): FormIntent | null {
  const passwords = fields
    .filter(isShownPassword)
    .map((field) => field.autocomplete);
  const declares = (token: string) =>
    passwords.some((tokens) => tokens.includes(token));
  const current = declares("current-password");
  const created = declares("new-password");
  if (created && !current) return "sign-up";
  if (current && !created) return "sign-in";
  return passwords.length >= 2 ? "sign-up" : null;
}

function isShownPassword(field: FieldDescription): boolean {
  return field.type === "password" && field.visible;
}

function extraFields(fields: readonly FieldDescription[]): FieldDescription[] {
  const kinds = fields.map(classify);
  const account = accountAt(fields, kinds);
  return fields.filter(
    (field, index) =>
      index !== account &&
      kinds[index] === null &&
      field.type !== "hidden" &&
      field.type !== "password",
  );
}

/** The paired login field, else the last account-like field before the password; -1 for none. */
function accountAt(
  fields: readonly FieldDescription[],
  kinds: readonly (FieldKind | null)[],
): number {
  const passwordAt = fields.findIndex(isShownPassword);
  const login = pairedLogin(kinds, passwordAt);
  if (login >= 0) return login;
  const candidates = fields.flatMap((field, index) =>
    kinds[index] === null && field.visible && accountTypes.has(field.type)
      ? [index]
      : [],
  );
  if (passwordAt >= 0) {
    return candidates.filter((index) => index < passwordAt).at(-1) ?? -1;
  }
  const [only, ...others] = candidates;
  return only !== undefined && others.length === 0 ? only : -1;
}

function wordsIntent(text: string): FormIntent | null {
  const words = wordsIn(text);
  if (mentions(words, signUpTerms)) return "sign-up";
  if (mentions(words, signInTerms)) return "sign-in";
  return null;
}

/** A checkbox counts even when hidden: pages often draw a picture over the real input. */
function asksBeyondSignIn(field: FieldDescription): boolean {
  const words = wordsIn(
    [field.name, field.id, field.placeholder, field.label].join(" "),
  );
  if (field.type === "checkbox") {
    return mentions(words, termsTerms) && !mentions(words, rememberTerms);
  }
  if (!field.visible) return false;
  const declares = (matches: (token: string) => boolean) =>
    field.autocomplete.some(matches);
  return (
    declares((token) => nameTokens.has(token)) ||
    declares((token) => token.startsWith("tel")) ||
    declares((token) => token.startsWith("bday")) ||
    field.type === "tel" ||
    mentions(words, nameTerms) ||
    mentions(words, phoneTerms) ||
    mentions(words, birthTerms)
  );
}

function addressIntent(path: string, fragment: string): FormIntent | null {
  const [route = ""] = fragment.replace(/^#/, "").split(/[?&]/);
  const segments = [path, route]
    .flatMap((part) => part.split("/"))
    .map((segment) =>
      segment
        .toLowerCase()
        .replace(/\.[a-z0-9]+$/, "")
        .replaceAll("_", "-"),
    );
  if (segments.some((segment) => signUpSegments.has(segment))) {
    return "sign-up";
  }
  if (segments.some((segment) => signInSegments.has(segment))) {
    return "sign-in";
  }
  return null;
}
