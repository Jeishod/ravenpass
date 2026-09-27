import assert from "node:assert/strict";
import test from "node:test";
import type { FieldDescription } from "./fields.ts";
import { type FormDescription, formIntent } from "./form-intent.ts";
import { field } from "./test-fields.test-support.ts";

const login = field({ name: "login" });
const password = field({ type: "password" });

/** A form that nothing decides, a login field and a password field, with `shape` over it. */
function form(shape: Partial<FormDescription>): FormDescription {
  return {
    fields: [login, password],
    control: "",
    attributes: "",
    path: "/",
    fragment: "",
    ...shape,
  };
}

function withExtra(extra: FieldDescription, shape: Partial<FormDescription>) {
  return form({ fields: [login, password, extra], ...shape });
}

const declared = (...autocomplete: string[]) =>
  field({ type: "password", autocomplete });

test("a form nothing decides signs in", () => {
  assert.equal(formIntent(form({})), "sign-in");
  assert.equal(
    formIntent(form({ fields: [login], control: "Continue" })),
    "sign-in",
  );
});

test("declared password fields decide first", () => {
  assert.equal(
    formIntent(
      form({ fields: [login, declared("new-password")], control: "Sign in" }),
    ),
    "sign-up",
  );
  assert.equal(
    formIntent(
      form({
        fields: [login, declared("current-password")],
        control: "Create account",
      }),
    ),
    "sign-in",
  );
  assert.equal(
    formIntent(
      form({
        fields: [login, declared("section-a", "current-password")],
        path: "/register",
      }),
    ),
    "sign-in",
  );
});

test("two or more shown password fields create an account", () => {
  assert.equal(
    formIntent(
      form({ fields: [login, password, password], control: "Log in" }),
    ),
    "sign-up",
  );
  assert.equal(
    formIntent(
      form({
        fields: [
          declared("current-password"),
          declared("new-password"),
          declared("new-password"),
        ],
      }),
    ),
    "sign-up",
  );
  assert.equal(
    formIntent(
      form({
        fields: [login, password, field({ type: "password", visible: false })],
      }),
    ),
    "sign-in",
  );
});

test("the sending control's words decide next, in English and Russian", () => {
  for (const control of [
    "Sign up",
    "Register",
    "Create account",
    "Join now",
    "Get started",
    "Зарегистрироваться",
    "Создать аккаунт",
    "Регистрация",
  ]) {
    assert.equal(
      formIntent(form({ control, attributes: "login-form", path: "/login" })),
      "sign-up",
      control,
    );
  }
  for (const control of ["Sign in", "Log In", "Войти", "Вход"]) {
    assert.equal(
      formIntent(
        withExtra(field({ label: "Full name" }), {
          control,
          attributes: "signup-form",
          path: "/signup",
        }),
      ),
      "sign-in",
      control,
    );
  }
});

test("the form's own attributes decide after its control", () => {
  assert.equal(
    formIntent(form({ control: "Continue", attributes: "registerForm" })),
    "sign-up",
  );
  assert.equal(
    formIntent(form({ attributes: "form /users/sign_up", path: "/login" })),
    "sign-up",
  );
  assert.equal(
    formIntent(
      withExtra(field({ type: "tel", label: "Phone" }), {
        attributes: "js-login-form",
        path: "/join",
      }),
    ),
    "sign-in",
  );
});

test("a field a sign-in never asks for, besides the account field, creates an account", () => {
  for (const extra of [
    field({ label: "First name" }),
    field({ name: "lastName" }),
    field({ autocomplete: ["given-name"] }),
    field({ label: "Фамилия" }),
    field({ type: "tel" }),
    field({ autocomplete: ["tel-national"] }),
    field({ placeholder: "Номер телефона" }),
    field({ type: "date", label: "Date of birth" }),
    field({ autocomplete: ["bday-day"] }),
    field({ type: "checkbox", label: "I agree to the Terms", visible: false }),
    field({ type: "checkbox", label: "Я принимаю условия оферты" }),
  ]) {
    assert.equal(
      formIntent(withExtra(extra, { path: "/login" })),
      "sign-up",
      JSON.stringify(extra),
    );
  }
});

test("a phone number that is the account field signs in", () => {
  const phone = field({ type: "tel", placeholder: "Телефон" });
  assert.equal(
    formIntent(form({ fields: [phone, password], control: "Продолжить" })),
    "sign-in",
    "a phone number and a password",
  );
  assert.equal(
    formIntent(form({ fields: [phone], control: "Продолжить" })),
    "sign-in",
    "a phone number asked for first",
  );
  assert.equal(
    formIntent(
      form({
        fields: [field({ autocomplete: ["tel"], name: "phone" }), password],
        control: "Continue",
      }),
    ),
    "sign-in",
    "a text field declared a phone number",
  );
});

test("a phone number besides the account field creates an account", () => {
  const email = field({ type: "email" });
  const phone = field({ type: "tel" });
  for (const fields of [
    [email, phone, password],
    [phone, email, password],
    [field({ label: "Имя" }), phone, password],
    [phone, field({ label: "Имя" })],
  ]) {
    assert.equal(
      formIntent(form({ fields, control: "Продолжить" })),
      "sign-up",
      JSON.stringify(fields.map(({ type, label }) => [type, label])),
    );
  }
});

test("a remember-me checkbox or a hidden field asks nothing a sign-in would not", () => {
  for (const extra of [
    field({ type: "checkbox", label: "Remember me" }),
    field({ type: "checkbox", label: "Keep me signed in" }),
    field({ type: "checkbox", label: "Запомнить меня" }),
    field({ label: "Full name", visible: false }),
    field({ type: "hidden", name: "first_name" }),
    field({ label: "Search" }),
  ]) {
    assert.equal(
      formIntent(withExtra(extra, {})),
      "sign-in",
      JSON.stringify(extra),
    );
  }
});

test("the page's address decides last", () => {
  for (const [path, fragment] of [
    ["/signup", ""],
    ["/users/sign_up", ""],
    ["/account/register.php", ""],
    ["/registration/step-1", ""],
    ["/join", ""],
    ["/", "#/create-account"],
    ["/auth/register", ""],
  ] as const) {
    assert.equal(
      formIntent(form({ path, fragment })),
      "sign-up",
      path + fragment,
    );
  }
  for (const [path, fragment] of [
    ["/login", ""],
    ["/signin", ""],
    ["/", "#/sign-in?next=%2F"],
    ["/auth", ""],
    ["/joiners", ""],
  ] as const) {
    assert.equal(
      formIntent(form({ path, fragment })),
      "sign-in",
      path + fragment,
    );
  }
});
