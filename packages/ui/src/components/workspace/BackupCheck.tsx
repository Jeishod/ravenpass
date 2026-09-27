import { useState } from "react";
import { useTranslator } from "../../i18n/translator.tsx";
import { bareField, EditorRow } from "../editor/EditorFields.tsx";
import {
  ResponsiveDialog,
  ResponsiveDialogContent,
  ResponsiveDialogDescription,
  ResponsiveDialogFooter,
  ResponsiveDialogHeader,
  ResponsiveDialogTitle,
} from "../ResponsiveDialog.tsx";
import {
  chooseWordPositions,
  matchesWordChallenge,
} from "../recovery-challenge.ts";
import { Button } from "../ui/button.tsx";
import { Input } from "../ui/input.tsx";

/** BackupCheck picks its random positions on mount; remount it for each check. */
export function BackupCheck({
  open,
  words,
  busy,
  onMatch,
  onClose,
}: {
  open: boolean;
  words: readonly string[];
  busy: boolean;
  onMatch: () => void;
  onClose: () => void;
}) {
  const { t } = useTranslator();
  const [positions] = useState(() => chooseWordPositions(words.length));
  const [answers, setAnswers] = useState(() => positions.map(() => ""));
  const [tried, setTried] = useState(false);
  const [mismatch, setMismatch] = useState(false);

  function answer(index: number, value: string) {
    setMismatch(false);
    setAnswers((current) =>
      current.map((previous, at) => (at === index ? value : previous)),
    );
  }

  function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setTried(true);
    if (answers.some((value) => !value.trim())) return;
    if (!matchesWordChallenge(words, positions, answers)) {
      setMismatch(true);
      return;
    }
    onMatch();
  }

  return (
    <ResponsiveDialog
      open={open}
      dismissible={!busy}
      onOpenChange={(next) => {
        if (!next) onClose();
      }}
    >
      <ResponsiveDialogContent className="sm:max-w-[380px]">
        <form onSubmit={submit} className="grid gap-3" noValidate>
          <ResponsiveDialogHeader>
            <ResponsiveDialogTitle>
              {t("seed.check.open")}
            </ResponsiveDialogTitle>
            <ResponsiveDialogDescription>
              {t("seed.check.detail")}
            </ResponsiveDialogDescription>
          </ResponsiveDialogHeader>
          <div className="overflow-hidden rounded-row bg-field">
            {positions.map((position, index) => {
              const id = `seed-check-${position}`;
              const value = answers[index] ?? "";
              const empty = tried && !value.trim();
              return (
                <EditorRow
                  key={position}
                  label={t("seed.check.word", { number: position + 1 })}
                  htmlFor={id}
                >
                  <Input
                    id={id}
                    className={`${bareField} font-mono`}
                    value={value}
                    onChange={(event) => answer(index, event.target.value)}
                    autoFocus={index === 0}
                    autoComplete="off"
                    autoCapitalize="none"
                    autoCorrect="off"
                    spellCheck={false}
                    aria-invalid={empty ? true : undefined}
                    disabled={busy}
                  />
                  {empty && (
                    <span className="shrink-0 text-[11px] text-destructive">
                      {t("seed.check.empty")}
                    </span>
                  )}
                </EditorRow>
              );
            })}
          </div>
          {mismatch && (
            <p className="text-xs text-warning" role="alert">
              {t("seed.check.mismatch")}
            </p>
          )}
          <ResponsiveDialogFooter>
            <Button
              type="button"
              variant="quiet"
              size="pill"
              disabled={busy}
              onClick={onClose}
            >
              {t("workspace.editor.cancel")}
            </Button>
            <Button type="submit" variant="raised" size="pill" disabled={busy}>
              {t("seed.check.submit")}
            </Button>
          </ResponsiveDialogFooter>
        </form>
      </ResponsiveDialogContent>
    </ResponsiveDialog>
  );
}
