import { cn } from "cn";
import { useTranslator } from "../i18n/translator.tsx";
import { vaultLocation } from "../storage/location.ts";
import type { StorageKind, StorageStatus } from "../vault-api.ts";
import { BlockRow, blockControl } from "./Block.tsx";
import { Button } from "./ui/button.tsx";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "./ui/select.tsx";

/** StorageTypeRow picks how Ravenpass keeps a vault current. */
export function StorageTypeRow({
  id,
  label,
  status,
  disabled = false,
  onChange,
}: {
  id: string;
  label: string;
  status: StorageStatus | null;
  disabled?: boolean;
  /** Called with the type chosen when it is not the current one. */
  onChange: (kind: StorageKind) => void;
}) {
  const { t } = useTranslator();
  const kinds = status?.kinds ?? [];
  return (
    <BlockRow title={label} htmlFor={id}>
      <Select
        value={status?.kind ?? ""}
        disabled={disabled || kinds.length < 2}
        onValueChange={(value) => {
          const kind = kinds.find((each) => each === value);
          if (kind && kind !== status?.kind) onChange(kind);
        }}
      >
        <SelectTrigger
          id={id}
          size="sm"
          className={cn(blockControl, "w-[180px]")}
          aria-label={label}
        >
          <SelectValue placeholder="…" />
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            {kinds.map((kind) => (
              <SelectItem key={kind} value={kind}>
                {t(`storage.type.${kind}`)}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    </BlockRow>
  );
}

/** StorageFileRow shows where the vault file is, and changes it where the owner picks the file. */
export function StorageFileRow({
  label,
  status,
  actionLabel,
  busyLabel,
  emptyLabel,
  onAction,
  disabled = false,
  busy = false,
}: {
  label: string;
  status: StorageStatus | null;
  actionLabel: string;
  busyLabel: string;
  emptyLabel: string;
  onAction?: () => void;
  disabled?: boolean;
  /** Set while the action itself runs. */
  busy?: boolean;
}) {
  return (
    <BlockRow
      title={label}
      detail={
        <span
          className={cn(
            "select-text",
            status?.kind === "local-file" && "font-mono",
          )}
        >
          {status?.path ? vaultLocation(status) : emptyLabel}
        </span>
      }
    >
      {onAction && (
        <Button
          type="button"
          size="pill-sm"
          variant="quiet"
          onClick={onAction}
          disabled={disabled || busy}
        >
          {busy ? busyLabel : actionLabel}
        </Button>
      )}
    </BlockRow>
  );
}
