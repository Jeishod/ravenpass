import { cn } from "cn";
import {
  Check,
  ChevronsUpDown,
  FolderOpen,
  HardDrive,
  type LucideIcon,
  Plus,
} from "lucide-react";
import { type ComponentProps, type ReactNode, useId, useState } from "react";
import { useCapabilities } from "../host/capabilities.tsx";
import { useCompactLayout } from "../host/compact.ts";
import { useTranslator } from "../i18n/translator.tsx";
import { SelectionGroup } from "../motion/SelectionIndicator.tsx";
import { vaultName } from "../storage/location.ts";
import type { Vault } from "../vault-api.ts";
import {
  ResponsiveDialog,
  ResponsiveDialogContent,
  ResponsiveDialogHeader,
  ResponsiveDialogTitle,
} from "./ResponsiveDialog.tsx";
import { Button } from "./ui/button.tsx";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "./ui/dropdown-menu.tsx";
import { Separator } from "./ui/separator.tsx";
import { Avatar } from "./workspace/Avatar.tsx";
import { ItemRowContent, itemRowClass } from "./workspace/ItemRowContent.tsx";

/** A vault the menu lists. */
interface VaultChoice {
  path: string;
  name: string;
  /** Where the vault is kept. */
  place: string;
  current: boolean;
  act: () => void;
}

/** A way to add a vault. */
interface VaultAction {
  id: "new" | "open";
  icon: LucideIcon;
  title: string;
  act: () => void;
}

/** What each form of the menu shows and does. */
interface VaultMenuView {
  /** The current vault's name, on the button. */
  label: string;
  heading: string;
  choices: VaultChoice[];
  actions: VaultAction[];
  busy: boolean;
}

/** VaultMenu switches between the vaults the device knows and starts new ones: a dropdown when wide, a drawer when compact. */
export function VaultMenu({
  vaults,
  currentPath,
  busy,
  onSwitch,
  onCreate,
  onOpen,
}: {
  vaults: Vault[];
  currentPath: string;
  busy: boolean;
  onSwitch: (path: string) => void;
  onCreate: () => void;
  onOpen: () => void;
}) {
  const { t } = useTranslator();
  const compact = useCompactLayout();
  // Where the host keeps vaults in a place of its own, a file chosen elsewhere would be a copy.
  const { storageLocations } = useCapabilities();
  const current = vaults.find((vault) => vault.path === currentPath);

  const choices = vaults.map((vault): VaultChoice => {
    const chosen = vault.path === currentPath;
    return {
      path: vault.path,
      name: vaultName(vault.name),
      place: vault.place,
      current: chosen,
      act: () => {
        if (!chosen) onSwitch(vault.path);
      },
    };
  });
  const actions: VaultAction[] = [
    { id: "new", icon: Plus, title: t("vault-menu.new"), act: onCreate },
  ];
  if (storageLocations) {
    actions.push({
      id: "open",
      icon: FolderOpen,
      title: t("vault-menu.open"),
      act: onOpen,
    });
  }

  const View = compact ? VaultDrawer : VaultDropdown;
  return (
    <View
      label={vaultName(current?.name ?? "") || t("vault-menu.current")}
      heading={t("vault-menu.heading")}
      choices={choices}
      actions={actions}
      busy={busy}
    />
  );
}

