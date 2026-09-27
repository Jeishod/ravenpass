import assert from "node:assert/strict";
import test from "node:test";
import {
  type CreateOptions,
  createOptionsOf,
  type GetOptions,
  getOptionsOf,
  type PageCreationOptions,
  type PageRequest,
  readPageRequest,
  servable,
  timeoutMs,
  verifies,
} from "./requests.ts";

const bytes = (...values: number[]) => new Uint8Array(values);

const creation: PageCreationOptions = {
  rp: { id: "example.com", name: "Example" },
  user: {
    id: bytes(0xfb, 0xff, 0x01),
    name: "alex@example.com",
    displayName: "Alex",
  },
  challenge: bytes(1, 2, 3, 4).buffer,
  pubKeyCredParams: [
    { type: "public-key", alg: -8 },
    { type: "public-key", alg: -7 },
    { type: "other" as PublicKeyCredentialType, alg: -257 },
  ],
  timeout: 60_000,
  excludeCredentials: [
    { type: "public-key", id: bytes(9, 9) },
    { type: "public-key", id: new DataView(bytes(0, 7, 8, 0).buffer, 1, 2) },
  ],
  authenticatorSelection: {
    authenticatorAttachment: "platform",
    residentKey: "required",
    userVerification: "required",
  },
  attestation: "direct",
  hints: ["client-device"],
  extensions: { credProps: true, prf: {} },
};

const created: CreateOptions = {
  rpId: "example.com",
  rpName: "Example",
  user: { id: "-_8B", name: "alex@example.com", displayName: "Alex" },
  challenge: "AQIDBA",
  algorithms: [-8, -7],
  timeout: 60_000,
  exclude: ["CQk", "Bwg"],
  attachment: "platform",
  userVerification: "required",
  hints: ["client-device"],
  credProps: true,
};

const got: GetOptions = {
  rpId: "example.com",
  challenge: "BQY",
  timeout: 30_000,
  allow: ["AQ", "Ag"],
  userVerification: "discouraged",
  hints: ["hybrid"],
};

test("creation options become plain data with byte fields in unpadded base64url", () => {
  assert.deepEqual(createOptionsOf(creation), created);
});

test("request options become plain data with the allow list's IDs", () => {
  assert.deepEqual(
    getOptionsOf({
      rpId: "example.com",
      challenge: bytes(5, 6),
      timeout: 30_000,
      allowCredentials: [
        { type: "public-key", id: bytes(1), transports: ["internal"] },
        { type: "public-key", id: bytes(2).buffer },
      ],
      userVerification: "discouraged",
      hints: ["hybrid"],
    }),
    got,
  );
});

test("options the page leaves out read as WebAuthn's defaults", () => {
  const options = createOptionsOf({
    rp: { name: "Example" },
    user: { id: bytes(1), name: "alex", displayName: "" },
    challenge: bytes(1),
    pubKeyCredParams: [],
  });
  assert.equal(options.rpId, null);
  assert.equal(options.timeout, null);
  assert.deepEqual(options.exclude, []);
  assert.equal(options.attachment, null);
  assert.equal(options.userVerification, "preferred");
  assert.deepEqual(options.hints, []);
  assert.equal(options.credProps, false);
  assert.deepEqual(getOptionsOf({ challenge: bytes(1) }), {
    rpId: null,
    challenge: "AQ",
    timeout: null,
    allow: [],
    userVerification: "preferred",
    hints: [],
  });
});

test("options WebAuthn refuses throw a TypeError", () => {
  const { challenge: _challenge, ...withoutChallenge } = creation;
  assert.throws(
    () => createOptionsOf(withoutChallenge as PageCreationOptions),
    TypeError,
  );
  assert.throws(
    () =>
      createOptionsOf({
        ...creation,
        user: {
          id: bytes(1),
          displayName: "",
        } as unknown as PublicKeyCredentialUserEntity,
      }),
    TypeError,
  );
  assert.throws(
    () => getOptionsOf({ challenge: "AQ" as unknown as BufferSource }),
    TypeError,
  );
});

test("a request survives a message whole, and only its fields are kept", () => {
  const requests: PageRequest[] = [
    { mode: "create", options: created },
    { mode: "get", options: got, conditional: false },
    { mode: "get", options: got, conditional: true },
  ];
  for (const request of requests) {
    assert.deepEqual(readPageRequest(structuredClone(request)), request);
  }
  assert.deepEqual(
    readPageRequest({
      mode: "get",
      conditional: true,
      options: { ...got, origin: "https://evil.example" },
      extra: 1,
    }),
    { mode: "get", options: got, conditional: true },
  );
});

