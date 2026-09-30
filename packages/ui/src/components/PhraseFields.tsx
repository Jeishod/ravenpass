import { cn } from "cn";
import { type Ref, useImperativeHandle, useRef } from "react";
import { useTranslator } from "../i18n/translator.tsx";
import { bareField } from "./editor/EditorFields.tsx";
import { PhraseCell, PhraseGrid } from "./PhraseGrid.tsx";
import type { PhraseEdit, PhraseEntry } from "./phrase-entry.ts";
import { Input } from "./ui/input.tsx";

/** What a form asks of its PhraseFields. */
export interface PhraseFieldsHandle {
  /** Puts the caret at the end of field `index`. */
  focus(index: number): void;
}

function focusEnd(field: HTMLInputElement | null | undefined) {
  if (!field) return;
  field.focus();
  field.setSelectionRange(field.value.length, field.value.length);
}

/** PhraseFields takes a phrase in linked numbered fields; the keys and pasting follow `PhraseEntry`. */
export function PhraseFields({
  ref,
  label,
  positions,
  entry,
  onEntry,
  concealed = false,
  invalid = false,
  disabled = false,
  autoFocus = false,
}: {
  ref?: Ref<PhraseFieldsHandle>;
  /** What the words make together, such as the recovery key. */
  label: string;
  /** One-based phrase position of each field's word; left out, the fields hold the whole phrase. */
  positions?: readonly number[];
  entry: PhraseEntry;
  onEntry: (entry: PhraseEntry) => void;
  /** Hides the words as a password field hides its characters. */
  concealed?: boolean;
  invalid?: boolean;
  disabled?: boolean;
  /** Focuses the first field once the fields show. */
  autoFocus?: boolean;
}) {
  const { t } = useTranslator();
  const fields = useRef<(HTMLInputElement | null)[]>([]);
  const last = entry.words.length - 1;

  useImperativeHandle(
    ref,
    () => ({ focus: (index) => focusEnd(fields.current[index]) }),
    [],
  );

  function apply(index: number, edit: PhraseEdit) {
    onEntry(edit.entry);
    if (edit.focus !== index) focusEnd(fields.current[edit.focus]);
  }

  return (
    <PhraseGrid label={label} count={entry.words.length}>
      {entry.words.map((word, index) => {
        const position = positions?.[index] ?? index + 1;
        return (
          <PhraseCell
            key={position}
            position={position}
            className="items-center py-0 focus-within:inset-ring focus-within:inset-ring-foreground/14 has-aria-invalid:inset-ring has-aria-invalid:inset-ring-destructive/60"
          >
            <Input
              ref={(field) => {
                fields.current[index] = field;
              }}
              className={cn(bareField, "h-8 font-medium max-sm:h-10")}
              type={concealed ? "password" : "text"}
              aria-label={t("phrase.word", { number: position })}
              aria-invalid={invalid || undefined}
              value={word}
              onChange={(event) =>
                apply(index, entry.enter(index, event.target.value))
              }
              onPaste={(event) => {
                const edit = entry.paste(
                  index,
                  event.clipboardData.getData("text"),
                );
                if (!edit) return;
                event.preventDefault();
                apply(index, edit);
              }}
              onKeyDown={(event) => {
                const target = entry.keyMove(index, event.key);
                if (target === null) return;
                event.preventDefault();
                focusEnd(fields.current[target]);
              }}
              enterKeyHint={index < last ? "next" : "done"}
              autoFocus={autoFocus && index === 0}
              autoComplete="off"
              autoCapitalize="none"
              autoCorrect="off"
              spellCheck={false}
              disabled={disabled}
            />
          </PhraseCell>
        );
      })}
    </PhraseGrid>
  );
}
