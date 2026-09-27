/** StepNumber marks a step of a numbered list; the list itself carries the number for assistive technology. */
export function StepNumber({ value }: { value: number }) {
  return (
    <span
      className="flex size-[22px] shrink-0 items-center justify-center rounded-md bg-tile text-[11px] tabular-nums text-muted-foreground"
      aria-hidden="true"
    >
      {value}
    </span>
  );
}
