import { cn } from "cn";
import { Check, ChevronsUpDown, X } from "lucide-react";
import { type ReactNode, useRef, useState } from "react";
import { useTranslator } from "../../i18n/translator.tsx";
import type { Group } from "../../vault-api.ts";
import { Button } from "../ui/button.tsx";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "../ui/command.tsx";
import { Popover, PopoverContent, PopoverTrigger } from "../ui/popover.tsx";
import { Textarea } from "../ui/textarea.tsx";

/** A field without a border; its row shows the focus. */
export const bareField =
  "h-auto border-0 bg-transparent px-0 py-0 text-[13px] shadow-none focus-visible:border-0 focus-visible:ring-0";

/** The first and last rows round their own corners, or the block's clip cuts the focus ring. */
export const editorRow =
  "flex min-h-[41px] items-center gap-2.5 border-b px-[13px] py-1.5 first:rounded-t-row last:rounded-b-row last:border-b-0 focus-within:bg-field-hover focus-within:inset-ring focus-within:inset-ring-foreground/14";

export const labelColumn =
  "w-[88px] shrink-0 text-[11px] text-muted-foreground";

/** A block holding one long field under its label, such as notes. */
export const fieldSection =
  "shrink-0 rounded-row bg-field px-[13px] py-[11px] focus-within:bg-field-hover focus-within:inset-ring focus-within:inset-ring-foreground/14";

/** useRemaining returns the characters left once a field nears its limit, and nothing before. */
export function useRemaining() {
  const { t } = useTranslator();
  return (value: string, limit: number | undefined, margin: number) => {
    if (!limit || value.length <= limit - margin) return undefined;
    return t("workspace.editor.remaining", { used: value.length, limit });
  };
}

export function EditorRow({
  label,
  htmlFor,
  counter,
  children,
}: {
  label: string;
  htmlFor: string;
  counter?: string;
  children: ReactNode;
}) {
  return (
    <div className={editorRow}>
      <label className={labelColumn} htmlFor={htmlFor}>
        {label}
      </label>
      <div className="flex min-w-0 flex-1 items-center gap-1">{children}</div>
      {counter && (
        <span className="shrink-0 text-[11px] text-faint tabular-nums">
          {counter}
        </span>
      )}
    </div>
  );
}

/** GroupField picks groups from a searchable list and shows the chosen ones in its row. */
export function GroupField({
  groups,
  membership,
  busy,
  onToggle,
}: {
  groups: Group[];
  membership: string[];
  busy: boolean;
  onToggle: (id: string) => void;
}) {
  const { t } = useTranslator();
  const [open, setOpen] = useState(false);
  const chosen = groups.filter((group) => membership.includes(group.id));

  return (
    <div className={editorRow}>
      <span className={labelColumn}>{t("workspace.editor.groups")}</span>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <button
            type="button"
            disabled={busy}
            aria-expanded={open}
            className="flex min-w-0 flex-1 items-center gap-1.5 rounded-md py-1 text-left outline-none disabled:opacity-50"
          >
            <span className="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
              {chosen.map((group) => (
                <span
                  key={group.id}
                  className="max-w-[140px] truncate rounded-full bg-tile px-2 py-px text-[11px]"
                >
                  {group.name}
                </span>
              ))}
              {!chosen.length && (
                <span className="text-[13px] text-muted-foreground">
                  {t("workspace.editor.groups.none")}
                </span>
              )}
            </span>
            <ChevronsUpDown className="size-3.5 shrink-0 text-muted-foreground" />
          </button>
        </PopoverTrigger>
        <PopoverContent align="start" className="w-[240px]">
          <Command>
            <CommandInput placeholder={t("workspace.editor.groups.search")} />
            <CommandList>
              <CommandEmpty>{t("workspace.editor.groups.empty")}</CommandEmpty>
              <CommandGroup>
                {groups.map((group) => (
                  <CommandItem
                    key={group.id}
                    value={group.name}
                    onSelect={() => onToggle(group.id)}
                  >
                    <span className="min-w-0 flex-1 truncate">
                      {group.name}
                    </span>
                    {membership.includes(group.id) && (
                      <Check className="size-4 text-foreground" />
                    )}
                  </CommandItem>
                ))}
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>
    </div>
  );
}

