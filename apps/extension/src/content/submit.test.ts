import assert from "node:assert/strict";
import test from "node:test";
import { formIntent } from "./form-intent.ts";
import {
  type ControlDescription,
  controlEffect,
  sendingControl,
  submitControl,
} from "./submit.ts";
import { field } from "./test-fields.test-support.ts";

/** A visible, enabled button after the fields that does not submit a form, with `shape` over it. */
function control(shape: Partial<ControlDescription>): ControlDescription {
  return {
    submits: false,
    text: "",
    visible: true,
    enabled: true,
    afterFields: true,
    ...shape,
  };
}

test("a form's first visible, enabled submit control is the one clicked", () => {
  assert.equal(
    submitControl([
      control({ text: "Show password" }),
      control({ submits: true, text: "Sign in", visible: false }),
      control({ submits: true, text: "Sign in", enabled: false }),
      control({ submits: true, text: "Sign in" }),
      control({ submits: true, text: "Log in" }),
    ]),
    3,
  );
});

test("a submit control is clicked whatever it says, unless it names another way in or signing up", () => {
  assert.equal(submitControl([control({ submits: true, text: "→" })]), 0);
  const signUp = [
    control({ submits: true, text: "Sign in with a passkey" }),
    control({ submits: true, text: "Forgot password?" }),
    control({ submits: true, text: "Create account" }),
  ];
  assert.equal(sendingControl(signUp), 2);
  assert.equal(submitControl(signUp), -1);
});

test("a sign-in button inside a sign-up form is not the control that sends it", () => {
  const withSubmit = [
    control({ submits: true, text: "Create account" }),
    control({ text: "Already have an account? Sign in" }),
  ];
  const withoutForm = [
    control({ text: "Sign in", afterFields: false }),
    control({ text: "Зарегистрироваться" }),
    control({ text: "Уже есть аккаунт? Войти" }),
  ];
  for (const controls of [withSubmit, withoutForm]) {
    const sending = controls[sendingControl(controls)];
    assert.ok(sending);
    assert.equal(
      formIntent({
        fields: [field({ type: "email" }), field({ type: "password" })],
        control: sending.text,
        attributes: "",
        path: "/login",
        fragment: "",
      }),
      "sign-up",
    );
    assert.equal(submitControl(controls), -1);
  }
});

test("without a submit control, the control after the fields that says to sign in is clicked", () => {
  assert.equal(
    submitControl([
      control({ text: "Log in", afterFields: false }),
      control({ text: "Show password" }),
      control({ text: "Continue with Google" }),
      control({ text: "Next" }),
      control({ text: "Sign in" }),
    ]),
    3,
  );
  for (const text of [
    "Sign in",
    "Log In",
    "signIn",
    "Continue",
    "Verify",
    "Confirm",
    "Войти",
    "Далее",
    "Продолжить",
    "Подтвердить",
  ]) {
    assert.equal(submitControl([control({ text })]), 0, text);
  }
});

/** A sign-in form's password field with its show-password toggle, then the button that signs in. */
function signInControls(signIn: Partial<ControlDescription>) {
  return [
    control({ text: "Show password", afterFields: false }),
    control(signIn),
  ];
}

test("a type=button button or a role=button element that signs in sends a form without a submit control", () => {
  // Element Plus, Vuetify, Quasar and Ant Design Vue buttons render as type="button".
  for (const text of ["Login", "LOG IN", "Sign in", "Войти", "Продолжить"]) {
    assert.equal(controlEffect(signInControls({ text }), 1), "sends", text);
  }
});

test("a control that signs up, or saves or changes a password, sends a form without a submit control", () => {
  for (const text of [
    "Create account",
    "Зарегистрироваться",
    "Save",
    "Change password",
    "Сохранить",
    "Сменить пароль",
  ]) {
    assert.equal(
      controlEffect([control({ text, afterFields: false })], 0),
      "sends",
      text,
    );
  }
});

test("a show-password toggle, or an icon without words, may send the form", () => {
  assert.equal(
    controlEffect(signInControls({ text: "Log in" }), 0),
    "may-send",
  );
  assert.equal(
    controlEffect([control({ text: "" }), control({ text: "Sign in" })], 0),
    "may-send",
  );
});

test("beside a submit control, a type=button control only may send the form", () => {
  const controls = [
    control({ text: "Show password", afterFields: false }),
    control({ submits: true, text: "Sign in" }),
    control({ text: "Log in" }),
  ];
  assert.equal(controlEffect(controls, 1), "sends");
  assert.equal(controlEffect(controls, 2), "may-send");
});

test("a control that cancels, closes, goes back or leads another way leaves the form unsent", () => {
  for (const text of [
    "Cancel",
    "Close",
    "Back",
    "Go back",
    "Forgot password?",
    "Sign in with Google",
    "Отмена",
    "Закрыть",
    "Назад",
    "Вернуться",
  ]) {
    assert.equal(
      controlEffect([control({ text: "Log in" }), control({ text })], 1),
      "unsent",
      text,
    );
  }
});

test("a form without a control that says to sign in is not submitted by a click", () => {
  assert.equal(
    submitControl([
      control({ text: "Show password" }),
      control({ text: "Sign up" }),
      control({ text: "Войти через Госуслуги" }),
      control({ text: "Forgot your password?" }),
      control({ text: "Sign in", enabled: false }),
    ]),
    -1,
  );
  assert.equal(submitControl([]), -1);
});
