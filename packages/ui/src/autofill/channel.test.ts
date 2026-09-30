import assert from "node:assert/strict";
import test from "node:test";
import { type Answer, AutofillChannel, hostOf } from "./channel.ts";

class RecordingNative {
  readonly posted: [number, unknown][] = [];

  post(id: number, request: string): void {
    this.posted.push([id, JSON.parse(request)]);
  }
}

test("each answer reaches the request it belongs to, in whatever order it arrives", async () => {
  const native = new RecordingNative();
  const channel = new AutofillChannel(native);
  const first = channel.request("search", { query: "mail" });
  const second = channel.request("icon", { site: "example.com" });
  assert.deepEqual(native.posted, [
    [1, { query: "mail", op: "search" }],
    [2, { site: "example.com", op: "icon" }],
  ]);
  channel.answer(2, '{"status":"ok","image":"iVBOR"}');
  channel.answer(1, '{"status":"locked"}');
  assert.deepEqual(await second, { status: "ok", image: "iVBOR" });
  assert.deepEqual(await first, { status: "locked" });
});

test("a request's own fields cannot rename its operation", () => {
  const native = new RecordingNative();
  void new AutofillChannel(native).request("search", { op: "unlock" });
  assert.deepEqual(native.posted, [[1, { op: "search" }]]);
});

test("an answer that is not an object with a status reads as failed", async () => {
  const channel = new AutofillChannel(new RecordingNative());
  const answers = ["not json", "[]", "null", '{"ok":true}', '{"status":1}'].map(
    (text) => {
      const answer = channel.request("open");
      return { text, answer };
    },
  );
  answers.forEach(({ text }, index) => {
    channel.answer(index + 1, text);
  });
  for (const { answer } of answers) {
    assert.deepEqual(await answer, { status: "failed" });
  }
});

test("a notice takes no answer, and an answer to no request is dropped", () => {
  const native = new RecordingNative();
  const channel = new AutofillChannel(native);
  channel.notify("ready");
  channel.answer(7, '{"status":"ok"}');
  assert.deepEqual(native.posted, [[0, { op: "ready" }]]);
});

test("the host's notices reach each watcher until it stops, and one that is no object with a status is dropped", () => {
  const channel = new AutofillChannel(new RecordingNative());
  const seen: Answer[] = [];
  const stop = channel.watch((notice) => seen.push(notice));
  channel.notice('{"status":"wait","wait":"opening"}');
  channel.notice("not json");
  channel.notice('{"wait":"opening"}');
  stop();
  channel.notice('{"status":"wait","wait":"unlocking"}');
  assert.deepEqual(seen, [{ status: "wait", wait: "opening" }]);
});

test("the phone takes requests as text through its listener and shows the page as sheets", () => {
  const messages: string[] = [];
  const { native, surface } = hostOf({
    ravenpassAutofill: { postMessage: (message) => messages.push(message) },
  });
  native.post(1, '{"op":"open"}');
  assert.equal(surface, "sheet");
  assert.deepEqual(
    messages.map((message) => JSON.parse(message)),
    [{ id: 1, request: '{"op":"open"}' }],
  );
});

test("the macOS extension takes requests through its message handler and shows the page as its window", () => {
  const messages: unknown[] = [];
  const { native, surface } = hostOf({
    webkit: {
      messageHandlers: {
        ravenpassAutofill: { postMessage: (message) => messages.push(message) },
      },
    },
  });
  native.post(2, '{"op":"search"}');
  assert.equal(surface, "window");
  assert.deepEqual(messages, [{ id: 2, request: '{"op":"search"}' }]);
});

test("a page no host opened cannot start", () => {
  assert.throws(() => hostOf({}));
});
