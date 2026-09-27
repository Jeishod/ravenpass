import assert from "node:assert/strict";
import test from "node:test";
import type { VirtualItem } from "@tanstack/react-virtual";
import { ListPositions } from "./list-positions.ts";

const measured: VirtualItem[] = [
  { index: 0, key: "a", start: 0, size: 53, end: 53, lane: 0 },
  { index: 1, key: "b", start: 57, size: 53, end: 110, lane: 0 },
];

test("a listing never left starts at its top", () => {
  const positions = new ListPositions();
  assert.deepEqual(positions.listing("passwords", "all", "", "").recall(), {
    offset: 0,
    measured: [],
  });
});

test("a listing returns to where it was left", () => {
  const positions = new ListPositions();
  positions.listing("passwords", "all", "", "").remember({
    offset: 480,
    measured,
  });
  assert.deepEqual(positions.listing("passwords", "all", "", "").recall(), {
    offset: 480,
    measured,
  });
});

test("each place, section and group keeps a position of its own", () => {
  const positions = new ListPositions();
  positions.listing("passwords", "all", "", "").remember({
    offset: 480,
    measured,
  });
  positions.listing("passwords", "pinned", "", "").remember({
    offset: 120,
    measured: [],
  });
  assert.equal(
    positions.listing("passwords", "all", "", "").recall().offset,
    480,
  );
  assert.equal(
    positions.listing("passwords", "pinned", "", "").recall().offset,
    120,
  );
  assert.equal(positions.listing("cards", "all", "", "").recall().offset, 0);
  assert.equal(
    positions.listing("passwords", "all", "work", "").recall().offset,
    0,
  );
});

test("a position returns only under the search it was left with", () => {
  const positions = new ListPositions();
  positions.listing("notes", "all", "", "bank").remember({
    offset: 300,
    measured,
  });
  assert.equal(
    positions.listing("notes", "all", "", "bank").recall().offset,
    300,
  );
  assert.deepEqual(positions.listing("notes", "all", "", "").recall(), {
    offset: 0,
    measured: [],
  });
});

test("a listing left at its top forgets where it was before", () => {
  const positions = new ListPositions();
  const listing = positions.listing("seeds", "recent", "", "");
  listing.remember({ offset: 200, measured });
  listing.remember({ offset: 0, measured });
  assert.deepEqual(listing.recall(), { offset: 0, measured: [] });
});

test("the same listing has the same key whatever its search", () => {
  const positions = new ListPositions();
  assert.equal(
    positions.listing("passwords", "all", "work", "").key,
    positions.listing("passwords", "all", "work", "bank").key,
  );
  assert.notEqual(
    positions.listing("passwords", "all", "work", "").key,
    positions.listing("passwords", "recent", "work", "").key,
  );
});

test("names that hold a separator do not make two listings one", () => {
  const positions = new ListPositions();
  assert.notEqual(
    positions.listing("x,all,y", "all", "z", "").key,
    positions.listing("x", "all", "y,all,z", "").key,
  );
});
