import assert from "node:assert/strict";
import test from "node:test";
import sharedCodes from "../../../../packages/app/autofill/fieldwords/testdata/codes.json" with {
  type: "json",
};
import sharedMatches from "../../../../packages/app/autofill/fieldwords/testdata/matches.json" with {
  type: "json",
};
import fieldWords from "../../../../packages/app/autofill/fieldwords/words.json" with {
  type: "json",
};
import {
  classify,
  classifyAmong,
  codeEntries,
  type FieldDescription,
  type FieldKind,
  type FieldShape,
  fillTargets,
  isVisible,
  loginValue,
  mentions,
  pairedLogin,
  wordsIn,
} from "./fields.ts";
import { field } from "./test-fields.test-support.ts";

const shapes: readonly [string, Partial<FieldDescription>, FieldKind | null][] =
  [
    // Autocomplete decides when present
    [
      "Google's identifier field",
      {
        type: "email",
        autocomplete: ["username", "webauthn"],
        name: "identifier",
        id: "identifierId",
      },
      "login",
    ],
    [
      "a username declared in a billing section",
      { autocomplete: ["section-login", "username"], name: "field_1" },
      "login",
    ],
    [
      "a phone number declared as the username",
      { type: "tel", autocomplete: ["username"], name: "phone" },
      "login",
    ],
    [
      "an email declared by autocomplete",
      { autocomplete: ["email"], name: "x1" },
      "login",
    ],
    [
      "a current password",
      { type: "password", autocomplete: ["current-password"], name: "pass" },
      "password",
    ],
    [
      "a current password shown as text",
      { type: "text", autocomplete: ["current-password"], name: "pass" },
      "password",
    ],
    [
      "a new password declared by autocomplete",
      { type: "password", autocomplete: ["new-password"], name: "password" },
      null,
    ],
    [
      "a one-time code",
      {
        type: "text",
        autocomplete: ["one-time-code"],
        name: "otp",
        label: "Verification code",
      },
      "code",
    ],
    [
      "autocomplete off falls through to the type",
      { type: "password", autocomplete: ["off"], name: "pwd" },
      "password",
    ],
    // Password inputs
    [
      "GitHub's password field",
      { type: "password", name: "password", id: "password" },
      "password",
    ],
    ["a new password by name", { type: "password", name: "newPassword" }, null],
    [
      "a confirmation by id",
      { type: "password", name: "pass2", id: "password_confirmation" },
      null,
    ],
    [
      "a repeated password by placeholder",
      { type: "password", placeholder: "Repeat password" },
      null,
    ],
    [
      "a retyped password by label",
      { type: "password", label: "Retype your password" },
      null,
    ],
    [
      "a Russian new password",
      { type: "password", label: "Новый пароль" },
      null,
    ],
    [
      "a Russian confirmation",
      { type: "password", placeholder: "Подтвердите пароль" },
      null,
    ],
    [
      "a Russian repeated password",
      { type: "password", label: "Повторите пароль" },
      null,
    ],
    [
      "a renewal wording is not new",
      { type: "password", name: "renewal_pin" },
      "password",
    ],
    ["a Russian password", { type: "password", label: "Пароль" }, "password"],
    // Login inputs
    ["an email input", { type: "email", name: "subscriber" }, "login"],
    ["a username by name", { name: "user_name", id: "login_field" }, "login"],
    ["an account by camel case", { name: "accountName" }, "login"],
    ["an e-mail placeholder", { placeholder: "E-mail" }, "login"],
    [
      "a text field before the password",
      { name: "q7", precedesPassword: true },
      "login",
    ],
    ["a Russian login", { label: "Логин или e-mail", name: "f1" }, "login"],
    ["a Russian username", { placeholder: "Имя пользователя" }, "login"],
    ["a Russian email", { label: "Электронная почта" }, "login"],
    ["a Russian account", { label: "Учётная запись" }, "login"],
    [
      "an unrelated text field",
      { name: "first_name", label: "First name" },
      null,
    ],
    [
      "a phone field without a declaration",
      { type: "tel", name: "user_phone" },
      null,
    ],
    // One-time code inputs
    [
      "GitHub's two-factor field",
      {
        autocomplete: ["one-time-code"],
        inputMode: "numeric",
        name: "app_otp",
        id: "app_totp",
        placeholder: "XXXXXX",
      },
      "code",
    ],
    [
      "Google's authenticator field",
      {
        type: "tel",
        autocomplete: ["one-time-code"],
        name: "totpPin",
        id: "totpPin",
      },
      "code",
    ],
    [
      "a number input declared as a one-time code",
      { type: "number", autocomplete: ["one-time-code"], name: "pin" },
      "code",
    ],
    [
      "Microsoft's code field",
      {
        type: "tel",
        autocomplete: ["off"],
        maxLength: 8,
        name: "otc",
        id: "idTxtBx_SAOTCC_OTC",
        placeholder: "Code",
      },
      "code",
    ],
    [
      "an MFA code by camel case",
      { name: "mfaCode", id: "mfacode", maxLength: 6 },
      "code",
    ],
    [
      "a numeric one-time password by label",
      { inputMode: "numeric", label: "One-time password" },
      "code",
    ],
    ["a 2FA field by name", { type: "tel", maxLength: 6, name: "2FA" }, "code"],
    ["a numeric token", { inputMode: "numeric", name: "token" }, "code"],
    [
      "a verification field that asks for digits",
      { type: "number", inputMode: "numeric", placeholder: "Verify" },
      "code",
    ],
    [
      "a Russian confirmation code",
      { inputMode: "numeric", label: "Код подтверждения", name: "f2" },
      "code",
    ],
    [
      "a Russian code by placeholder",
      { maxLength: 6, placeholder: "Введите код из приложения" },
      "code",
    ],
    [
      "a code field that takes too many characters",
      { name: "promo_code", maxLength: 20 },
      null,
    ],
    [
      "a code field that takes too few characters",
      { name: "country_code", maxLength: 3 },
      null,
    ],
    [
      "a code field without a numeric hint or a length",
      { name: "coupon_code", label: "Coupon code" },
      null,
    ],
    [
      "a numeric field whose naming says nothing of a code",
      { type: "tel", inputMode: "numeric", maxLength: 10, name: "phone" },
      null,
    ],
    [
      "an email input naming a code stays a login field",
      { type: "email", inputMode: "numeric", maxLength: 6, name: "code" },
      "login",
    ],
    [
      "a code field before a password is a code field",
      { maxLength: 6, name: "code", precedesPassword: true },
      "code",
    ],
    [
      "a password input naming a code stays a password field",
      { type: "password", inputMode: "numeric", maxLength: 6, name: "otp" },
      "password",
    ],
    [
      "a hidden one-time code",
      { autocomplete: ["one-time-code"], visible: false },
      null,
    ],
    // Never classified
    ["a hidden username", { type: "hidden", name: "username" }, null],
    ["a zero-size username", { name: "username", visible: false }, null],
    [
      "a disabled password",
      { type: "password", name: "password", editable: false },
      null,
    ],
    [
      "a read-only email",
      { type: "email", autocomplete: ["email"], editable: false },
      null,
    ],
    ["a search input", { type: "search", name: "q" }, null],
    [
      "a user search box",
      { name: "user_search", placeholder: "Search users" },
      null,
    ],
    [
      "a Russian search box",
      { placeholder: "Поиск по сайту", precedesPassword: true },
      null,
    ],
    ["a checkbox", { type: "checkbox", name: "remember_login" }, null],
  ];

