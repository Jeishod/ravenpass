import { NotebookText } from "lucide-react";
import { useTranslator } from "../../i18n/translator.tsx";
import type { NoteSummary } from "../../vault-api.ts";
import { ItemList, type ListProps } from "./ItemList.tsx";

export function NoteList(props: ListProps<NoteSummary>) {
  const { t } = useTranslator();

  return (
    <ItemList
      {...props}
      label={t("note.list.label")}
      untitled={t("note.untitled")}
      shape="tile"
      icon={NotebookText}
      detail={(entry) => ({
        text: entry.hidden
          ? t("note.preview.hidden")
          : entry.preview || t("note.preview.empty"),
      })}
    />
  );
}
