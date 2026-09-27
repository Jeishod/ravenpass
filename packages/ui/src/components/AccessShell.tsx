import { cn } from "cn";
import { LockKeyhole } from "lucide-react";
import {
  type ComponentType,
  createContext,
  type ReactNode,
  useContext,
  useState,
} from "react";
import { createPortal } from "react-dom";
import { useCompactLayout } from "../host/compact.ts";
import { useTranslator } from "../i18n/translator.tsx";
import { CrossFade } from "../motion/CrossFade.tsx";
import {
  SelectionGroup,
  SelectionIndicator,
} from "../motion/SelectionIndicator.tsx";
import { StepTransition } from "../motion/StepTransition.tsx";
import { roll } from "../motion/timings.ts";
import { LanguageMenu } from "./LanguageMenu.tsx";
import { ScrollArea } from "./ui/scroll-area.tsx";

const ActionSlot = createContext<HTMLElement | null>(null);

export type AccessLayout = "window" | "popup" | "dialog";

interface Look {
  content: string;
  icon: string;
  glyph: string;
  title: string;
  description: string;
  children: string;
}

const windowLook: Look = {
  content: "flex w-full max-w-[440px] flex-col items-center text-center",
  icon: "mb-4 flex size-12 items-center justify-center rounded-2xl border border-white/8 bg-linear-to-b from-[#2a2b31] to-[#141518] text-foreground",
  glyph: "size-[22px]",
  title: "text-[24px] leading-tight font-semibold tracking-[-0.02em]",
  description: "mt-1.5 max-w-[380px] text-[13px] leading-[1.5]",
  children: "mt-6 text-left",
};

/** `compact` is the window layout on a phone's screen. */
const looks: Record<AccessLayout | "compact", Look> = {
  window: windowLook,
  compact: {
    ...windowLook,
    title: "text-[20px] leading-tight font-semibold tracking-[-0.02em]",
    children: "mt-5 text-left",
  },
  popup: {
    content: "flex w-full flex-col items-center text-center",
    icon: "action-fill mb-[11px] flex size-[52px] items-center justify-center rounded-full",
    glyph: "size-6",
    title: "text-[17px]",
    description: "mt-0.5 text-[11px]",
    children: "mt-3.5",
  },
  dialog: {
    ...windowLook,
    title: "text-[18px] leading-tight font-semibold tracking-[-0.02em]",
    children: "mt-5 text-left",
  },
};

/** On a phone the system bars draw over the page's edges, so the compact frame pads by the safe-area insets. */
const frames = {
  window: {
    column: "justify-center px-10 pb-6",
    footer: "h-16 items-center justify-between gap-3 px-6",
    actions: "",
  },
  compact: {
    column: "px-5 pt-[calc(env(safe-area-inset-top)+1rem)] pb-5",
    footer:
      "flex-col gap-3 px-5 pt-3 pb-[calc(env(safe-area-inset-bottom)+1rem)]",
    actions: "[&>*]:min-h-11 [&>:last-child]:flex-1",
  },
};

export interface AccessShellProps {
  title: string;
  description?: string;
  icon?: ComponentType<{ className?: string }>;
  steps?: readonly string[];
  activeStep?: number;
  /** `popup` is the browser extension's popup, which the browser sizes to its content; `dialog` sits inside a dialog. */
  layout?: AccessLayout;
  languageMenu?: boolean;
  children: ReactNode;
}

