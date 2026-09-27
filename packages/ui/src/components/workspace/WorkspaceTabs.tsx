import { cn } from "cn";
import { useTranslator } from "../../i18n/translator.tsx";
import {
  aboveIndicator,
  SelectionGroup,
  SelectionIndicator,
} from "../../motion/SelectionIndicator.tsx";
import { type ItemPlaceName, itemPlaces } from "./places.ts";

/** WorkspaceTabs is the compact screen's rail; settings open from the toolbar. */
export function WorkspaceTabs({
  place,
  onPlace,
}: {
  place: ItemPlaceName;
  onPlace: (place: ItemPlaceName) => void;
}) {
  const { t } = useTranslator();

  return (
    <nav
      className="flex shrink-0 border-t bg-background px-1 pt-1.5 pb-[max(env(safe-area-inset-bottom),6px)]"
      aria-label={t("workspace.rail.label")}
    >
      <SelectionGroup id="workspace-tabs">
        {itemPlaces.map((item) => {
          const Icon = item.icon;
          const active = item.id === place;
          return (
            <button
              key={item.id}
              type="button"
              aria-current={active ? "page" : undefined}
              onClick={() => onPlace(item.id)}
              className={cn(
                "flex min-h-11 min-w-0 flex-1 flex-col items-center gap-1 rounded-lg text-[11px] outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50",
                active ? "text-foreground" : "text-muted-foreground",
              )}
            >
              <span
                className={cn(
                  "relative flex h-7 w-11 items-center justify-center rounded-full",
                  active && "text-(--action-foreground)",
                )}
              >
                {active && (
                  <SelectionIndicator className="action-fill rounded-full" />
                )}
                <Icon className={`${aboveIndicator} size-[19px]`} />
              </span>
              <span className="max-w-full truncate">{t(item.label)}</span>
            </button>
          );
        })}
      </SelectionGroup>
    </nav>
  );
}
