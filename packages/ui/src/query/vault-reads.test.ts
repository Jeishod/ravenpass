import assert from "node:assert/strict";
import test from "node:test";
import { createQueryClient } from "./client.ts";
import { VaultRead } from "./vault-reads.ts";

const key = ["vault", "example"] as const;

test("options name the failure a failed read reports", () => {
  const read = new VaultRead(key, async () => [1], "workspace.error.list");
  assert.deepEqual(read.options().meta, { failure: "workspace.error.list" });
  assert.deepEqual(read.options().queryKey, key);
});

test("refresh puts the new read in the cache", async () => {
  const client = createQueryClient();
  const read = new VaultRead(
    key,
    async () => ["fresh"],
    "workspace.error.list",
  );
  client.setQueryData(key, ["stale"]);
  await read.refresh(client);
  assert.deepEqual(client.getQueryData(key), ["fresh"]);
});

test("a failed refresh rejects, keeps the cached data and leaves the report to the caller", async () => {
  const client = createQueryClient();
  const cause = new Error("unreadable");
  const read = new VaultRead(
    key,
    () => Promise.reject(cause),
    "workspace.error.list",
  );
  client.setQueryData(key, ["kept"]);
  const errors: unknown[] = [];
  const unwatch = client.getQueryCache().subscribe((event) => {
    if (event.type === "updated" && event.action.type === "error") {
      errors.push(event.action.error);
    }
  });
  await assert.rejects(read.refresh(client), cause);
  unwatch();
  assert.deepEqual(client.getQueryData(key), ["kept"]);
  assert.deepEqual(errors, []);
});

test("refresh replaces a read still in flight", async () => {
  const client = createQueryClient();
  const answers = [
    { items: ["first"], delay: 30 },
    { items: ["second"], delay: 5 },
  ];
  const read = new VaultRead(
    key,
    async () => {
      const answer = answers.shift();
      assert(answer);
      await new Promise((resolve) => setTimeout(resolve, answer.delay));
      return answer.items;
    },
    "workspace.error.list",
  );
  const first = client.fetchQuery(read.options());
  await read.refresh(client);
  await assert.rejects(first);
  assert.deepEqual(client.getQueryData(key), ["second"]);
});
