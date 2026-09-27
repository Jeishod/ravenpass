/** GroupBadge names one group an item belongs to. */
export function GroupBadge({ name }: { name: string }) {
  return (
    <span className="shrink-0 rounded-full bg-raised px-2 py-px text-[11px] text-secondary-foreground">
      {name}
    </span>
  );
}
