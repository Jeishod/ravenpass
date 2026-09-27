import assert from "node:assert/strict";
import test from "node:test";
import type { Confirmation, UnlockMethods } from "../vault-api.ts";
import { pinReady, planCard } from "./card.ts";

const share: Confirmation = {
  id: "1",
  kind: "verify",
  reason: "share",
  file: "passport.pdf",
  identity: "Alex",
  site: "example.com",
  account: "",
};

const unlock: Confirmation = {
  id: "2",
  kind: "unlock",
  requester: "extension",
};

function methods(overrides: Partial<UnlockMethods>): UnlockMethods {
  return {
    biometryAvailable: true,
    biometryEnabled: true,
    pinSet: true,
    pinAttemptsLeft: 10,
    pinMinLength: 6,
    pinMaxLength: 12,
    ...overrides,
  };
}

test("a verification asks for the PIN in its reason's words", () => {
  assert.deepEqual(planCard(share, methods({})), {
    title: "sharing.prompt.title",
    description: "sharing.prompt.description",
    values: {
      file: "passport.pdf",
      identity: "Alex",
      site: "example.com",
      account: "",
    },
    pin: { confirm: "sharing.prompt.share" },
    biometry: null,
    recovery: false,
    noMethod: false,
    error: "sharing.prompt.error",
  });
  const saving = planCard(
    { ...share, reason: "save-passkey", file: "", identity: "" },
    null,
  );
  assert.equal(saving.title, "sharing.prompt.save-passkey.title");
  assert.deepEqual(saving.pin, {
    confirm: "sharing.prompt.save-passkey.confirm",
  });
  assert.equal(saving.error, "sharing.prompt.passkey.error");
});

test("a sign-in names its account only when it has one", () => {
  const named = planCard(
    { ...share, reason: "sign-in", account: "alex" },
    null,
  );
  assert.equal(named.description, "sharing.prompt.sign-in.description");
  assert.deepEqual(named.pin, { confirm: "sharing.prompt.sign-in.confirm" });
  const unnamed = planCard({ ...share, reason: "sign-in" }, null);
  assert.equal(
    unnamed.description,
    "sharing.prompt.sign-in.description.no-account",
  );
});

test("a fill from the extension names its site and account", () => {
  const filling = planCard(
    { ...share, reason: "fill", file: "", identity: "", account: "alex" },
    methods({}),
  );
  assert.equal(filling.title, "sharing.prompt.fill.title");
  assert.equal(filling.description, "sharing.prompt.fill.description");
  assert.deepEqual(filling.values, {
    file: "",
    identity: "",
    site: "example.com",
    account: "alex",
  });
  assert.deepEqual(filling.pin, { confirm: "sharing.prompt.fill.confirm" });
  assert.equal(filling.error, "sharing.prompt.passkey.error");
});

test("a change to how the vault unlocks asks for the current PIN", () => {
  const changing = planCard(
    {
      ...share,
      reason: "change-unlock",
      file: "",
      identity: "",
      site: "",
    },
    methods({ biometryEnabled: false }),
  );
  assert.equal(changing.title, "confirmation.change-unlock.title");
  assert.equal(changing.description, "confirmation.change-unlock.description");
  assert.deepEqual(changing.pin, {
    confirm: "confirmation.change-unlock.confirm",
  });
  assert.equal(changing.biometry, null);
  assert.equal(changing.recovery, false);
  assert.equal(changing.error, "confirmation.change-unlock.error");
});

test("an unlock offers what the vault opens with and the recovery key", () => {
  const both = planCard(unlock, methods({}));
  assert.equal(both.title, "confirmation.unlock.title");
  assert.deepEqual(both.pin, { confirm: "unlock.pin.action" });
  assert.equal(both.biometry, "unlock-methods.biometry.title");
  assert.equal(both.recovery, true);
  assert.equal(both.noMethod, false);
  assert.equal(both.error, "unlock.errors.failed");

  const macOnly = planCard(unlock, methods({ pinSet: false }));
  assert.equal(macOnly.pin, null);
  assert.equal(macOnly.biometry, "unlock.action");

  const pinOnly = planCard(unlock, methods({ biometryEnabled: false }));
  assert.deepEqual(pinOnly.pin, { confirm: "unlock.pin.action" });
  assert.equal(pinOnly.biometry, null);

  const unavailable = planCard(
    unlock,
    methods({ biometryAvailable: false, pinSet: false }),
  );
  assert.equal(unavailable.pin, null);
  assert.equal(unavailable.biometry, null);
  assert.equal(unavailable.noMethod, true);
  assert.equal(unavailable.recovery, true);
});

test("an unlock says what asks to open the vault", () => {
  assert.equal(
    planCard(unlock, null).description,
    "confirmation.unlock.description",
  );
  assert.equal(
    planCard({ ...unlock, requester: "autofill" }, null).description,
    "confirmation.unlock.description.autofill",
  );
});

test("an unlock offers nothing but recovery until its methods are known", () => {
  const loading = planCard(unlock, null);
  assert.equal(loading.pin, null);
  assert.equal(loading.biometry, null);
  assert.equal(loading.noMethod, false);
  assert.equal(loading.recovery, true);
});

test("a PIN is ready once it is as long as the vault asks", () => {
  assert.equal(pinReady("", null), false);
  assert.equal(pinReady("1", null), true);
  assert.equal(pinReady("12345", methods({})), false);
  assert.equal(pinReady("123456", methods({})), true);
});
