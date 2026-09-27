import { useMutation } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import type { AutofillApi, AutofillWait } from "../../autofill/autofill-api.ts";
import { SiteIconStore } from "../../workspace/site-icons.ts";
import { SiteIconsProvider } from "../workspace/SiteIcons.tsx";
import { CoveredSheet } from "./AutofillSheet.tsx";
import { SaveSheet } from "./SaveSheet.tsx";
import { SearchSheet } from "./SearchSheet.tsx";
import { UnlockSheet } from "./UnlockSheet.tsx";
import { WaitSheet } from "./WaitSheet.tsx";

/** AutofillView shows the screen the host opens, or its wait in the screen's place; a screen that cannot open ends. */
export function AutofillView({ api }: { api: AutofillApi }) {
  const [wait, setWait] = useState<AutofillWait | null>(null);
  const [icons] = useState(
    () => new SiteIconStore((site) => api.siteIcon(site)),
  );
  const { mutate: open, data: opening } = useMutation({
    mutationFn: () => api.open(),
    onError: () => api.cancel(),
  });

  useEffect(() => {
    // Watched before the page reports ready, which is when the host sends its first wait.
    const unwatch = api.watchWait(setWait);
    api.ready();
    open();
    return unwatch;
  }, [api, open]);

  return (
    <SiteIconsProvider store={icons}>
      {wait && <WaitSheet wait={wait} onCancel={() => api.cancel()} />}
      {opening && (
        <CoveredSheet covered={wait !== null}>
          {opening.screen === "search" && (
            <SearchSheet api={api} opening={opening} />
          )}
          {opening.screen === "unlock" && (
            <UnlockSheet api={api} opening={opening} />
          )}
          {opening.screen === "save" && (
            <SaveSheet api={api} opening={opening} />
          )}
        </CoveredSheet>
      )}
    </SiteIconsProvider>
  );
}
