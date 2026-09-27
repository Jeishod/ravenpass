import { AccessShell } from "@ravenpass/ui/components/AccessShell.tsx";
import { Block, BlockRow } from "@ravenpass/ui/components/Block.tsx";
import { ConfirmDialog } from "@ravenpass/ui/components/ConfirmDialog.tsx";
import { Button } from "@ravenpass/ui/components/ui/button.tsx";
import type { MessageKey } from "@ravenpass/ui/i18n/messages.ts";
import { useTranslator } from "@ravenpass/ui/i18n/translator.tsx";
import { CalendarDate } from "@ravenpass/ui/identities/dates.ts";
import { Plug } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import type { DesktopState, LinkStatus } from "../link/client.ts";
import { ask } from "../messages.ts";
import { ChromeAutofillSetting } from "./ChromeAutofillSetting.tsx";

export type LinkedStatus = Extract<LinkStatus, { linked: true }>;

const desktopStates: Record<DesktopState, MessageKey> = {
  unlocked: "extension.linked.desktop.unlocked",
  locked: "extension.linked.desktop.locked",
  "not-open": "extension.linked.desktop.not-open",
};

export function LinkedScreen({
  status,
  onUnlinked,
}: {
  status: LinkedStatus;
  onUnlinked: () => void;
}) {
  const { t, language } = useTranslator();
  const [confirming, setConfirming] = useState(false);
  const [unlinking, setUnlinking] = useState(false);

  async function unlink() {
    setUnlinking(true);
    try {
      await ask({ kind: "unlink" });
      onUnlinked();
    } catch {
      toast.error(t("extension.unlink.error"));
      setUnlinking(false);
      setConfirming(false);
    }
  }

  const unlocked = status.desktop === "unlocked";

  return (
    <AccessShell
      layout="popup"
      languageMenu={false}
      icon={Plug}
      title={t("extension.linked.title")}
      description={t("extension.linked.description")}
    >
      <Block className="text-left">
        <BlockRow
          title={t("extension.linked.desktop")}
          wrap={!unlocked}
          detail={
            unlocked ? (
              <span className="flex items-center gap-1.5">
                <span
                  className="size-1.5 shrink-0 rounded-full bg-muted-foreground"
                  aria-hidden="true"
                />
                {t(desktopStates.unlocked)}
              </span>
            ) : (
              t(desktopStates[status.desktop])
            )
          }
        />
        <BlockRow
          title={t("extension.linked.since")}
          detail={CalendarDate.of(new Date(status.linkedAt)).format(language)}
        />
      </Block>
      <ChromeAutofillSetting />
      <Button
        type="button"
        variant="quiet"
        size="pill"
        className="mt-3.5 w-full"
        onClick={() => setConfirming(true)}
      >
        {t("extension.unlink.action")}
      </Button>
      <ConfirmDialog
        open={confirming}
        title={t("extension.unlink.title")}
        detail={t("extension.unlink.detail")}
        confirm={t("extension.unlink.action")}
        cancel={t("extension.unlink.cancel")}
        destructive
        busy={unlinking}
        onConfirm={() => void unlink()}
        onCancel={() => setConfirming(false)}
      />
    </AccessShell>
  );
}
