import { cn } from "cn";
import { Fragment } from "react";
import { useCompactLayout } from "../../host/compact.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import {
  aboveIndicator,
  SelectionGroup,
  SelectionIndicator,
} from "../../motion/SelectionIndicator.tsx";
import { Block, BlockIconTile, BlockRowButton } from "../Block.tsx";
import { Button } from "../ui/button.tsx";
import { Separator } from "../ui/separator.tsx";
import {
  type SettingsSection,
  type SettingsSectionEntry,
  settingsParts,
} from "./sections.ts";

/** SettingsNav names the subsections, set apart by part; a compact screen lists them on their own. */
export function SettingsNav({
  sections,
  section,
  onSection,
}: {
  /** The subsections the host offers, in order. */
  sections: readonly SettingsSectionEntry[];
  /** The subsection in view, when one is. */
  section?: SettingsSection;
  onSection: (section: SettingsSection) => void;
}) {
  const { t } = useTranslator();
  const compact = useCompactLayout();
  const parts = settingsParts(sections);

  if (compact) {
    return (
      <nav
        className="flex flex-col px-4 pt-1 pb-6"
        aria-label={t("settings.nav.label")}
      >
        <h1 className="text-[22px] font-medium">{t("settings.nav.heading")}</h1>
        {parts.map(({ part, sections: members }) => (
          <Block key={part} className="mt-3.5">
            {members.map((item) => (
              <BlockRowButton
                key={item.id}
                leading={<BlockIconTile icon={item.icon} />}
                title={t(item.label)}
                onClick={() => onSection(item.id)}
              />
            ))}
          </Block>
        ))}
      </nav>
    );
  }

  return (
    <nav
      className="flex w-[216px] shrink-0 flex-col border-r px-2.5 py-4"
      aria-label={t("settings.nav.label")}
    >
      <span className="mx-2.5 mb-[18px] text-[11px] font-normal text-muted-foreground">
        {t("settings.nav.heading")}
      </span>
      <SelectionGroup id="settings-nav">
        {parts.map(({ part, sections: members }, index) => (
          <Fragment key={part}>
            {index > 0 && (
              <Separator className="mx-[9px] my-4 data-[orientation=horizontal]:w-auto" />
            )}
            {members.map((item) => {
              const Icon = item.icon;
              const active = item.id === section;
              return (
                <Button
                  key={item.id}
                  type="button"
                  variant="ghost"
                  size="sm"
                  aria-current={active ? "page" : undefined}
                  onClick={() => onSection(item.id)}
                  className={cn(
                    "relative my-px h-auto min-h-[34px] justify-start gap-[9px] rounded-[10px] px-[9px] py-[7px] text-left text-[13px] font-normal whitespace-normal has-[>svg]:px-[9px]",
                    !active && "text-secondary-foreground/85",
                  )}
                >
                  {active && (
                    <SelectionIndicator className="rounded-[10px] bg-raised inset-ring inset-ring-white/10" />
                  )}
                  <Icon
                    className={cn(
                      aboveIndicator,
                      "size-[17px] text-muted-foreground",
                    )}
                  />
                  <span className={cn(aboveIndicator, "min-w-0 break-words")}>
                    {t(item.label)}
                  </span>
                </Button>
              );
            })}
          </Fragment>
        ))}
      </SelectionGroup>
    </nav>
  );
}
