import type { MessageKey } from "../i18n/messages.ts";
import type { ImportSource } from "../vault-api.ts";
import aliasvaultLogo from "./logos/aliasvault.svg";
import bitwardenLogo from "./logos/bitwarden.svg";

export interface ExportStep {
  readonly title: MessageKey;
  readonly detail: MessageKey;
}

export interface ImportGrouping {
  /** The detail of the review row counting new groups. */
  readonly from: MessageKey;
  /** The title of the option that makes groups. */
  readonly option: MessageKey;
  /** The detail of the option that makes groups. */
  readonly detail: MessageKey;
  /** The title of the row counting names that cannot become groups. */
  readonly dropped: MessageKey;
}

/** `name` is the same in every language; `logo` is the app's icon as it ships it. */
export interface ImportSourceGuide {
  readonly id: ImportSource;
  readonly name: string;
  readonly logo: string;
  /** `full` is its own tile; `mark` has no background and sits on Ravenpass's tile. */
  readonly logoFit: "full" | "mark";
  readonly formats: MessageKey;
  readonly steps: readonly ExportStep[];
  readonly grouping: ImportGrouping;
  /** What the import puts into notes and what stays behind. */
  readonly review: MessageKey;
}

/** In the order the Import section lists them. */
const guides: Record<ImportSource, Omit<ImportSourceGuide, "id">> = {
  bitwarden: {
    name: "Bitwarden",
    logo: bitwardenLogo,
    logoFit: "full",
    formats: "settings.import.bitwarden.formats",
    steps: [
      {
        title: "settings.import.bitwarden.step.open",
        detail: "settings.import.bitwarden.step.open.detail",
      },
      {
        title: "settings.import.bitwarden.step.format",
        detail: "settings.import.bitwarden.step.format.detail",
      },
    ],
    grouping: {
      from: "settings.import.folders",
      option: "settings.import.option.groups",
      detail: "settings.import.option.groups.detail",
      dropped: "settings.import.dropped",
    },
    review: "settings.import.bitwarden.review",
  },
  aliasvault: {
    name: "AliasVault",
    logo: aliasvaultLogo,
    logoFit: "mark",
    formats: "settings.import.aliasvault.formats",
    steps: [
      {
        title: "settings.import.aliasvault.step.open",
        detail: "settings.import.aliasvault.step.open.detail",
      },
      {
        title: "settings.import.aliasvault.step.format",
        detail: "settings.import.aliasvault.step.format.detail",
      },
    ],
    grouping: {
      from: "settings.import.folders-tags",
      option: "settings.import.option.groups-tags",
      detail: "settings.import.option.groups-tags.detail",
      dropped: "settings.import.dropped-tags",
    },
    review: "settings.import.aliasvault.review",
  },
};

export const importSources: readonly ImportSourceGuide[] = (
  Object.keys(guides) as ImportSource[]
).map((id) => ({ id, ...guides[id] }));
