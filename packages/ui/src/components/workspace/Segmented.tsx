import { cn } from "cn";
import {
  aboveIndicator,
  SelectionGroup,
  SelectionIndicator,
} from "../../motion/SelectionIndicator.tsx";
import { ToggleGroup, ToggleGroupItem } from "../ui/toggle-group.tsx";

/** Segmented is a single choice that always holds a value and scrolls sideways when too narrow. */
export function Segmented<Value extends string>({
  id,
  legend,
  options,
  value,
  disabled = false,
  stretch = false,
  onChange,
}: {
  /** Names the indicator's travel, unique among the segmented controls on screen. */
  id: string;
  legend: string;
  options: readonly { value: Value; label: string }[];
  value: Value;
  disabled?: boolean;
  /** Set when the choices share the whole width of the row. */
  stretch?: boolean;
  onChange: (value: Value) => void;
}) {
  function choose(next: string) {
    const option = options.find((candidate) => candidate.value === next);
    if (option) onChange(option.value);
  }

  return (
    <ToggleGroup
      type="single"
      aria-label={legend}
      value={value}
      onValueChange={choose}
      disabled={disabled}
      rovingFocus={false}
      spacing={0.5}
      className={cn(
        "min-w-0 shrink-0 overflow-x-auto rounded-lg border bg-control p-[3px]",
        stretch && "w-full",
      )}
    >
      <SelectionGroup id={id}>
        {options.map((option) => (
          <ToggleGroupItem
            key={option.value}
            value={option.value}
            className={cn(
              "relative h-[23px] gap-0 px-[11px] text-xs font-normal text-muted-foreground transition-all hover:bg-transparent hover:text-foreground focus:z-auto focus-visible:z-auto motion-safe:active:scale-[0.97] data-[state=on]:bg-transparent data-[state=on]:text-foreground max-sm:h-8 max-sm:px-2",
              stretch && "flex-1",
            )}
          >
            {option.value === value && (
              <SelectionIndicator className="rounded-md bg-secondary" />
            )}
            <span className={aboveIndicator}>{option.label}</span>
          </ToggleGroupItem>
        ))}
      </SelectionGroup>
    </ToggleGroup>
  );
}
