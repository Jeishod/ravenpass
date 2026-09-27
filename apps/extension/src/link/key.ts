import { base64urlnopad } from "@scure/base";

const encodedLength = 56;
const size = 42;
const header = [0x9a, 0x5c, 0x13];
const version = 1;
const versionOffset = 3;
const portOffset = 4;
const secretOffset = 6;
const checksumOffset = 38;
const minPort = 1024;

export type KeyRejection =
  | "length"
  | "encoding"
  | "header"
  | "version"
  | "checksum"
  | "port";

/** A text that is not a connection key, and the first check it failed. */
export class KeyFormatError extends Error {
  readonly reason: KeyRejection;

  constructor(reason: KeyRejection) {
    super(`This is not a Ravenpass connection key (${reason}).`);
    this.name = "KeyFormatError";
    this.reason = reason;
  }
}

/** Unpadded base64url of: header[3] version[1] port[2, big-endian] secret[32] SHA-256(preceding)[:4]. */
export class ConnectionKey {
  readonly port: number;
  readonly secret: Uint8Array<ArrayBuffer>;

  private constructor(port: number, secret: Uint8Array<ArrayBuffer>) {
    this.port = port;
    this.secret = secret;
  }

  /** Rejects with KeyFormatError. */
  static async parse(text: string): Promise<ConnectionKey> {
    const trimmed = text.trim();
    if (trimmed.length !== encodedLength) throw new KeyFormatError("length");
    let raw: Uint8Array<ArrayBuffer>;
    try {
      raw = Uint8Array.from(base64urlnopad.decode(trimmed));
    } catch {
      throw new KeyFormatError("encoding");
    }
    if (raw.length !== size) throw new KeyFormatError("encoding");
    if (header.some((byte, index) => raw[index] !== byte)) {
      throw new KeyFormatError("header");
    }
    if (raw[versionOffset] !== version) throw new KeyFormatError("version");
    const sum = new Uint8Array(
      await crypto.subtle.digest("SHA-256", raw.subarray(0, checksumOffset)),
    );
    if (
      raw.subarray(checksumOffset).some((byte, index) => byte !== sum[index])
    ) {
      throw new KeyFormatError("checksum");
    }
    const port = new DataView(raw.buffer).getUint16(portOffset);
    if (port < minPort) throw new KeyFormatError("port");
    return new ConnectionKey(port, raw.slice(secretOffset, checksumOffset));
  }
}
