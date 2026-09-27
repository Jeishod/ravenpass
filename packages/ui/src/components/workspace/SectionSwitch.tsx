import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import type { WorkspaceSection } from "../../workspace/sections.ts";
import { Segmented } from "./Segmented.tsx";

/** What each section is called, in the switch and above the list it shows. */
export const sectionLabels: Record<WorkspaceSection, MessageKey> = {
  all: "workspace.section.all",
  recent: "workspace.section.recent",
  pinned: "workspace.section.pinned",
};

const sections = ["all", "recent", "pinned"] as const;

export function SectionSwitch({
  section,
  onSection,
  stretch,
}: {
  section: WorkspaceSection;
  onSection: (section: WorkspaceSection) => void;
  /** As in `Segmented`. */
  stretch?: boolean;
}) {
  const { t } = useTranslator();

  return (
    <Segmented
      id="sections"
      legend={t("workspace.sections.label")}
      options={sections.map((id) => ({
        value: id,
        label: t(sectionLabels[id]),
      }))}
      value={section}
      stretch={stretch}
      onChange={onSection}
    />
  );
}
