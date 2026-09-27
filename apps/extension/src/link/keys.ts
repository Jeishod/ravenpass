/** Supplies the static X25519 pair made when linking and each handshake's ephemeral pair. */
export interface KeySource {
  generate(): Promise<CryptoKeyPair>;
}

/** The private key is non-extractable; the public key exports as 32 raw bytes. */
export class WebCryptoKeys implements KeySource {
  generate(): Promise<CryptoKeyPair> {
    return crypto.subtle.generateKey({ name: "X25519" }, false, ["deriveBits"]);
  }
}
