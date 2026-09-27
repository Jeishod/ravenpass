import assert from "node:assert/strict";
import test from "node:test";
import { maskitoTransform } from "@maskito/core";
import { phoneMask } from "./phone.ts";

// libphonenumber's example mobile number for Kazakhstan.
const kazakhMobile = "77710009998";

test("a phone is formatted as an international number", () => {
  assert.equal(
    maskitoTransform(`+${kazakhMobile}`, phoneMask),
    "+7 771 000 9998",
  );
  assert.equal(
    maskitoTransform("+442079460958", phoneMask),
    "+44 20 7946 0958",
  );
});

test("a number begun with a digit gets a leading plus", () => {
  assert.equal(maskitoTransform(kazakhMobile, phoneMask), "+7 771 000 9998");
  assert.equal(maskitoTransform("12125551234", phoneMask), "+1 212 555 1234");
});

test("a formatted phone is kept as shown", () => {
  assert.equal(
    maskitoTransform("+7 771 000 9998", phoneMask),
    "+7 771 000 9998",
  );
  assert.equal(maskitoTransform("", phoneMask), "");
});
