import { cn } from "cn";
import { XIcon } from "lucide-react";
import { createContext, type ReactNode, useContext, useRef } from "react";
import type { AutofillSurface } from "../../autofill/channel.ts";
import { useTranslator } from "../../i18n/translator.tsx";
import {
  ResponsiveDialog,
  ResponsiveDialogContent,
  ResponsiveDialogDescription,
  ResponsiveDialogHeader,
  ResponsiveDialogTitle,
} from "../ResponsiveDialog.tsx";
import { Button } from "../ui/button.tsx";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "../ui/dialog.tsx";

const SurfaceContext = createContext<AutofillSurface>("sheet");

const CoveredContext = createContext(false);

/** AutofillSurfaceProvider tells the screens below it how the host shows the page. */
export function AutofillSurfaceProvider({
  surface,
  children,
}: {
  surface: AutofillSurface;
  children: ReactNode;
}) {
  return (
    <SurfaceContext.Provider value={surface}>
      {children}
    </SurfaceContext.Provider>
  );
}

interface SheetProps {
  title: string;
  /** Left out when the title says enough. */
  description?: string;
  busy: boolean;
  onClose: () => void;
  /** Applies to both forms; a size meant for the dialog alone takes the `sm:` variant. */
  className?: string;
  children: ReactNode;
}

/** CoveredSheet closes the sheets below it while `covered`; their screens keep their state. */
export function CoveredSheet({
  covered,
  children,
}: {
  covered: boolean;
  children: ReactNode;
}) {
  return (
    <CoveredContext.Provider value={covered}>
      {children}
    </CoveredContext.Provider>
  );
}

/** AutofillSheet is a screen of the autofill page: a drawer or dialog over the app being filled, or the whole window; neither closes while `busy`. */
export function AutofillSheet(props: SheetProps) {
  const surface = useContext(SurfaceContext);
  const covered = useContext(CoveredContext);
  return surface === "window" ? (
    <WindowSheet {...props} open={!covered} />
  ) : (
    <OverlaySheet {...props} open={!covered} />
  );
}

function WindowSheet({
  open,
  title,
  description,
  busy,
  onClose,
  className,
  children,
}: SheetProps & { open: boolean }) {
  const { t } = useTranslator();
  const content = useRef<HTMLDivElement>(null);

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next && !busy) onClose();
      }}
    >
      <DialogContent
        ref={content}
        showCloseButton={false}
        {...(description ? {} : { "aria-describedby": undefined })}
        // Radix focuses the close button by default and draws its focus ring on every opening.
        onOpenAutoFocus={(event) => {
          event.preventDefault();
          content.current?.focus();
        }}
        className={cn(
          "inset-0 top-0 left-0 flex size-full max-w-none translate-x-0 translate-y-0 flex-col gap-0 rounded-none border-0 bg-background p-0 shadow-none data-[state=closed]:animate-none data-[state=open]:animate-none sm:max-w-none",
          className,
        )}
      >
        <header className="flex items-start gap-3 px-[18px] pt-[18px] pb-3">
          <div className="grid min-w-0 flex-1 gap-2">
            <DialogTitle asChild>
              <h1>{title}</h1>
            </DialogTitle>
            {description && (
              <DialogDescription className="break-words">
                {description}
              </DialogDescription>
            )}
          </div>
          <Button
            type="button"
            variant="ghost"
            size="icon-xs"
            className="-mt-1 -mr-1 text-muted-foreground"
            aria-label={t("autofill.close")}
            disabled={busy}
            onClick={onClose}
          >
            <XIcon />
          </Button>
        </header>
        <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto px-[18px] pb-[18px] [&>[data-slot=dialog-footer]]:mt-auto">
          {children}
        </div>
      </DialogContent>
    </Dialog>
  );
}

function OverlaySheet({
  open,
  title,
  description,
  busy,
  onClose,
  className,
  children,
}: SheetProps & { open: boolean }) {
  return (
    <ResponsiveDialog
      open={open}
      dismissible={!busy}
      onOpenChange={(next) => {
        if (!next) onClose();
      }}
    >
      <ResponsiveDialogContent
        className={cn("bg-background sm:max-w-[400px]", className)}
      >
        <ResponsiveDialogHeader>
          <ResponsiveDialogTitle>{title}</ResponsiveDialogTitle>
          {description && (
            <ResponsiveDialogDescription>
              {description}
            </ResponsiveDialogDescription>
          )}
        </ResponsiveDialogHeader>
        {children}
      </ResponsiveDialogContent>
    </ResponsiveDialog>
  );
}

/** AutofillNote says why the screen's last act did not go through. */
export function AutofillNote({ children }: { children: string }) {
  return (
    <p role="alert" className="text-[11px] text-destructive">
      {children}
    </p>
  );
}
