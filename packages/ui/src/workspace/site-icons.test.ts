import assert from "node:assert/strict";
import test from "node:test";
import type { SiteIcon } from "../vault-api.ts";
import { SiteIconStore } from "./site-icons.ts";

/** A loader whose answers the test releases by hand. */
function deferredLoader() {
  const calls: string[] = [];
  const pending = new Map<
    string,
    { resolve: (icon: SiteIcon) => void; reject: (cause: Error) => void }
  >();
  const load = (site: string) => {
    calls.push(site);
    return new Promise<SiteIcon>((resolve, reject) => {
      pending.set(site, { resolve, reject });
    });
  };
  const answer = async (site: string, image: string) => {
    pending.get(site)?.resolve({ image, tint: image ? "#e67814" : "" });
    await Promise.resolve();
  };
  const fail = async (site: string) => {
    pending.get(site)?.reject(new Error("unreachable"));
    await Promise.resolve();
  };
  return { calls, load, answer, fail };
}

test("a host is asked for once however often it is requested", async () => {
  const loader = deferredLoader();
  const store = new SiteIconStore(loader.load);
  store.request("google.com");
  store.request("google.com");
  store.request("github.com");
  await loader.answer("google.com", "iVBORw0KGgo");
  store.request("google.com");
  assert.deepEqual(loader.calls, ["google.com", "github.com"]);
});

test("an empty site is never asked for", () => {
  const loader = deferredLoader();
  const store = new SiteIconStore(loader.load);
  store.request("");
  assert.deepEqual(loader.calls, []);
  assert.equal(store.state(""), undefined);
});

test("a site is pending until its icon arrives, and listeners hear both", async () => {
  const loader = deferredLoader();
  const store = new SiteIconStore(loader.load);
  let heard = 0;
  store.subscribe(() => {
    heard += 1;
  });
  const before = store.snapshot();
  store.request("google.com");
  assert.deepEqual(store.state("google.com"), { kind: "pending" });
  assert.equal(heard, 1);
  await loader.answer("google.com", "iVBORw0KGgo");
  assert.deepEqual(store.state("google.com"), {
    kind: "icon",
    image: "iVBORw0KGgo",
    tint: "#e67814",
  });
  assert.equal(heard, 2);
  assert.notEqual(store.snapshot(), before);
});

test("a site without an icon, or a failed request, is not asked for again", async () => {
  const loader = deferredLoader();
  const store = new SiteIconStore(loader.load);
  store.request("plain.example");
  store.request("down.example");
  await loader.answer("plain.example", "");
  await loader.fail("down.example");
  store.request("plain.example");
  store.request("down.example");
  assert.deepEqual(store.state("plain.example"), { kind: "none" });
  assert.deepEqual(store.state("down.example"), { kind: "none" });
  assert.deepEqual(loader.calls, ["plain.example", "down.example"]);
});

test("clearing forgets icons, drops late answers and asks again", async () => {
  const loader = deferredLoader();
  const store = new SiteIconStore(loader.load);
  store.request("google.com");
  store.request("github.com");
  await loader.answer("google.com", "iVBORw0KGgo");
  let heard = 0;
  store.subscribe(() => {
    heard += 1;
  });
  store.clear();
  assert.equal(heard, 1);
  assert.equal(store.state("google.com"), undefined);
  await loader.answer("github.com", "iVBORw0KGgo");
  assert.equal(store.state("github.com"), undefined);
  assert.equal(heard, 1);
  store.request("github.com");
  assert.deepEqual(loader.calls, ["google.com", "github.com", "github.com"]);
});

test("a listener that unsubscribed hears nothing more", async () => {
  const loader = deferredLoader();
  const store = new SiteIconStore(loader.load);
  let heard = 0;
  const stop = store.subscribe(() => {
    heard += 1;
  });
  stop();
  store.request("google.com");
  await loader.answer("google.com", "iVBORw0KGgo");
  assert.equal(heard, 0);
});
