import { cn } from "cn";
import {
  ArchiveRestore,
  FileLock2,
  FolderOpen,
  HardDrive,
  LockKeyhole,
  type LucideIcon,
  Save,
  TriangleAlert,
} from "lucide-react";
import type { ReactNode } from "react";
import type { MessageKey } from "../../i18n/messages.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import { CrossFade } from "../../motion/CrossFade.tsx";
import { roll } from "../../motion/timings.ts";
import {
  type StorageWarning,
  storageKindDetail,
  storageWarnings,
} from "../../storage/kinds.ts";
import type { StorageKind, StorageStatus } from "../../vault-api.ts";
import { ActionBar } from "../AccessShell.tsx";
import { ForwardButton } from "../ForwardButton.tsx";
import { Button } from "../ui/button.tsx";

const kindIcons: Record<StorageKind, LucideIcon> = {
  "local-file": HardDrive,
  document: FolderOpen,
};

const facts: { icon: LucideIcon; message: MessageKey }[] = [
  { icon: LockKeyhole, message: "wizard.storage.fact-encrypted" },
  { icon: Save, message: "wizard.storage.fact-saved" },
  { icon: ArchiveRestore, message: "wizard.storage.fact-backup" },
];

const warnings: Record<StorageWarning, MessageKey> = {
  unrestricted: "wizard.storage.unrestricted",
  concurrent: "storage.type.document.concurrent",
};

const row = "flex items-center gap-3.5 p-3.5 pr-4";

const card = cn(row, "rounded-2xl bg-field");

const tile =
  "flex size-11 shrink-0 items-center justify-center rounded-xl border border-white/8 bg-linear-to-b from-[#2a2b31] to-[#141518]";

/** StorageStep shows where the vault file will live; its forward action starts creation. */
export function StorageStep({
  storage,
  choosing,
  busy,
  onChoose,
  leave,
  onContinue,
}: {
  storage: StorageStatus | null;
  choosing: boolean;
  busy: boolean;
  /** Asks for a location of `kind`: another type, or another file of the current one. */
  onChoose: (kind: StorageKind) => void;
  /** Present when the walk can be left: back to where it started, or cancelled. */
  leave?: { label: string; run: () => void };
  onContinue: () => void;
}) {
  const { t } = useTranslator();
  const path = storage?.path ?? "";
  const kind = storage?.kind || null;
  const chosen = storage?.chosen ?? [];

  return (
    <div className="flex flex-col gap-5">
      {storage && kind && (
        <Section title={t("wizard.storage.type")}>
          {storage.kinds.length > 1 ? (
            <KindChoice
              label={t("wizard.storage.type")}
              kinds={storage.kinds}
              chosen={chosen}
              value={kind}
              disabled={choosing || busy}
              onChoose={onChoose}
            />
          ) : (
            <div className={card}>
              <KindLabel kind={kind} chosen={chosen} />
            </div>
          )}
        </Section>
      )}

      <Section title={t("wizard.storage.location")}>
        <div className={card}>
          <span aria-hidden="true" className={tile}>
            <FileLock2 className="size-5 text-muted-foreground" />
          </span>
          <div className="min-w-0 flex-1">
            <span className="relative flex h-5 overflow-hidden">
              <CrossFade id={path} presence={roll} className="min-w-0 flex-1">
                <span className="block truncate text-sm leading-5 font-medium">
                  {path ? storage?.name : t("wizard.storage.file-empty")}
                </span>
              </CrossFade>
            </span>
            {path && (
              <p
                className={cn(
                  "truncate text-[11px] text-muted-foreground select-text",
                  kind === "local-file" && "font-mono",
                )}
                title={storage?.place}
              >
                {storage?.place}
              </p>
            )}
          </div>
          {kind && chosen.includes(kind) && (
            <Button
              type="button"
              variant="quiet"
              size="pill-sm"
              onClick={() => onChoose(kind)}
              disabled={choosing || busy}
            >
              {choosing
                ? t("wizard.storage.change-busy")
                : t("wizard.storage.change")}
            </Button>
          )}
        </div>
      </Section>

      <ul className="flex flex-col gap-2.5 px-1">
        {facts.map(({ icon: Icon, message }) => (
          <li
            key={message}
            className="flex items-start gap-2.5 text-[13px] text-muted-foreground"
          >
            <Icon
              className="mt-0.5 size-4 shrink-0 text-faint"
              aria-hidden="true"
            />
            {t(message)}
          </li>
        ))}
        {storage &&
          storageWarnings(storage).map((warning) => (
            <li
              key={warning}
              className="flex items-start gap-2.5 text-[13px] text-warning"
            >
              <TriangleAlert
                className="mt-0.5 size-4 shrink-0"
                aria-hidden="true"
              />
              {t(warnings[warning])}
            </li>
          ))}
      </ul>

      <ActionBar>
        {leave && (
          <Button
            type="button"
            variant="ghost"
            size="pill"
            onClick={leave.run}
            disabled={busy || choosing}
          >
            {leave.label}
          </Button>
        )}
        <ForwardButton
          type="button"
          onClick={onContinue}
          disabled={!storage || choosing || busy}
        >
          {busy ? t("wizard.actions.preparing") : t("wizard.actions.continue")}
        </ForwardButton>
      </ActionBar>
    </div>
  );
}

/** KindChoice marks the current storage type; a cancelled location request leaves the mark in place. */
function KindChoice({
  label,
  kinds,
  chosen,
  value,
  disabled,
  onChoose,
}: {
  label: string;
  kinds: readonly StorageKind[];
  chosen: readonly StorageKind[];
  value: StorageKind;
  disabled: boolean;
  onChoose: (kind: StorageKind) => void;
}) {
  return (
    <div
      role="radiogroup"
      aria-label={label}
      className="overflow-hidden rounded-2xl bg-field"
    >
      {kinds.map((kind) => {
        const selected = kind === value;
        return (
          <label
            key={kind}
            className={cn(
              row,
              "border-b transition-colors last:border-b-0 hover:bg-field-hover has-focus-visible:bg-field-hover has-disabled:pointer-events-none has-disabled:opacity-50",
            )}
          >
            <input
              type="radio"
              name="storage-kind"
              className="peer sr-only"
              checked={selected}
              disabled={disabled}
              onChange={() => onChoose(kind)}
            />
            <KindLabel kind={kind} chosen={chosen} />
            <span
              aria-hidden="true"
              className={cn(
                "flex size-[18px] shrink-0 items-center justify-center rounded-full border peer-focus-visible:ring-[3px] peer-focus-visible:ring-ring/50",
                selected ? "border-primary bg-primary" : "border-input",
              )}
            >
              {selected && (
                <span className="size-2 rounded-full bg-primary-foreground" />
              )}
            </span>
          </label>
        );
      })}
    </div>
  );
}

function KindLabel({
  kind,
  chosen,
}: {
  kind: StorageKind;
  chosen: readonly StorageKind[];
}) {
  const { t } = useTranslator();
  const Icon = kindIcons[kind];
  return (
    <>
      <span aria-hidden="true" className={tile}>
        <Icon className="size-5 text-muted-foreground" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="block truncate text-sm font-medium">
          {t(`storage.type.${kind}`)}
        </span>
        <span className="block text-xs leading-snug text-muted-foreground">
          {t(storageKindDetail(kind, chosen))}
        </span>
      </span>
    </>
  );
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-2">
      <h2 className="px-1 text-xs text-muted-foreground">{title}</h2>
      {children}
    </section>
  );
}
