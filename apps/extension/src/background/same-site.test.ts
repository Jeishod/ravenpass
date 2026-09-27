import assert from "node:assert/strict";
import test from "node:test";
import { sameSite } from "./same-site.ts";

test("origins of one registrable domain over https are the same site", () => {
  assert.ok(
    sameSite("https://secure.soundcloud.com", "https://soundcloud.com"),
  );
  assert.ok(
    sameSite("https://a.example.co.uk", "https://b.example.co.uk:8443"),
  );
});

test("other domains, private suffixes, plain http and hosts without a domain are not", () => {
  const pairs = [
    ["https://soundcloud.com", "https://soundcloud.example"],
    ["https://alice.github.io", "https://bob.github.io"],
    ["https://example.co.uk", "https://other.co.uk"],
    ["http://secure.soundcloud.com", "https://soundcloud.com"],
    ["https://secure.soundcloud.com", "http://soundcloud.com"],
    ["https://127.0.0.1", "https://127.0.0.1"],
    ["https://localhost", "https://localhost"],
    ["not an origin", "https://soundcloud.com"],
  ] as const;
  for (const [origin, other] of pairs) {
    assert.equal(sameSite(origin, other), false, `${origin} ${other}`);
  }
});
