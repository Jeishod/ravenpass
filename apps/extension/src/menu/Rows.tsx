import { Button } from "@ravenpass/ui/components/ui/button.tsx";
import { Kbd } from "@ravenpass/ui/components/ui/kbd.tsx";
import { Avatar } from "@ravenpass/ui/components/workspace/Avatar.tsx";
import {
  ItemRowContent,
  itemRowClass,
} from "@ravenpass/ui/components/workspace/ItemRowContent.tsx";
import type { MessageKey } from "@ravenpass/ui/i18n/messages.ts";
import { useTranslator } from "@ravenpass/ui/i18n/translator.tsx";
import { cn } from "cn";
import {
  ChevronRight,
  Fingerprint,
  Lock,
  type LucideIcon,
  MonitorOff,
  RectangleEllipsis,
} from "lucide-react";
import type { ReactNode } from "react";
import type { ShareProgress } from "../link/client.ts";
import { toolbarIcon } from "../toolbar/icon.ts";

export function StateRow({
  icon,
  title,
  detail,
  action,
}: {
  icon: LucideIcon;
  title: string;
  detail: string;
  action?: ReactNode;
}) {
  return (
    <div className={itemRowClass(false, "menu")}>
      <ItemRowContent
        active={false}
        avatar={
          <Avatar
            label=""
            icon={icon}
            size="row"
            shape="tile"
            emphasis="none"
          />
        }
        title={title}
        detail={{ text: detail }}
        trailing={action}
        wrap
        look="menu"
      />
    </div>
  );
}

const verificationIcons: Record<ShareProgress, LucideIcon> = {
  "confirm-on-device": Fingerprint,
  "confirm-in-ravenpass": RectangleEllipsis,
};

/** What Ravenpass asks of the person while a file share, a passkey or a fill waits. */
export function VerificationRow({
  progress,
  subject,
}: {
  progress: ShareProgress;
  subject: "upload" | "passkey" | "fill";
}) {
  const { t } = useTranslator();
  return (
    <div role="status">
      <StateRow
        icon={verificationIcons[progress]}
        title={t(`extension.${subject}.${progress}.title`)}
        detail={t(`extension.${subject}.${progress}.detail`)}
      />
    </div>
  );
}

export function LockedRow({
  detail,
  onUnlock,
}: {
  detail: string;
  onUnlock: () => void;
}) {
  const { t } = useTranslator();
  return (
    <div className="grid gap-1">
      <StateRow
        icon={Lock}
        title={t("extension.menu.locked.title")}
        detail={detail}
      />
      <div className="px-2 pb-1.5">
        <Button
          type="button"
          variant="raised"
          size="pill-sm"
          className="w-full"
          data-menu-item
          onClick={onUnlock}
        >
          {t("extension.menu.locked.action")}
        </Button>
      </div>
    </div>
  );
}

export function NotOpenRow({ detail }: { detail: string }) {
  const { t } = useTranslator();
  return (
    <StateRow
      icon={MonitorOff}
      title={t("extension.menu.not-open.title")}
      detail={detail}
    />
  );
}

export function RowChevron({ active }: { active: boolean }) {
  return (
    <ChevronRight
      className={cn(
        "size-3.5 transition-[color,translate] duration-150 motion-reduce:transition-none",
        active ? "translate-x-0.5 text-foreground/75" : "text-faint",
      )}
    />
  );
}

export function FailureNote({ children }: { children: string }) {
  return (
    <p className="px-2 pt-1 pb-1.5 text-[11px] text-destructive" role="alert">
      {children}
    </p>
  );
}

export type MenuAction = "show" | "fill" | "open" | "upload";

const actionKeys: Record<MenuAction, { key: string; word: MessageKey }> = {
  show: { key: "⌥", word: "extension.menu.hint.show" },
  fill: { key: "↵", word: "extension.menu.hint.fill" },
  open: { key: "↵", word: "extension.menu.hint.open" },
  upload: { key: "↵", word: "extension.menu.hint.upload" },
};

const keyCap =
  "h-[18px] min-w-[18px] rounded-[5px] bg-white/7 px-[5px] text-[10.5px] text-foreground/75 shadow-[inset_0_-1px_0_rgb(0_0_0/0.35),inset_0_0_0_1px_rgb(255_255_255/0.05)]";

export function MenuFooter({
  actions = [],
}: {
  actions?: readonly MenuAction[];
}) {
  const { t } = useTranslator();
  return (
    <footer className="m-[5px_-5px_-5px] flex h-8 items-center rounded-b-row border-t border-white/5 bg-white/2 px-3 text-[11px] text-faint">
      <span className="flex items-center gap-1.5 font-medium text-muted-foreground">
        <KeyGlyph />
        Ravenpass
      </span>
      <span className="ml-auto flex items-center gap-3">
        {actions.map((action) => (
          <span key={action} className="flex items-center gap-1.5">
            <Kbd className={keyCap}>{actionKeys[action].key}</Kbd>
            {t(actionKeys[action].word)}
          </span>
        ))}
        <Kbd className={keyCap}>esc</Kbd>
      </span>
    </footer>
  );
}

export function KeyGlyph() {
  return (
    <span
      aria-hidden="true"
      className="size-3.5 shrink-0 bg-current"
      style={{
        mask: `url("${toolbarIcon("dark")[32]}") center / contain no-repeat`,
      }}
    />
  );
}
