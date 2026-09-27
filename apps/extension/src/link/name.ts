export interface UserAgentData {
  readonly brands: readonly { readonly brand: string }[];
  readonly platform: string;
}

declare global {
  interface Navigator {
    /** TypeScript's DOM library does not declare User-Agent Client Hints. */
    readonly userAgentData?: UserAgentData;
  }
}

/** The desktop app's limit, in code points. */
const maxNameLength = 64;

// Chromium lists a made-up brand, such as "Not A(Brand", beside the real ones.
const madeUpBrand = /not.a.brand/i;
const vendor = /^(Google|Microsoft) /;

/** The name the desktop app lists the extension under, such as `Chrome · macOS`. */
export function linkName(agent: UserAgentData | undefined): string {
  const brands =
    agent?.brands
      .map(({ brand }) => brand)
      .filter((brand) => !madeUpBrand.test(brand)) ?? [];
  const brand = brands.find((name) => name !== "Chromium") ?? brands[0];
  const name = [(brand ?? "Chrome").replace(vendor, ""), agent?.platform]
    .filter(Boolean)
    .join(" · ");
  return Array.from(name).slice(0, maxNameLength).join("");
}
