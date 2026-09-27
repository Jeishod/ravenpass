import { useAnimate, useReducedMotionConfig } from "motion/react";
import { type FormEvent, useId, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { useTranslator } from "../../i18n/translator.tsx";
import { splitWords } from "../../seeds/phrase.ts";
import { ActionBar } from "../AccessShell.tsx";
import { ForwardButton } from "../ForwardButton.tsx";
import { PhraseFields, type PhraseFieldsHandle } from "../PhraseFields.tsx";
import { PhraseEntry } from "../phrase-entry.ts";
import {
  chooseWordPositions,
  matchesWordChallenge,
} from "../recovery-challenge.ts";
import { Button } from "../ui/button.tsx";

/** ConfirmStep asks for recovery key words at positions drawn anew each time it opens. */
export function ConfirmStep({
  phrase,
  onBack,
  onConfirmed,
}: {
  phrase: string;
  onBack: () => void;
  onConfirmed: () => void;
}) {
  const { t } = useTranslator();
  const formID = useId();
  const reduced = useReducedMotionConfig() === true;
  const words = useMemo(() => splitWords(phrase), [phrase]);
  const [positions] = useState(() => chooseWordPositions(words.length));
  const [answers, setAnswers] = useState(() =>
    PhraseEntry.empty(positions.length),
  );
  const [mismatch, setMismatch] = useState(false);
  const [scope, animate] = useAnimate<HTMLDivElement>();
  const fields = useRef<PhraseFieldsHandle>(null);

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (matchesWordChallenge(words, positions, answers.words)) {
      onConfirmed();
      return;
    }
    setAnswers(PhraseEntry.empty(positions.length));
    setMismatch(true);
    toast.error(t("wizard.errors.words-mismatch"));
    fields.current?.focus(0);
    if (!reduced) {
      animate(
        scope.current,
        { x: [0, -10, 10, -6, 6, -2, 0] },
        { duration: 0.42, ease: "easeOut" },
      );
    }
  }

  return (
    <form id={formID} onSubmit={submit} className="flex flex-col gap-3">
      <div ref={scope}>
        <PhraseFields
          ref={fields}
          label={t("wizard.confirm.words")}
          positions={positions.map((position) => position + 1)}
          entry={answers}
          onEntry={(next) => {
            setMismatch(false);
            setAnswers(next);
          }}
          invalid={mismatch}
          autoFocus
        />
      </div>
      <p className="text-center text-[11px] text-faint">
        {t("wizard.confirm.hint")}
      </p>

      <ActionBar>
        <Button type="button" variant="ghost" size="pill" onClick={onBack}>
          {t("wizard.actions.back")}
        </Button>
        <ForwardButton type="submit" form={formID} disabled={!answers.complete}>
          {t("wizard.actions.continue")}
        </ForwardButton>
      </ActionBar>
    </form>
  );
}
