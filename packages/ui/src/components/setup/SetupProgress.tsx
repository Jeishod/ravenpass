import { LoaderCircle } from "lucide-react";
import { useTranslator } from "../../i18n/translator.tsx";

/** SetupProgress stands in for a step while what it submitted is saved; it offers no action, so nothing is sent twice. */
export function SetupProgress({
  title,
  biometry,
}: {
  title: string;
  /** Whether a device authentication key is being created, which the device can take long over. */
  biometry: boolean;
}) {
  const { t } = useTranslator();
  return (
    <div
      role="status"
      className="flex flex-col items-center gap-2 py-4 text-center"
    >
      <LoaderCircle
        className="mb-1 size-6 animate-spin text-muted-foreground motion-reduce:animate-none"
        aria-hidden="true"
      />
      <p className="text-[15px] font-medium">{title}</p>
      {biometry && (
        <p className="text-[13px] leading-[1.5] text-muted-foreground">
          {t("unlock-methods.biometry.creating")}
        </p>
      )}
    </div>
  );
}
