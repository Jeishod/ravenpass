import assert from "node:assert/strict";
import test from "node:test";
import { StoredLanguage } from "./language.ts";
import { StoredSignInStyle } from "./sign-in-style.ts";
import { MemoryLocalArea } from "./test-doubles.test-support.ts";

test("the card is the style until the desktop app reports one", async () => {
  const style = new StoredSignInStyle(new MemoryLocalArea());
  assert.equal(await style.signInStyle(), "card");
});

test("the reported style is kept until it is forgotten", async () => {
  const style = new StoredSignInStyle(new MemoryLocalArea());
  await style.keepSignInStyle("field");
  assert.equal(await style.signInStyle(), "field");
  await style.forgetSignInStyle();
  assert.equal(await style.signInStyle(), "card");
});

test("a kept style this build does not know reads as the card", async () => {
  const area = new MemoryLocalArea();
  await area.set({ desktopSignIn: "popup" });
  assert.equal(await new StoredSignInStyle(area).signInStyle(), "card");
});

test("the style is kept beside the desktop language without touching it", async () => {
  const area = new MemoryLocalArea();
  const language = new StoredLanguage({ area, browserLanguage: () => "en" });
  const style = new StoredSignInStyle(area);
  await language.keepDesktopLanguage("ru");
  await style.keepSignInStyle("field");
  await style.forgetSignInStyle();
  assert.equal((await language.getLanguage()).language, "ru");
  await style.keepSignInStyle("field");
  await language.forgetDesktopLanguage();
  assert.equal(await style.signInStyle(), "field");
});
