import { motion } from "motion/react";
import { useCallback, useRef, useState, type WheelEvent } from "react";
import { useTranslator } from "../../i18n/translator.tsx";
import {
  aboveIndicator,
  SelectionGroup,
  SelectionIndicator,
} from "../../motion/SelectionIndicator.tsx";
import type { Group } from "../../vault-api.ts";
import {
  countInGroup,
  everyGroup,
  type ListedItem,
} from "../../workspace/sections.ts";
import { ToggleGroup, ToggleGroupItem } from "../ui/toggle-group.tsx";

// ToggleGroup reports "" for a deselected item, so every group cannot use everyGroup's "" as its value.
const everyGroupValue = "*";

/** fadedEdges masks each edge the strip can still scroll towards. */
function fadedEdges(start: boolean, end: boolean) {
  if (!start && !end) return undefined;
  const opening = start ? "transparent 0, #000 20px" : "#000 0";
  const closing = end
    ? "#000 calc(100% - 20px), transparent 100%"
    : "#000 100%";
  const mask = `linear-gradient(to right, ${opening}, ${closing})`;
  return { maskImage: mask, WebkitMaskImage: mask };
}

/** GroupTabs narrows the list to one group and scrolls sideways within its column. */
export function GroupTabs({
  groups,
  group,
  onGroup,
  items,
}: {
  groups: Group[];
  group: string;
  onGroup: (group: string) => void;
  /** The items the tabs count: those of the open place. */
  items: readonly ListedItem[];
}) {
  const { t } = useTranslator();
  const strip = useRef<HTMLDivElement | null>(null);
  const [edges, setEdges] = useState({ start: false, end: false });

  const measure = useCallback(() => {
    const element = strip.current;
    if (!element) return;
    const room = element.scrollWidth - element.clientWidth;
    setEdges({
      start: element.scrollLeft > 1,
      end: room > 1 && element.scrollLeft < room - 1,
    });
  }, []);

  const observe = useCallback(
    (element: HTMLDivElement | null) => {
      strip.current = element;
      if (!element) return;
      const observer = new ResizeObserver(measure);
      observer.observe(element);
      if (element.firstElementChild)
        observer.observe(element.firstElementChild);
      return () => observer.disconnect();
    },
    [measure],
  );

  if (!groups.length) return null;

  // Most mouse wheels have no horizontal axis.
  function scrollSideways(event: WheelEvent<HTMLDivElement>) {
    const element = strip.current;
    if (!element || Math.abs(event.deltaY) <= Math.abs(event.deltaX)) return;
    element.scrollLeft += event.deltaY;
  }

  function choose(next: string) {
    if (next) onGroup(next === everyGroupValue ? everyGroup : next);
  }

  return (
    // layoutScroll: Motion measures the indicator against this element's scroll offset.
    <motion.div
      ref={observe}
      layoutScroll
      onWheel={scrollSideways}
      onScroll={measure}
      style={fadedEdges(edges.start, edges.end)}
      className="w-full min-w-0 shrink-0 overflow-x-auto pb-0.5 [-ms-overflow-style:none] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
    >
      <ToggleGroup
        type="single"
        aria-label={t("workspace.groups.label")}
        value={group === everyGroup ? everyGroupValue : group}
        onValueChange={choose}
        rovingFocus={false}
        spacing={0.75}
        className="w-max"
      >
        <SelectionGroup id="group-tabs">
          <GroupTab
            value={everyGroupValue}
            label={t("workspace.groups.all")}
            active={group === everyGroup}
          />
          {groups.map((item) => (
            <GroupTab
              key={item.id}
              value={item.id}
              label={item.name}
              count={countInGroup(items, item.id)}
              active={group === item.id}
            />
          ))}
        </SelectionGroup>
      </ToggleGroup>
    </motion.div>
  );
}

function GroupTab({
  value,
  label,
  count,
  active,
}: {
  value: string;
  label: string;
  count?: number;
  active: boolean;
}) {
  return (
    <ToggleGroupItem
      value={value}
      className="relative h-[26px] gap-1.5 rounded-full px-3 text-xs font-normal text-muted-foreground transition-all hover:bg-accent hover:text-foreground focus:z-auto focus-visible:z-auto motion-safe:active:scale-[0.97] data-[state=on]:bg-transparent data-[state=on]:text-foreground dark:hover:bg-accent/50 max-sm:h-8"
    >
      {active && (
        <SelectionIndicator className="rounded-full bg-raised inset-ring inset-ring-white/10" />
      )}
      <span className={`${aboveIndicator} max-w-[120px] truncate`}>
        {label}
      </span>
      {count !== undefined && (
        <span className={`${aboveIndicator} text-faint`}>{count}</span>
      )}
    </ToggleGroupItem>
  );
}
