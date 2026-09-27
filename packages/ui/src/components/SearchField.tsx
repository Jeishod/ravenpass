import { cn } from "cn";
import { Search } from "lucide-react";
import type { RefObject } from "react";
import { useCompactLayout } from "../host/compact.ts";
import { Input } from "./ui/input.tsx";

/** SearchField is a search box whose label only assistive technology reads. */
export function SearchField({
  id,
  inputRef,
  value,
  onChange,
  label,
  placeholder,
  shortcut,
  className,
}: {
  id: string;
  inputRef?: RefObject<HTMLInputElement | null>;
  value: string;
  onChange: (value: string) => void;
  label: string;
  placeholder: string;
  /** The keys that focus the field, as the platform writes them, on a host that takes shortcuts. */
  shortcut?: string;
  className?: string;
}) {
  const compact = useCompactLayout();
  return (
    <div className={cn("relative", className)}>
      <Search
        className={cn(
          "pointer-events-none absolute top-1/2 -translate-y-1/2 text-muted-foreground",
          compact ? "left-3 size-4" : "left-2.5 size-[15px]",
        )}
        aria-hidden="true"
      />
      <label className="sr-only" htmlFor={id}>
        {label}
      </label>
      <Input
        id={id}
        ref={inputRef}
        type="search"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className={
          compact
            ? "h-10 rounded-xl border bg-control pl-9"
            : cn(
                "h-[29px] rounded-lg border bg-control pl-8 text-xs",
                shortcut && "pr-11",
              )
        }
      />
      {shortcut && !compact && (
        <kbd className="pointer-events-none absolute top-1/2 right-2.5 -translate-y-1/2 font-mono text-[11px] text-faint">
          {shortcut}
        </kbd>
      )}
    </div>
  );
}
