import { KeyRound, ListOrdered, Sprout } from "lucide-react";
import type { ComponentType } from "react";
import { useTranslator } from "../../i18n/translator.tsx";
import { summaryLabel } from "../../seeds/phrase.ts";
import type { SeedFormat, SeedSummary } from "../../vault-api.ts";
import { Avatar } from "./Avatar.tsx";
import { ItemList, type ListProps } from "./ItemList.tsx";

const formatIcons: Record<SeedFormat, ComponentType<{ className?: string }>> = {
  phrase: Sprout,
  key: KeyRound,
  codes: ListOrdered,
};

/** seedFace is what a seed's avatar shows: a phrase's word count, or the icon of its format. */
export function seedFace(format: SeedFormat, total: number) {
  return {
    mark: format === "phrase" ? String(total) : "",
    icon: formatIcons[format],
  };
}

/** useSeedLine is the line under a seed's name: what it holds, then its wallet. */
export function useSeedLine() {
  const { t } = useTranslator();
  return (seed: Pick<SeedSummary, "format" | "total" | "used" | "wallet">) => {
    const label = summaryLabel(seed);
    return {
      text: [t(label.key, label.values), seed.wallet]
        .filter(Boolean)
        .join(" · "),
      warning: label.warning,
    };
  };
}

export function SeedList(props: ListProps<SeedSummary>) {
  const { t } = useTranslator();
  const line = useSeedLine();

  return (
    <ItemList
      {...props}
      label={t("seed.list.label")}
      untitled={t("seed.untitled")}
      shape="tile"
      icon={Sprout}
      detail={(entry) => {
        const { text, warning } = line(entry);
        return { text, tone: warning ? "warning" : "default" };
      }}
      avatar={(entry, { selected }) => (
        <Avatar
          label={entry.label}
          {...seedFace(entry.format, entry.total)}
          size="row"
          shape="tile"
          emphasis={selected ? "selected" : "none"}
        />
      )}
    />
  );
}
