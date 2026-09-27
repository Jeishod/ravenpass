import assert from "node:assert/strict";
import test from "node:test";
import type { VaultState } from "../vault-api.ts";
import { openingPhase } from "./opening.ts";

function host(phase: VaultState["phase"], knowsVault: boolean | Error) {
  let questions = 0;
  return {
    api: {
      getState: async () => ({ phase }),
      knowsVault: async () => {
        questions++;
        if (knowsVault instanceof Error) throw knowsVault;
        return knowsVault;
      },
    },
    questions: () => questions,
  };
}

test("setup on a device that holds no vault opens on the introduction", async () => {
  const { api } = host("setup", false);
  assert.equal(await openingPhase(api), "intro");
});

test("setup opens directly while another vault exists", async () => {
  const { api } = host("setup", true);
  assert.equal(await openingPhase(api), "setup");
});

test("an unanswered question leaves the introduction out", async () => {
  const { api } = host("setup", new Error("unreadable"));
  assert.equal(await openingPhase(api), "setup");
});

test("every other phase opens as it is, without asking", async () => {
  for (const phase of ["storage", "locked", "ready"] as const) {
    const { api, questions } = host(phase, false);
    assert.equal(await openingPhase(api), phase);
    assert.equal(questions(), 0);
  }
});
