import {
  ResponsiveDialog,
  ResponsiveDialogAction,
  ResponsiveDialogCancel,
  ResponsiveDialogContent,
  ResponsiveDialogDescription,
  ResponsiveDialogFooter,
  ResponsiveDialogHeader,
  ResponsiveDialogTitle,
} from "./ResponsiveDialog.tsx";

/** ConfirmDialog asks before an irreversible act and stays open until its opener closes it. */
export function ConfirmDialog({
  open,
  title,
  detail,
  confirm,
  cancel,
  destructive = false,
  busy,
  onConfirm,
  onCancel,
}: {
  open: boolean;
  title: string;
  detail: string;
  confirm: string;
  cancel: string;
  destructive?: boolean;
  busy: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  return (
    <ResponsiveDialog
      role="alertdialog"
      open={open}
      onOpenChange={(next) => {
        if (!next) onCancel();
      }}
    >
      <ResponsiveDialogContent>
        <ResponsiveDialogHeader>
          <ResponsiveDialogTitle>{title}</ResponsiveDialogTitle>
          <ResponsiveDialogDescription>{detail}</ResponsiveDialogDescription>
        </ResponsiveDialogHeader>
        <ResponsiveDialogFooter>
          <ResponsiveDialogCancel disabled={busy}>
            {cancel}
          </ResponsiveDialogCancel>
          <ResponsiveDialogAction
            variant={destructive ? "destructive" : "raised"}
            disabled={busy}
            onClick={(event) => {
              event.preventDefault();
              onConfirm();
            }}
          >
            {confirm}
          </ResponsiveDialogAction>
        </ResponsiveDialogFooter>
      </ResponsiveDialogContent>
    </ResponsiveDialog>
  );
}
