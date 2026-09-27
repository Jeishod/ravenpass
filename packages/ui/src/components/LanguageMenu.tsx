import { Languages } from "lucide-react";
import { languageNames } from "../i18n/language.ts";
import { useLanguageChoice, useTranslator } from "../i18n/translator.tsx";
import { Button } from "./ui/button.tsx";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "./ui/dropdown-menu.tsx";

/** LanguageMenu switches the interface language on screens shown before settings are reachable. */
export function LanguageMenu({ compact = false }: { compact?: boolean }) {
  const { t } = useTranslator();
  const { language, options, choose } = useLanguageChoice();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-7 px-2 text-[13px] text-muted-foreground"
          aria-label={t("language.label")}
          title={t("language.label")}
        >
          <Languages data-icon="inline-start" />
          {compact ? language.toUpperCase() : languageNames[language]}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-44">
        <DropdownMenuRadioGroup value={language} onValueChange={choose}>
          {options.map((option) => (
            <DropdownMenuRadioItem key={option.value} value={option.value}>
              {option.label}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
