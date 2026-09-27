import { cn } from "cn";
import {
  type ComponentProps,
  createContext,
  type ReactNode,
  useContext,
  useMemo,
} from "react";
import { useSystemBack } from "../host/back.ts";
import { useCompactLayout } from "../host/compact.ts";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "./ui/alert-dialog.tsx";
import { Button } from "./ui/button.tsx";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "./ui/dialog.tsx";
import {
  Drawer,
  DrawerClose,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
} from "./ui/drawer.tsx";

interface Presentation {
  /** Shown as a drawer from the bottom of a compact screen. */
  drawer: boolean;
  /** Asks for a decision between an explicit cancel and confirm. */
  alert: boolean;
}

const PresentationContext = createContext<Presentation>({
  drawer: false,
  alert: false,
});

/** Spread on an element with drags of its own, or dragging it pulls the drawer closed. */
export const ownsDrag = { "data-vaul-no-drag": "" } as const;

/** ResponsiveDialog is a drawer on a compact screen and a dialog on a wider one; the system back gesture closes it. */
export function ResponsiveDialog({
  open,
  onOpenChange,
  dismissible = true,
  role = "dialog",
  children,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Whether Escape, a tap outside or a swipe down may close it. */
  dismissible?: boolean;
  /** An `alertdialog` footer holds `ResponsiveDialogCancel` and `ResponsiveDialogAction`. */
  role?: "dialog" | "alertdialog";
  children: ReactNode;
}) {
  const drawer = useCompactLayout();
  const alert = role === "alertdialog";
  const presentation = useMemo(() => ({ drawer, alert }), [drawer, alert]);

  function change(next: boolean) {
    if (next || dismissible) onOpenChange(next);
  }

  useSystemBack(() => change(false), open, !dismissible);

  let root: ReactNode;
  if (drawer) {
    root = (
      <Drawer open={open} onOpenChange={change} dismissible={dismissible}>
        {children}
      </Drawer>
    );
  } else if (alert) {
    root = (
      <AlertDialog open={open} onOpenChange={change}>
        {children}
      </AlertDialog>
    );
  } else {
    root = (
      <Dialog open={open} onOpenChange={change}>
        {children}
      </Dialog>
    );
  }

  return (
    <PresentationContext.Provider value={presentation}>
      {root}
    </PresentationContext.Provider>
  );
}

export function ResponsiveDialogContent({
  className,
  showCloseButton,
  children,
}: {
  /** Applies to both forms; a width meant for the dialog alone takes the `sm:` variant. */
  className?: string;
  /** Applies to a dialog that is not an alert; a drawer never shows one. */
  showCloseButton?: boolean;
  children: ReactNode;
}) {
  const { drawer, alert } = useContext(PresentationContext);

  if (drawer) {
    return (
      <DrawerContent
        className={className}
        role={alert ? "alertdialog" : "dialog"}
      >
        <div className="grid min-h-0 gap-3 overflow-y-auto px-[18px] pt-3 pb-[calc(env(safe-area-inset-bottom)+18px)]">
          {children}
        </div>
      </DrawerContent>
    );
  }
  if (alert) {
    return (
      <AlertDialogContent className={className}>{children}</AlertDialogContent>
    );
  }
  return (
    <DialogContent className={className} showCloseButton={showCloseButton}>
      {children}
    </DialogContent>
  );
}

export function ResponsiveDialogHeader({
  className,
  ...props
}: ComponentProps<"div">) {
  const { drawer, alert } = useContext(PresentationContext);

  if (drawer) {
    return <DrawerHeader className={cn("p-0", className)} {...props} />;
  }
  if (alert) return <AlertDialogHeader className={className} {...props} />;
  return <DialogHeader className={className} {...props} />;
}

export function ResponsiveDialogTitle(
  props: ComponentProps<typeof DialogTitle>,
) {
  const { drawer, alert } = useContext(PresentationContext);

  if (drawer) return <DrawerTitle {...props} />;
  if (alert) return <AlertDialogTitle {...props} />;
  return <DialogTitle {...props} />;
}

export function ResponsiveDialogDescription(
  props: ComponentProps<typeof DialogDescription>,
) {
  const { drawer, alert } = useContext(PresentationContext);

  if (drawer) return <DrawerDescription {...props} />;
  if (alert) return <AlertDialogDescription {...props} />;
  return <DialogDescription {...props} />;
}

/** In a drawer the actions stack full width, the main one on top. */
export function ResponsiveDialogFooter({
  className,
  ...props
}: ComponentProps<"div">) {
  const { drawer, alert } = useContext(PresentationContext);

  if (drawer) {
    return (
      <DrawerFooter
        className={cn("flex-col-reverse p-0 pt-2 [&>*]:min-h-11", className)}
        {...props}
      />
    );
  }
  if (alert) return <AlertDialogFooter className={className} {...props} />;
  return <DialogFooter className={className} {...props} />;
}

type CloseButtonProps = Omit<ComponentProps<typeof Button>, "asChild">;

/** Closes without acting; an alert's cancel takes the focus on a wide screen. */
export function ResponsiveDialogCancel({
  variant = "quiet",
  size = "pill",
  ...props
}: CloseButtonProps) {
  return <CloseButton kind="cancel" variant={variant} size={size} {...props} />;
}

/** A button that acts, then closes unless its click handler prevents the default. */
export function ResponsiveDialogAction({
  variant = "default",
  size = "pill",
  ...props
}: CloseButtonProps) {
  return <CloseButton kind="action" variant={variant} size={size} {...props} />;
}

function CloseButton({
  kind,
  ...props
}: CloseButtonProps & { kind: "cancel" | "action" }) {
  const { drawer, alert } = useContext(PresentationContext);

  if (drawer) {
    return (
      <DrawerClose asChild>
        <Button {...props} />
      </DrawerClose>
    );
  }
  if (alert) {
    return kind === "cancel" ? (
      <AlertDialogCancel {...props} />
    ) : (
      <AlertDialogAction {...props} />
    );
  }
  return (
    <DialogClose asChild>
      <Button {...props} />
    </DialogClose>
  );
}