for (const [name, shape, expected] of shapes) {
  test(`classify: ${name}`, () => {
    assert.equal(classify(field(shape)), expected);
  });
}

/** A box of a split code field as code inputs render it: one digit, asking for digits. */
function box(shape: Partial<FieldDescription> = {}): FieldDescription {
  return field({ inputMode: "numeric", maxLength: 1, ...shape });
}

function boxes(count: number): FieldDescription[] {
  return Array.from({ length: count }, () => box());
}

const runs: readonly [
  string,
  readonly FieldDescription[],
  number,
  FieldShape | null,
][] = [
  [
    "six boxes, focused on the first",
    boxes(6),
    0,
    { kind: "code", inputs: [0, 1, 2, 3, 4, 5] },
  ],
  [
    "six boxes, focused in the middle",
    boxes(6),
    3,
    { kind: "code", inputs: [0, 1, 2, 3, 4, 5] },
  ],
  ["four boxes", boxes(4), 3, { kind: "code", inputs: [0, 1, 2, 3] }],
  ["three boxes are no split field", boxes(3), 1, null],
  [
    "the first box declaring a one-time code",
    [box({ autocomplete: ["one-time-code"] }), ...boxes(5)],
    0,
    { kind: "code", inputs: [0, 1, 2, 3, 4, 5] },
  ],
  [
    "tel boxes without naming",
    Array.from({ length: 6 }, () =>
      field({ type: "tel", maxLength: 1, name: "digit" }),
    ),
    2,
    { kind: "code", inputs: [0, 1, 2, 3, 4, 5] },
  ],
  [
    "number boxes",
    Array.from({ length: 6 }, () => field({ type: "number", maxLength: 1 })),
    5,
    { kind: "code", inputs: [0, 1, 2, 3, 4, 5] },
  ],
  [
    "boxes after an unrelated input and before a hidden one",
    [
      field({ name: "first_name" }),
      ...boxes(6),
      field({ type: "hidden", name: "code" }),
    ],
    1,
    { kind: "code", inputs: [1, 2, 3, 4, 5, 6] },
  ],
  [
    "a wider input breaks the run",
    [...boxes(3), box({ maxLength: 2 }), ...boxes(3)],
    1,
    null,
  ],
  [
    "a hidden box breaks the run",
    [...boxes(3), box({ visible: false }), ...boxes(3)],
    5,
    null,
  ],
  [
    "a read-only box breaks the run",
    [...boxes(2), box({ editable: false }), ...boxes(4)],
    4,
    { kind: "code", inputs: [3, 4, 5, 6] },
  ],
  [
    "password boxes stay password fields",
    Array.from({ length: 6 }, () => field({ type: "password", maxLength: 1 })),
    0,
    { kind: "password", inputs: [0] },
  ],
  [
    "a box before a password stays a login field",
    [...boxes(3), box({ precedesPassword: true }), field({ type: "password" })],
    3,
    { kind: "login", inputs: [3] },
  ],
  [
    "a single code field among its siblings",
    [
      field({ name: "remember", type: "checkbox" }),
      field({ autocomplete: ["one-time-code"], maxLength: 6 }),
    ],
    1,
    { kind: "code", inputs: [1] },
  ],
  [
    "an input that is no field",
    [field({ name: "first_name" }), ...boxes(4)],
    0,
    null,
  ],
];

