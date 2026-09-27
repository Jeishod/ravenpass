import { CircleAlert, LoaderCircle } from "lucide-react";
import trayIcon from "../../../../../apps/desktop/build/trayicon.png";
import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import type { StorageSyncState } from "../../vault-api.ts";

const syncStates: Record<StorageSyncState, MessageKey> = {
  synced: "workspace.status.sync.synced",
  writing: "workspace.status.sync.writing",
  failed: "workspace.status.sync.failed",
};

export function WorkspaceStatusBar({
  syncState,
  count,
}: {
  syncState: StorageSyncState;
  count: number;
}) {
  const { t } = useTranslator();
  const failed = syncState === "failed";

  return (
    <footer className="flex h-9 shrink-0 items-center gap-2 px-4 text-[11px] text-faint">
      {syncState === "writing" && (
        <LoaderCircle
          className="size-3 shrink-0 animate-spin motion-reduce:animate-none"
          aria-hidden="true"
        />
      )}
      {failed && <CircleAlert className="size-3 shrink-0 text-destructive" />}
      {syncState === "synced" && (
        <span
          className="size-1.5 shrink-0 rounded-full bg-muted-foreground"
          aria-hidden="true"
        />
      )}
      <span role="status" className={failed ? "text-destructive" : ""}>
        {t(syncStates[syncState])}
      </span>
      <span>{t("workspace.status.items", { count })}</span>
      <span className="ml-auto flex shrink-0 select-none items-center gap-1.5">
        <img
          src={trayIcon}
          alt=""
          draggable={false}
          className="size-3.5 object-contain opacity-50 invert"
        />
        <span>Ravenpass</span>
      </span>
    </footer>
  );
}
