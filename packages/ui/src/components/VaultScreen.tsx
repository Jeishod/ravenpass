import type { ReactNode, Ref } from "react";
import { AccessHeader } from "./AccessShell.tsx";
import { VaultMark } from "./VaultArt.tsx";

/** VaultScreen fills the window before the workspace; a compact one scrolls when the keyboard leaves too little height. */
export function VaultScreen({
  title,
  description,
  markRef,
  children,
}: {
  title: string;
  description?: ReactNode;
  markRef?: Ref<HTMLDivElement>;
  children?: ReactNode;
}) {
  return (
    <main className="flex h-dvh min-h-0 min-w-0 flex-col overflow-hidden bg-background text-foreground">
      <AccessHeader />
      <div className="flex min-h-0 flex-1 flex-col items-center justify-center overflow-y-auto px-10 pb-13 text-center max-sm:justify-center-safe max-sm:px-5">
        <div ref={markRef}>
          <VaultMark className="mb-6" />
        </div>
        <h1 className="max-w-[520px] text-[28px] leading-tight font-semibold tracking-[-0.025em] text-balance">
          {title}
        </h1>
        {description && (
          <div className="mt-2 max-w-[460px] text-[13px] leading-[1.5] text-balance text-muted-foreground">
            {description}
          </div>
        )}
        {children}
      </div>
    </main>
  );
}
