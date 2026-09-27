import { ChevronLeft } from "lucide-react";
import { useSystemBack } from "../../host/back.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import { Button } from "../ui/button.tsx";

/** BackRow heads a compact sub-screen; the system back gesture also triggers `onBack`. */
export function BackRow({
  label,
  onBack,
}: {
  /** The screen the row returns to. */
  label: string;
  onBack: () => void;
}) {
  const { t } = useTranslator();
  useSystemBack(onBack);

  return (
    <div className="flex shrink-0 px-2 pt-[env(safe-area-inset-top)]">
      <Button
        type="button"
        variant="ghost"
        aria-label={t("workspace.back", { place: label })}
        onClick={onBack}
        className="h-11 min-w-0 gap-1 px-2 text-[13px] font-normal text-muted-foreground hover:text-foreground"
      >
        <ChevronLeft className="size-5" />
        <span className="truncate">{label}</span>
      </Button>
    </div>
  );
}
