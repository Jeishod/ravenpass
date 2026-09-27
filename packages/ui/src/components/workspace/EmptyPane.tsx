import type { ComponentType, ReactNode } from "react";

/** EmptyPane fills a pane that holds nothing yet. */
export function EmptyPane({
  icon: Icon,
  title,
  detail,
  children,
}: {
  icon: ComponentType<{ className?: string }>;
  title: string;
  detail: string;
  children?: ReactNode;
}) {
  return (
    <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-3 px-6 text-center">
      <span
        className="flex size-12 items-center justify-center rounded-xl bg-field text-muted-foreground"
        aria-hidden="true"
      >
        <Icon className="size-6" />
      </span>
      <span>
        <span className="block text-[15px]">{title}</span>
        <span className="mt-1 block max-w-[280px] text-xs leading-[1.5] text-muted-foreground">
          {detail}
        </span>
      </span>
      {children}
    </div>
  );
}
