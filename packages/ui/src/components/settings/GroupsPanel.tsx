import { cn } from "cn";
import { useState } from "react";
import { useTranslator } from "../../i18n/translator.tsx";
import type { Group } from "../../vault-api.ts";
import { countInGroup, type ListedItem } from "../../workspace/sections.ts";
import { Block, BlockRow, blockControl } from "../Block.tsx";
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
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../ui/select.tsx";
import { NameDialog } from "./NameDialog.tsx";

/** A menu item cannot hold an empty value. */
const noGroup = "none";

export function GroupsPanel({
  groups,
  items,
  defaultGroup,
  busy,
  onCreate,
  onRename,
  onDelete,
  onDefaultGroup,
}: {
  groups: Group[];
  /** Every item of the open vault, of every kind. */
  items: readonly ListedItem[];
  defaultGroup: string;
  busy: boolean;
  onCreate: (name: string) => Promise<boolean>;
  onRename: (id: string, name: string) => Promise<boolean>;
  onDelete: (id: string) => void;
  onDefaultGroup: (id: string) => void;
}) {
  const { t } = useTranslator();
  const [editing, setEditing] = useState<Group | null>(null);
  const [creating, setCreating] = useState(false);
  const [deleting, setDeleting] = useState<Group | null>(null);
  const groupName = {
    label: t("settings.groups.name"),
    placeholder: t("settings.groups.name.placeholder"),
    maxLength: 64,
    save: t("settings.groups.save"),
    cancel: t("settings.groups.cancel"),
  };

  return (
    <>
      <Block>
        {groups.map((group) => (
          <BlockRow
            key={group.id}
            title={group.name}
            detail={t("settings.groups.count", {
              count: countInGroup(items, group.id),
            })}
          >
            <Button
              type="button"
              variant="quiet"
              size="pill-sm"
              className="bg-tile font-normal hover:bg-field-hover"
              disabled={busy}
              onClick={() => setEditing(group)}
            >
              {t("settings.groups.rename")}
            </Button>
          </BlockRow>
        ))}
        {!groups.length && (
          <p className="px-[13px] py-3 text-[13px] text-muted-foreground">
            {t("settings.groups.empty")}
          </p>
        )}
      </Block>
      <div className="mt-3 mb-5">
        <Button
          type="button"
          variant="quiet"
          size="pill-sm"
          className="bg-tile font-normal hover:bg-field-hover"
          onClick={() => setCreating(true)}
          disabled={busy}
        >
          {t("settings.groups.new")}
        </Button>
      </div>

      {groups.length > 0 && (
        <Block>
          <BlockRow
            title={t("settings.groups.default.label")}
            htmlFor="default-group"
          >
            <Select
              value={defaultGroup || noGroup}
              onValueChange={(next) =>
                onDefaultGroup(next === noGroup ? "" : next)
              }
              disabled={busy}
            >
              <SelectTrigger
                id="default-group"
                size="sm"
                className={cn(blockControl, "max-w-[170px]")}
                aria-label={t("settings.groups.default.label")}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectItem value={noGroup}>
                    {t("settings.groups.default.none")}
                  </SelectItem>
                  {groups.map((group) => (
                    <SelectItem key={group.id} value={group.id}>
                      {group.name}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </BlockRow>
        </Block>
      )}

      <NameDialog
        open={creating}
        title={t("settings.groups.create.title")}
        {...groupName}
        busy={busy}
        onSave={onCreate}
        onClose={() => setCreating(false)}
      />
      <NameDialog
        key={editing?.id ?? "rename"}
        open={editing !== null}
        title={t("settings.groups.rename.title")}
        {...groupName}
        initial={editing?.name ?? ""}
        busy={busy}
        onSave={(name) =>
          editing ? onRename(editing.id, name) : Promise.resolve(false)
        }
        onClose={() => setEditing(null)}
        onDelete={{
          label: t("settings.groups.delete"),
          run: () => {
            const group = editing;
            setEditing(null);
            if (group) setDeleting(group);
          },
        }}
      />

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
              {t("settings.groups.delete.title", {
                name: deleting?.name ?? "",
              })}
            </ResponsiveDialogTitle>
            <ResponsiveDialogDescription>
              {t("settings.groups.delete.detail")}
            </ResponsiveDialogDescription>
          </ResponsiveDialogHeader>
          <ResponsiveDialogFooter>
            <ResponsiveDialogCancel>
              {t("settings.groups.cancel")}
            </ResponsiveDialogCancel>
            <ResponsiveDialogAction
              variant="destructive"
              onClick={() => {
                if (deleting) onDelete(deleting.id);
                setDeleting(null);
              }}
            >
              {t("settings.groups.delete.confirm")}
            </ResponsiveDialogAction>
          </ResponsiveDialogFooter>
        </ResponsiveDialogContent>
      </ResponsiveDialog>
    </>
  );
}