export function AccessShell({
  title,
  description,
  icon: Icon = LockKeyhole,
  steps = [],
  activeStep = 0,
  layout = "window",
  languageMenu = true,
  children,
}: AccessShellProps) {
  const [actions, setActions] = useState<HTMLDivElement | null>(null);
  const compact = useCompactLayout() && layout === "window";
  const look = looks[compact ? "compact" : layout];

  const content = (
    <StepTransition step={activeStep} className={look.content}>
      <span className="relative flex flex-col items-center">
        {layout === "window" && (
          <span
            aria-hidden="true"
            className="pointer-events-none absolute top-1/2 left-1/2 -mt-[150px] -ml-[150px] size-[300px] rounded-full bg-radial from-foreground/7 to-transparent to-60%"
          />
        )}
        <span className={cn("relative", look.icon)} aria-hidden="true">
          <Icon className={look.glyph} />
        </span>
      </span>
      <h1 className={look.title}>{title}</h1>
      {description && (
        <p className={cn(look.description, "text-muted-foreground")}>
          {description}
        </p>
      )}
      <ActionSlot.Provider value={actions}>
        <div className={cn("w-full", look.children)}>{children}</div>
      </ActionSlot.Provider>
    </StepTransition>
  );

  if (layout === "popup") {
    return (
      <main className="flex min-w-0 flex-col bg-background text-foreground">
        <div className="flex h-13 shrink-0 items-center justify-end px-3.5">
          {languageMenu && <LanguageMenu />}
        </div>
        <div className="px-5 pb-5">{content}</div>
      </main>
    );
  }

  if (layout === "dialog") {
    return (
      <div className="flex min-w-0 flex-col text-foreground">
        {content}
        <footer className="mt-6 flex items-center justify-between gap-3 max-sm:flex-col max-sm:items-stretch">
          {steps.length > 1 ? (
            <StepProgress steps={steps} activeStep={activeStep} />
          ) : (
            <span />
          )}
          <div
            ref={setActions}
            className="flex items-center justify-end gap-2 max-sm:[&>*]:min-h-11 max-sm:[&>:last-child]:flex-1"
          />
        </footer>
      </div>
    );
  }

  const frame = frames[compact ? "compact" : "window"];
  const header = (
    <AccessHeader
      languageMenu={languageMenu}
      className={cn(compact && "absolute top-0 right-0 z-10")}
    />
  );

  return (
    <main className="flex h-dvh min-h-0 min-w-0 flex-col overflow-hidden bg-background text-foreground">
      {!compact && header}
      <ScrollArea className="min-h-0 flex-1">
        {compact && header}
        <div className={cn("flex flex-1 flex-col items-center", frame.column)}>
          {content}
        </div>
      </ScrollArea>
      <footer className={cn("flex shrink-0", frame.footer)}>
        {steps.length > 1 ? (
          <StepProgress steps={steps} activeStep={activeStep} />
        ) : (
          !compact && <span />
        )}
        <div
          ref={setActions}
          className={cn("flex items-center gap-2", frame.actions)}
        />
      </footer>
    </main>
  );
}

/** AccessHeader is the top strip of an access screen; it drags the title-bar-less desktop window. */
export function AccessHeader({
  languageMenu = true,
  className,
}: {
  languageMenu?: boolean;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "drag-region flex h-13 shrink-0 items-center justify-end px-3.5 max-sm:box-content max-sm:pt-[env(safe-area-inset-top)]",
        className,
      )}
    >
      {languageMenu && <LanguageMenu />}
    </div>
  );
}

function StepProgress({
  steps,
  activeStep,
}: {
  steps: readonly string[];
  activeStep: number;
}) {
  const { t } = useTranslator();
  const current = steps[activeStep] ?? "";

  return (
    <div className="flex items-center gap-3">
      <ol
        className="flex items-center gap-1"
        aria-label={t("wizard.steps.label")}
      >
        <SelectionGroup id="steps">
          {steps.map((step, index) => (
            <li
              key={step}
              aria-current={index === activeStep ? "step" : undefined}
              className={cn(
                "relative h-1 w-5 rounded-full transition-colors duration-300 motion-reduce:transition-none",
                index < activeStep ? "bg-foreground/30" : "bg-quiet",
              )}
            >
              {index === activeStep && (
                <SelectionIndicator className="rounded-full bg-foreground" />
              )}
              <span className="sr-only">{step}</span>
            </li>
          ))}
        </SelectionGroup>
      </ol>
      <span
        aria-hidden="true"
        className="relative flex h-4 overflow-hidden text-xs leading-4 text-muted-foreground"
      >
        <CrossFade id={current} presence={roll}>
          {current}
        </CrossFade>
      </span>
    </div>
  );
}

export function ActionBar({ children }: { children: ReactNode }) {
  const slot = useContext(ActionSlot);
  return slot ? createPortal(children, slot) : null;
}
