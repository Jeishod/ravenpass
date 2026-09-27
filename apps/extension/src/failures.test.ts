import assert from "node:assert/strict";
import test from "node:test";
import { logRejection } from "./failures.ts";

const failure = "Ravenpass could not build its upload menu.";

test("a task that settles logs nothing", async (t) => {
  const errors = t.mock.method(console, "error", () => {});
  await logRejection(Promise.resolve("done"), failure);
  assert.equal(errors.mock.callCount(), 0);
});

test("a rejected task resolves and logs the failure with the error's name once", async (t) => {
  const errors = t.mock.method(console, "error", () => {});
  await logRejection(
    Promise.reject(new TypeError("Invalid menu for https://example.com.")),
    failure,
  );
  assert.deepEqual(
    errors.mock.calls.map((call) => call.arguments),
    [[failure, "TypeError"]],
  );
});

test("a rejection that is not an Error logs its type, never its value", async (t) => {
  const errors = t.mock.method(console, "error", () => {});
  await logRejection(Promise.reject("alex@example.com"), failure);
  assert.deepEqual(
    errors.mock.calls.map((call) => call.arguments),
    [[failure, "string"]],
  );
});
