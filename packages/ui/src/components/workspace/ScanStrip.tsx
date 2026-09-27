import { cn } from "cn";
import {
  Copy,
  Download,
  FileText,
  FolderOpen,
  Images,
  Paperclip,
  X,
} from "lucide-react";
import { useCapabilities } from "../../host/capabilities.tsx";
import { useTranslator } from "../../i18n/translator.tsx";
import { scanLimit } from "../../identities/identity.ts";
import { photoSource } from "../../identities/photo.ts";
import type { ScanSummary } from "../../vault-api.ts";
import { Button } from "../ui/button.tsx";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "../ui/dropdown-menu.tsx";

/** What a stored scan can be asked to do; each action reports its own failure. */
export interface ScanActions {
  copy(id: string): void;
  /** Writes an unencrypted copy where the user picks. The caller confirms first. */
  save(id: string): Promise<void>;
}

export interface ScanMenu {
  onCopy: (scan: ScanSummary) => void;
  onSave: (scan: ScanSummary) => void;
}

/** Where a scan to attach comes from; the file dialog also takes a PDF. */
export type ScanSource = "photo" | "file";

/** ScanStrip shows a menu per tile with `menu`, and remove and attach controls with their callbacks. */
export function ScanStrip({
  scans,
  busy,
  className,
  menu,
  onRemove,
  onAttach,
}: {
  scans: ScanSummary[];
  busy: boolean;
  className?: string;
  menu?: ScanMenu;
  onRemove?: (scan: ScanSummary) => void;
  onAttach?: (source: ScanSource) => void;
}) {
  const { t } = useTranslator();
  const { copyScans } = useCapabilities();

  return (
    <ul
      className={cn("flex flex-wrap items-center gap-2", className)}
      aria-label={t("identity.scan.label")}
    >
      {scans.map((scan) => (
        <li key={scan.id} className="relative">
          {menu ? (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button
                  type="button"
                  aria-label={t("identity.scan.actions", { name: scan.name })}
                  title={scan.name}
                  disabled={busy}
                  className={`${tile} outline-none hover:brightness-110 focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:opacity-50`}
                >
                  <TileFace scan={scan} />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start">
                {copyScans && (
                  <DropdownMenuItem onSelect={() => menu.onCopy(scan)}>
                    <Copy />
                    {t("identity.scan.copy")}
                  </DropdownMenuItem>
                )}
                <DropdownMenuItem onSelect={() => menu.onSave(scan)}>
                  <Download />
                  {t("identity.scan.save")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ) : (
            <span className={tile} title={scan.name}>
              <TileFace scan={scan} />
            </span>
          )}
          {onRemove && (
            <button
              type="button"
              aria-label={t("identity.scan.remove", { name: scan.name })}
              title={t("identity.scan.remove", { name: scan.name })}
              disabled={busy}
              onClick={() => onRemove(scan)}
              className="absolute -top-1.5 -right-1.5 flex size-5 items-center justify-center rounded-full border bg-raised text-muted-foreground outline-none hover:bg-tile hover:text-foreground focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:opacity-50"
            >
              <X className="size-3" />
            </button>
          )}
        </li>
      ))}
      {onAttach && scans.length < scanLimit && (
        <li>
          <AttachButton busy={busy} onAttach={onAttach} />
        </li>
      )}
    </ul>
  );
}

function AttachButton({
  busy,
  onAttach,
}: {
  busy: boolean;
  onAttach: (source: ScanSource) => void;
}) {
  const { t } = useTranslator();
  const { photoPicker } = useCapabilities();
  const face = (
    <>
      <Paperclip data-icon="inline-start" />
      {t("identity.scan.attach")}
    </>
  );

  if (!photoPicker) {
    return (
      <Button
        type="button"
        variant="quiet"
        size="pill-sm"
        disabled={busy}
        onClick={() => onAttach("file")}
      >
        {face}
      </Button>
    );
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button type="button" variant="quiet" size="pill-sm" disabled={busy}>
          {face}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start">
        <DropdownMenuItem onSelect={() => onAttach("photo")}>
          <Images />
          {t("identity.scan.from-photos")}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => onAttach("file")}>
          <FolderOpen />
          {t("identity.scan.from-files")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

const tile =
  "flex size-14 items-center justify-center overflow-hidden rounded-[10px] bg-tile";

function TileFace({ scan }: { scan: ScanSummary }) {
  const preview = photoSource(scan.thumbnail);

  if (preview) {
    return (
      <img
        src={preview}
        alt=""
        draggable={false}
        className="size-full object-cover"
      />
    );
  }

  return (
    <span className="flex w-full flex-col items-center gap-1 px-1 text-muted-foreground">
      <FileText className="size-5" aria-hidden="true" />
      <span className="w-full truncate text-center text-[10px]">
        {scan.name}
      </span>
    </span>
  );
}