interface Row<Value> {
  key: number;
  value: Value;
}

/** useRows keys each row itself, or a removal would hand one row's field to another. */
export function useRows<Value>(initial: readonly Value[]) {
  const next = useRef(initial.length);
  const [rows, setRows] = useState<Row<Value>[]>(() =>
    initial.map((value, key) => ({ key, value })),
  );

  return {
    rows,
    values: rows.map((row) => row.value),
    add(value: Value) {
      const key = next.current++;
      setRows((current) => [...current, { key, value }]);
    },
    remove(key: number) {
      setRows((current) => current.filter((row) => row.key !== key));
    },
    change(key: number, value: Value) {
      setRows((current) =>
        current.map((row) => (row.key === key ? { key, value } : row)),
      );
    },
    /** update reads the row's current value, for a change that lands after an await. */
    update(key: number, next: (value: Value) => Value) {
      setRows((current) =>
        current.map((row) =>
          row.key === key ? { key, value: next(row.value) } : row,
        ),
      );
    },
  };
}

/** Whether another row fits under a count limit. Unknown limits leave the choice to the vault. */
export function roomFor(count: number, limit: number | undefined): boolean {
  return limit === undefined || count < limit;
}

export function RemoveButton({
  label,
  busy,
  onRemove,
}: {
  label: string;
  busy: boolean;
  onRemove: () => void;
}) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon-sm"
      className="size-7 shrink-0 rounded-md text-muted-foreground hover:text-foreground"
      onClick={onRemove}
      aria-label={label}
      title={label}
      disabled={busy}
    >
      <X className="size-4" />
    </Button>
  );
}

/** toggled adds a group to a membership or takes it out. */
export function toggled(membership: string[], id: string): string[] {
  return membership.includes(id)
    ? membership.filter((member) => member !== id)
    : [...membership, id];
}

export function NotesField({
  id,
  value,
  limit,
  busy,
  onChange,
}: {
  id: string;
  value: string;
  limit: number | undefined;
  busy: boolean;
  onChange: (value: string) => void;
}) {
  const { t } = useTranslator();
  const remaining = useRemaining()(value, limit, 500);

  return (
    <section className={fieldSection}>
      <div className="mb-1 flex items-baseline gap-2">
        <label className="text-[11px] text-muted-foreground" htmlFor={id}>
          {t("workspace.field.notes")}
        </label>
        {remaining && (
          <span className="ml-auto text-[11px] text-faint tabular-nums">
            {remaining}
          </span>
        )}
      </div>
      <Textarea
        id={id}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        maxLength={limit}
        rows={3}
        className={cn(bareField, "min-h-20 resize-none text-xs leading-[1.5]")}
        disabled={busy}
      />
    </section>
  );
}

/** The header every editor carries: what is being edited, and the two ways out of it. */
export function EditorHeader({
  title,
  name,
  form,
  submit,
  canSubmit,
  busy,
  onCancel,
}: {
  title: string;
  /** The name typed so far, shown under the title once there is one. */
  name: string;
  form: string;
  submit: string;
  canSubmit: boolean;
  busy: boolean;
  onCancel: () => void;
}) {
  const { t } = useTranslator();

  return (
    <header className="flex shrink-0 items-center gap-3">
      <div className="min-w-0 flex-1">
        <h2 className="truncate text-[17px]">{title}</h2>
        {name && (
          <p className="truncate text-[11px] text-muted-foreground">{name}</p>
        )}
      </div>
      <div className="flex shrink-0 items-center gap-1.5">
        <Button
          type="button"
          variant="quiet"
          size="pill-sm"
          className="h-[29px] px-3.5"
          onClick={onCancel}
          disabled={busy}
        >
          {t("workspace.editor.cancel")}
        </Button>
        <Button
          type="submit"
          form={form}
          variant="raised"
          size="pill-sm"
          className="h-[29px] px-3.5"
          disabled={busy || !canSubmit}
        >
          {busy ? t("workspace.editor.busy") : submit}
        </Button>
      </div>
    </header>
  );
}