test("a malformed request reads as nothing", () => {
  for (const value of [
    null,
    "create",
    { mode: "delete", options: got },
    { mode: "get", options: got },
    {
      mode: "get",
      options: { ...got, challenge: "not base64url!" },
      conditional: false,
    },
    { mode: "get", options: { ...got, allow: [1] }, conditional: false },
    {
      mode: "create",
      options: { ...created, user: { ...created.user, id: 7 } },
    },
    { mode: "create", options: { ...created, algorithms: ["-7"] } },
    { mode: "create", options: { ...created, credProps: "yes" } },
    { mode: "create", options: { ...created, timeout: Number.NaN } },
  ]) {
    assert.equal(readPageRequest(value), null);
  }
});

test("a request read from a message drops unknown fields at every level", () => {
  assert.deepEqual(
    readPageRequest({
      mode: "create",
      options: {
        ...created,
        user: { ...created.user, icon: "x" },
        attestation: "direct",
      },
    }),
    { mode: "create", options: created },
  );
});

test("a request's numbers must be finite, its algorithms safe integers, and its optional text null rather than missing", () => {
  const { rpId: _rpId, ...withoutRpId } = got;
  for (const value of [
    { mode: "create", options: { ...created, algorithms: [-7.5] } },
    { mode: "create", options: { ...created, algorithms: [2 ** 53] } },
    {
      mode: "create",
      options: { ...created, timeout: Number.POSITIVE_INFINITY },
    },
    { mode: "create", options: { ...created, algorithms: [-7, Number.NaN] } },
    {
      mode: "create",
      options: { ...created, algorithms: [Number.NEGATIVE_INFINITY] },
    },
    { mode: "create", options: { ...created, attachment: undefined } },
    { mode: "get", options: withoutRpId, conditional: false },
    { mode: "get", options: { ...got, rpId: undefined }, conditional: false },
    {
      mode: "get",
      options: { ...got, timeout: undefined },
      conditional: false,
    },
    {
      mode: "get",
      options: { ...got, allow: ["AQ", "not base64url!"] },
      conditional: false,
    },
    {
      mode: "get",
      options: { ...got, hints: ["hybrid", 1] },
      conditional: false,
    },
    { mode: "get", options: got, conditional: "true" },
    { mode: "Get", options: got, conditional: false },
  ]) {
    assert.equal(readPageRequest(structuredClone(value)), null);
  }
  const unnamed = { ...got, rpId: null, timeout: null };
  assert.deepEqual(
    readPageRequest({ mode: "get", options: unnamed, conditional: false }),
    { mode: "get", options: unnamed, conditional: false },
  );
});

test("Ravenpass serves ES256 on a platform or unnamed authenticator, and sign-ins, unless only a security key is hinted", () => {
  const create = (options: Partial<CreateOptions>): PageRequest => ({
    mode: "create",
    options: { ...created, ...options },
  });
  const get = (options: Partial<GetOptions>): PageRequest => ({
    mode: "get",
    options: { ...got, ...options },
    conditional: false,
  });
  assert.equal(servable(create({})), true);
  assert.equal(servable(create({ algorithms: [] })), true);
  assert.equal(servable(create({ attachment: null })), true);
  assert.equal(servable(get({})), true);
  assert.equal(servable(get({ hints: ["security-key", "hybrid"] })), true);

  assert.equal(servable(create({ algorithms: [-257, -8] })), false);
  assert.equal(servable(create({ attachment: "cross-platform" })), false);
  assert.equal(servable(create({ hints: ["security-key"] })), false);
  assert.equal(servable(get({ hints: ["security-key"] })), false);
});

test("the person is verified unless the page discourages it", () => {
  assert.equal(verifies(created), true);
  assert.equal(verifies({ ...created, userVerification: "preferred" }), true);
  assert.equal(verifies(got), false);
});

test("a request waits five minutes by default, and the page's timeout within Chrome's bounds", () => {
  assert.equal(timeoutMs({ ...got, timeout: null }), 300_000);
  assert.equal(timeoutMs({ ...got, timeout: 60_000 }), 60_000);
  assert.equal(timeoutMs({ ...got, timeout: 0 }), 10_000);
  assert.equal(timeoutMs({ ...got, timeout: 3_600_000 }), 600_000);
});