for (const [name, siblings, index, expected] of runs) {
  test(`classifyAmong: ${name}`, () => {
    assert.deepEqual(classifyAmong(siblings, index), expected);
  });
}

/** An input precedes a password when the next input is a visible password. */
function siblingsOf(
  inputs: readonly Partial<FieldDescription>[],
): FieldDescription[] {
  return inputs.map((input, index) => {
    const next = inputs[index + 1];
    return field({
      ...input,
      precedesPassword: next?.type === "password" && next.visible !== false,
    });
  });
}

for (const { name, inputs, focus, code } of sharedCodes.fields) {
  test(`shared code fields: ${name}`, () => {
    const shape = classifyAmong(siblingsOf(inputs), focus);
    assert.deepEqual(shape?.kind === "code" ? shape.inputs : null, code);
  });
}

test("a code fills its fields as the shared table says", () => {
  for (const { code, boxes, entries } of sharedCodes.entries) {
    assert.deepEqual(
      codeEntries(code, boxes),
      entries,
      `${code} over ${boxes}`,
    );
  }
});

test("an email field takes the email when the credential has one", () => {
  const values = { login: "alex", email: "alex@example.com" };
  assert.equal(loginValue(field({ type: "email" }), values), values.email);
  assert.equal(
    loginValue(field({ autocomplete: ["section-a", "email"] }), values),
    values.email,
  );
});

test("an email field takes the login when the credential has no email", () => {
  assert.equal(
    loginValue(field({ type: "email" }), { login: "alex", email: "" }),
    "alex",
  );
});

test("a fill from a detected field goes where detection places the login and the password", () => {
  const kinds = ["login", null, "password"] as const;
  assert.deepEqual(fillTargets(kinds, 0, "login"), { login: 0, password: 2 });
  assert.deepEqual(fillTargets(kinds, 2, "password"), {
    login: 0,
    password: 2,
  });
  assert.deepEqual(fillTargets(kinds, 0, "code"), { login: 0, password: 2 });
});

test("a field detection missed takes the login the menu was opened for, and the form's password field the password", () => {
  assert.deepEqual(fillTargets([null, "password"], 0, "login"), {
    login: 0,
    password: 1,
  });
  assert.deepEqual(fillTargets([null, null], 1, "login"), {
    login: 1,
    password: -1,
  });
  assert.deepEqual(fillTargets([null], -1, "login"), {
    login: -1,
    password: -1,
  });
});

test("a password field pairs with the last login field before it", () => {
  assert.equal(pairedLogin(["login", null, "login", "password"], 3), 2);
  assert.equal(pairedLogin([null, "password", "login"], 1), -1);
});

test("a form without a password field pairs its first login field", () => {
  assert.equal(pairedLogin([null, "login", "login"], -1), 1);
  assert.equal(pairedLogin([null, "code"], -1), -1);
});

test("a login field takes the login, else the email", () => {
  const values = { login: "alex", email: "alex@example.com" };
  assert.equal(
    loginValue(field({ autocomplete: ["username"] }), values),
    "alex",
  );
  assert.equal(
    loginValue(field({ name: "user" }), { login: "", email: values.email }),
    values.email,
  );
  assert.equal(loginValue(field({}), { login: "", email: "" }), "");
});

test("field naming mentions the term lists the shared table says", () => {
  const lists = Object.entries(fieldWords);
  assert.ok(sharedMatches.length > 0);
  for (const { text, mentions: expected } of sharedMatches) {
    const words = wordsIn(text);
    const found = lists
      .filter(([, terms]) => mentions(words, terms))
      .map(([name]) => name);
    assert.deepEqual(found, expected, text);
  }
});

function styledInput({ hidden = false, transparent = false }) {
  return {
    getBoundingClientRect: () => ({ width: 120, height: 32 }) as DOMRect,
    checkVisibility: (options?: CheckVisibilityOptions) =>
      !(hidden && options?.visibilityProperty) &&
      !(transparent && options?.opacityProperty),
  };
}

test("a fully transparent input is not visible, so its form is not reported", () => {
  assert.equal(isVisible(styledInput({})), true);
  assert.equal(isVisible(styledInput({ transparent: true })), false);
  assert.equal(isVisible(styledInput({ hidden: true })), false);
  assert.equal(classify(field({ type: "password", visible: false })), null);
});
