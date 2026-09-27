import type { SiteIcon } from "../vault-api.ts";

/** What the workspace knows of one site's icon: being asked for, arrived, or not offered. */
export type SiteIconState =
  | { kind: "pending" }
  | ({ kind: "icon" } & SiteIcon)
  | { kind: "none" };

/** SiteIconStore requests each normalized host at most once until cleared; a failed request settles as "none". */
export class SiteIconStore {
  private readonly load: (site: string) => Promise<SiteIcon>;
  private readonly sites = new Map<string, SiteIconState>();
  private readonly listeners = new Set<() => void>();
  private generation = 0;
  private version = 0;

  constructor(load: (site: string) => Promise<SiteIcon>) {
    this.load = load;
  }

  /** request asks the host for a site's icon once; an empty site is ignored. */
  request(site: string): void {
    if (!site || this.sites.has(site)) return;
    this.update(site, { kind: "pending" });
    const generation = this.generation;
    this.load(site).then(
      (icon) => this.settle(generation, site, icon),
      () => this.settle(generation, site, null),
    );
  }

  /** state answers what is known of a site's icon, or undefined for a site not asked for. */
  state(site: string): SiteIconState | undefined {
    return this.sites.get(site);
  }

  /** subscribe calls the listener on every change and returns the unsubscribe function. */
  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }

  /** snapshot answers a number that changes whenever what `state` answers may have changed. */
  snapshot(): number {
    return this.version;
  }

  /** clear forgets every site and drops answers to earlier requests. */
  clear(): void {
    this.generation += 1;
    if (!this.sites.size) return;
    this.sites.clear();
    this.changed();
  }

  private settle(
    generation: number,
    site: string,
    icon: SiteIcon | null,
  ): void {
    if (generation !== this.generation) return;
    this.update(
      site,
      icon?.image ? { kind: "icon", ...icon } : { kind: "none" },
    );
  }

  private update(site: string, state: SiteIconState): void {
    this.sites.set(site, state);
    this.changed();
  }

  private changed(): void {
    this.version += 1;
    for (const listener of this.listeners) listener();
  }
}
