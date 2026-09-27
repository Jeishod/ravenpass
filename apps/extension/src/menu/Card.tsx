import { Button } from "@ravenpass/ui/components/ui/button.tsx";
import { useTranslator } from "@ravenpass/ui/i18n/translator.tsx";
import { X } from "lucide-react";
import { KeyGlyph } from "./Rows.tsx";

export function CardHeader({
  title,
  onClose,
}: {
  title: string;
  onClose: () => void;
}) {
  const { t } = useTranslator();
  return (
    <header className="flex h-8 items-center gap-2 pr-0.5 pl-2.5 text-muted-foreground">
      <KeyGlyph />
      <h1 className="min-w-0 flex-1 truncate font-medium text-[12px] text-foreground/85">
        {title}
      </h1>
      <Button
        type="button"
        variant="ghost"
        size="icon-xs"
        className="text-faint hover:text-foreground"
        aria-label={t("extension.card.close")}
        onClick={onClose}
      >
        <X />
      </Button>
    </header>
  );
}
