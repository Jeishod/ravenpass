import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useSyncExternalStore,
} from "react";
import type {
  SiteIconState,
  SiteIconStore,
} from "../../workspace/site-icons.ts";

const SiteIconContext = createContext<SiteIconStore | null>(null);

/** SiteIconsProvider supplies the icon store; `null` turns site icons off. */
export function SiteIconsProvider({
  store,
  children,
}: {
  store: SiteIconStore | null;
  children: ReactNode;
}) {
  return (
    <SiteIconContext.Provider value={store}>
      {children}
    </SiteIconContext.Provider>
  );
}

/** Hosts cannot contain a line break, so one string carries the list as an effect dependency. */
const separator = "\n";

/** useSiteIcons requests the sites' icons and returns a lookup of what is known of each. */
export function useSiteIcons(
  sites: readonly string[],
): (site: string) => SiteIconState | undefined {
  const store = useContext(SiteIconContext);
  const subscribe = useCallback(
    (listener: () => void) => store?.subscribe(listener) ?? (() => {}),
    [store],
  );
  const snapshot = useCallback(() => store?.snapshot() ?? 0, [store]);
  useSyncExternalStore(subscribe, snapshot);
  const requested = sites.join(separator);

  useEffect(() => {
    if (!store) return;
    for (const site of requested.split(separator)) store.request(site);
  }, [store, requested]);

  return (site) => store?.state(site);
}

/** useSiteIcon requests one site's icon only while `wanted`; a known icon returns either way. */
export function useSiteIcon(
  site: string,
  wanted = true,
): SiteIconState | undefined {
  return useSiteIcons(wanted ? [site] : [])(site);
}
