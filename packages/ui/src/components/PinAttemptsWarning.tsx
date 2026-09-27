import { useTranslator } from "../i18n/translator.tsx";

// Wrong attempts are only worth counting out loud once the PIN is close to being removed.
const warnFromAttemptsLeft = 3;

/** PinAttemptsWarning counts the wrong PIN attempts left before the PIN is removed, once few are. */
export function PinAttemptsWarning({ attemptsLeft }: { attemptsLeft: number }) {
  const { t } = useTranslator();
  if (attemptsLeft > warnFromAttemptsLeft) return null;
  return (
    <p className="text-[11px] text-warning">
      {t("unlock.pin.attempts", { count: attemptsLeft })}
    </p>
  );
}
