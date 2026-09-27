import { MoreHorizontal } from "lucide-react";
import { useState } from "react";
import { useTranslator } from "../../i18n/translator.tsx";
import { vaultLocation, vaultName } from "../../storage/location.ts";
import type { StorageKind, StorageStatus, Vault } from "../../vault-api.ts";
import { Block, BlockHeading, BlockNote, BlockRow } from "../Block.tsx";
import {
  ResponsiveDialog,
  ResponsiveDialogAction,
  ResponsiveDialogCancel,
  ResponsiveDialogContent,
  ResponsiveDialogDescription,
  ResponsiveDialogFooter,
  ResponsiveDialogHeader,
  ResponsiveDialogTitle,
} from "../ResponsiveDialog.tsx";
import { Button } from "../ui/button.tsx";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "../ui/dropdown-menu.tsx";
import { VaultStorage } from "./VaultStorage.tsx";

export function VaultsPanel({
  storage,
  busy,
  moving,
  onMove,
  onForget,
  onDelete,
}: {
  storage: StorageStatus | null;
  busy: boolean;
  moving: boolean;
  onMove: (kind: StorageKind) => void;
  onForget: (vault: Vault) => void;
  onDelete: (vault: Vault) => void;
}) {
  const { t } = useTranslator();
  const [deleting, setDeleting] = useState<Vault | null>(null);

  return (
    <>
      <VaultStorage
        storage={storage}
        busy={busy}
        moving={moving}
        onMove={onMove}
      />

      <BlockHeading>{t("settings.vaults.list")}</BlockHeading>
      <Block>
        {(storage?.vaults ?? []).map((vault) => (
          <BlockRow
            key={vault.path}
            title={vaultName(vault.name)}
            detail={<span className="select-text">{vault.place}</span>}
          >
            {vault.current && (
              <span className="rounded-full bg-tile px-2 py-px text-[11px] text-secondary-foreground">
                {t("settings.vaults.current")}
              </span>
            )}
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  className="size-7 rounded-md text-muted-foreground hover:text-foreground"
                  aria-label={t("settings.vaults.actions", {
                    name: vaultName(vault.name),
                  })}
                  disabled={busy}
                >
                  <MoreHorizontal />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuGroup>
                  <DropdownMenuItem onSelect={() => onForget(vault)}>
                    {t("settings.vaults.forget")}
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    variant="destructive"
                    onSelect={() => setDeleting(vault)}
                  >
                    {t("settings.vaults.delete")}
                  </DropdownMenuItem>
                </DropdownMenuGroup>
              </DropdownMenuContent>
            </DropdownMenu>
          </BlockRow>
        ))}
      </Block>
      <BlockNote>{t("settings.vaults.note")}</BlockNote>

      <ResponsiveDialog
        role="alertdialog"
        open={deleting !== null}
        onOpenChange={(open) => {
          if (!open) setDeleting(null);
        }}
      >
        <ResponsiveDialogContent>
          <ResponsiveDialogHeader>
            <ResponsiveDialogTitle>
              {t("settings.delete.title", {
                name: deleting ? vaultName(deleting.name) : "",
              })}
            </ResponsiveDialogTitle>
            <ResponsiveDialogDescription>
              {t("settings.delete.description", {
                location: deleting ? vaultLocation(deleting) : "",
              })}
            </ResponsiveDialogDescription>
          </ResponsiveDialogHeader>
          <ResponsiveDialogFooter>
            <ResponsiveDialogCancel>
              {t("settings.delete.cancel")}
            </ResponsiveDialogCancel>
            <ResponsiveDialogAction
              variant="destructive"
              onClick={() => {
                if (deleting) onDelete(deleting);
                setDeleting(null);
              }}
            >
              {t("settings.delete.confirm")}
            </ResponsiveDialogAction>
          </ResponsiveDialogFooter>
        </ResponsiveDialogContent>
      </ResponsiveDialog>
    </>
  );
}