function VaultDropdown({
  label,
  heading,
  choices,
  actions,
  busy,
}: VaultMenuView) {
  const headingId = useId();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <VaultButton label={label} disabled={busy} />
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="start"
        collisionPadding={8}
        aria-labelledby={headingId}
        className="w-[260px] max-w-(--radix-dropdown-menu-content-available-width)"
      >
        <DropdownMenuLabel
          id={headingId}
          className="text-xs font-normal text-muted-foreground"
        >
          {heading}
        </DropdownMenuLabel>
        {choices.map((choice) => (
          <DropdownMenuItem
            key={choice.path}
            textValue={choice.name}
            aria-current={choice.current ? "true" : undefined}
            disabled={busy}
            onSelect={choice.act}
          >
            <span className="min-w-0 flex-1">
              <span className="block truncate">{choice.name}</span>
              {choice.place && (
                <span className="block truncate text-xs text-muted-foreground">
                  {choice.place}
                </span>
              )}
            </span>
            {choice.current && <Check className="text-current" />}
          </DropdownMenuItem>
        ))}
        <DropdownMenuSeparator />
        {actions.map((action) => (
          <DropdownMenuItem
            key={action.id}
            disabled={busy}
            onSelect={action.act}
          >
            <action.icon />
            {action.title}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function VaultDrawer({
  label,
  heading,
  choices,
  actions,
  busy,
}: VaultMenuView) {
  const [open, setOpen] = useState(false);

  function choose(act: () => void) {
    setOpen(false);
    act();
  }

  return (
    <>
      <VaultButton
        label={label}
        disabled={busy}
        aria-haspopup="dialog"
        aria-expanded={open}
        onClick={() => setOpen(true)}
      />
      <ResponsiveDialog open={open} onOpenChange={setOpen}>
        <ResponsiveDialogContent className="bg-background">
          <ResponsiveDialogHeader>
            <ResponsiveDialogTitle>{heading}</ResponsiveDialogTitle>
          </ResponsiveDialogHeader>
          {/* Without min-w-0 the drawer's grid column widens to the longest line and scrolls sideways. */}
          <div className="flex min-w-0 flex-col gap-3">
            <div className="flex flex-col gap-1">
              <SelectionGroup id="vault-menu">
                {choices.map((choice) => (
                  <DrawerRow
                    key={choice.path}
                    current={choice.current}
                    avatar={
                      <Avatar
                        label={choice.name}
                        icon={HardDrive}
                        size="row"
                        shape="tile"
                        emphasis={choice.current ? "selected" : "none"}
                      />
                    }
                    title={choice.name}
                    detail={choice.place}
                    disabled={busy}
                    onChoose={() => choose(choice.act)}
                  />
                ))}
              </SelectionGroup>
            </div>
            <Separator />
            <div className="flex flex-col gap-1">
              {actions.map((action) => (
                <DrawerRow
                  key={action.id}
                  avatar={<ActionTile icon={action.icon} />}
                  title={action.title}
                  disabled={busy}
                  onChoose={() => choose(action.act)}
                />
              ))}
            </div>
          </div>
        </ResponsiveDialogContent>
      </ResponsiveDialog>
    </>
  );
}

function VaultButton({
  label,
  ...props
}: { label: string } & Omit<ComponentProps<"button">, "children">) {
  return (
    <Button
      type="button"
      variant="quiet"
      size="pill-sm"
      className="max-w-[220px] max-sm:h-10 max-sm:px-3.5 max-sm:text-[13px]"
      {...props}
    >
      <HardDrive data-icon="inline-start" />
      <span className="truncate">{label}</span>
      <ChevronsUpDown data-icon="inline-end" />
    </Button>
  );
}

function DrawerRow({
  avatar,
  title,
  detail,
  current = false,
  disabled,
  onChoose,
}: {
  avatar: ReactNode;
  title: string;
  /** Where a vault is kept; unset for a way to add one. */
  detail?: string;
  current?: boolean;
  disabled: boolean;
  onChoose: () => void;
}) {
  return (
    <button
      type="button"
      className={cn(
        itemRowClass(!current),
        "relative w-full disabled:opacity-50",
      )}
      aria-current={current ? "true" : undefined}
      disabled={disabled}
      onClick={onChoose}
    >
      <ItemRowContent
        active={current}
        avatar={avatar}
        title={title}
        detail={detail ? { text: detail } : undefined}
      />
    </button>
  );
}

function ActionTile({ icon }: { icon: LucideIcon }) {
  return (
    <Avatar label="" icon={icon} size="row" shape="tile" emphasis="none" />
  );
}
