import assert from "node:assert/strict";
import test from "node:test";
import { setTimeout as delay } from "node:timers/promises";
import { MutationObserver } from "@tanstack/react-query";
import {
  createQueryClient,
  forgetVaultEdits,
  forgetVaultReads,
} from "./client.ts";
import { queryKeys } from "./keys.ts";

const pin = "482913";

test("a settled mutation leaves memory once nothing shows it", async () => {
  const client = createQueryClient();
  const observer = new MutationObserver(client, {
    mutationFn: async (entered: string) => entered.length,
  });

  await observer.mutate(pin);
  observer.reset();
  await delay(1);

  assert.equal(client.getMutationCache().getAll().length, 0);
});

test("forgetting the vault drops every mutation at once and vault queries after, and keeps host queries", () => {
  const client = createQueryClient();
  const observer = new MutationObserver(client, {
    mutationFn: (_entered: string) => new Promise<void>(() => {}),
  });
  void observer.mutate(pin);
  client.setQueryData(queryKeys.credentials, ["github.com"]);
  client.setQueryData(queryKeys.capabilities, { autofill: true });

  forgetVaultEdits(client);
  assert.equal(client.getMutationCache().getAll().length, 0);
  assert.deepEqual(client.getQueryData(queryKeys.credentials), ["github.com"]);

  forgetVaultReads(client);
  assert.equal(client.getQueryData(queryKeys.credentials), undefined);
  assert.deepEqual(client.getQueryData(queryKeys.capabilities), {
    autofill: true,
  });
});
