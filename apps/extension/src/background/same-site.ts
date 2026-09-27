import { getDomain } from "tldts";

/** The private section keeps `alice.github.io` and `bob.github.io` apart, as Chrome's schemeful site does. */
const suffixes = { allowPrivateDomains: true };

/** Whether two `https:` origins share a registrable domain; a host without one, such as an IP address, shares none. */
export function sameSite(origin: string, other: string): boolean {
  if (!URL.canParse(origin) || !URL.canParse(other)) return false;
  const first = new URL(origin);
  const second = new URL(other);
  if (first.protocol !== "https:" || second.protocol !== "https:") {
    return false;
  }
  const domain = getDomain(first.hostname, suffixes);
  return domain !== null && domain === getDomain(second.hostname, suffixes);
}
