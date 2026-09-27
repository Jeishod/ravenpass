declare const RAVENPASS_BUILD: Readonly<{
  version: string;
  build: string;
  /** Set when the build's commit is the one its version's release tag names. */
  released: boolean;
}>;
