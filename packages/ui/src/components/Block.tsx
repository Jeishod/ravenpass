import { cn } from "cn";
import {
  ChevronRight,
  ExternalLink,
  Info,
  type LucideIcon,
} from "lucide-react";
import { createContext, type ReactNode, useContext } from "react";

const WrapTextContext = createContext(false);

export function WrapBlockText({ children }: { children: ReactNode }) {
  return <WrapTextContext value={true}>{children}</WrapTextContext>;
}

/** Block is one surface of related lines with hairline dividers. */
export function Block({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("overflow-hidden rounded-row bg-field", className)}>
      {children}
    </div>
  );
}

/** The class of an inline control, a menu or a field, sized to its block row. */
export const blockControl = "h-7 rounded-md border-0 bg-tile px-2.5 text-xs";

const rowLine = "flex min-h-11 items-center border-b px-[13px] last:border-b-0";

function RowText({
  title,
  detail,
  htmlFor,
  wrap,
}: {
  title: string;
  detail?: ReactNode;
  htmlFor?: string;
  wrap?: boolean;
}) {
  const wrapByDefault = useContext(WrapTextContext);
  const wrapping = wrap ?? wrapByDefault;
  const titleClass = cn(
    "block text-[13px]",
    wrapByDefault ? "font-normal whitespace-normal break-words" : "truncate",
  );
  return (
    <span className={cn("min-w-0 flex-1", wrapByDefault && "basis-[140px]")}>
      {htmlFor ? (
        <label htmlFor={htmlFor} className={titleClass}>
          {title}
        </label>
      ) : (
        <span className={titleClass}>{title}</span>
      )}
      {detail && (
        <span
          className={cn(
            "block text-[11px] text-muted-foreground",
            wrapByDefault ? "mt-1 font-normal" : "mt-0.5",
            wrapping
              ? "whitespace-normal break-words leading-[1.5]"
              : "truncate",
          )}
        >
          {detail}
        </span>
      )}
    </span>
  );
}

export function BlockRow({
  title,
  detail,
  htmlFor,
  leading,
  wrap,
  children,
}: {
  title: string;
  detail?: ReactNode;
  /** Set when the row's control is a form field, so the title labels it. */
  htmlFor?: string;
  /** What stands before the title, such as an icon tile. */
  leading?: ReactNode;
  /** Set when the detail is a sentence to read in full. */
  wrap?: boolean;
  children?: ReactNode;
}) {
  const wrapping = useContext(WrapTextContext);
  return (
    <div
      className={cn(
        rowLine,
        wrapping ? "flex-wrap gap-3.5 py-[11px]" : "gap-2.5 py-2",
      )}
    >
      {leading}
      <RowText title={title} detail={detail} htmlFor={htmlFor} wrap={wrap} />
      {children && (
        <span
          className={cn(
            "flex shrink-0 items-center gap-2",
            wrapping && "min-w-0 max-w-full flex-wrap",
          )}
        >
          {children}
        </span>
      )}
    </div>
  );
}

/** A block row that is itself the control opening what it names. */
export function BlockRowButton({
  title,
  detail,
  leading,
  external,
  disabled,
  onClick,
  children,
}: {
  title: string;
  detail?: ReactNode;
  leading?: ReactNode;
  /** Set when the row opens a page in the browser. */
  external?: boolean;
  disabled?: boolean;
  onClick: () => void;
  /** What stands before the chevron, such as how many things the row opens onto. */
  children?: ReactNode;
}) {
  const wrapping = useContext(WrapTextContext);
  const Trailing = external ? ExternalLink : ChevronRight;
  return (
    <button
      type="button"
      className={cn(
        rowLine,
        wrapping ? "gap-3.5 py-[11px]" : "gap-2.5 py-2",
        "w-full text-left outline-none transition-colors hover:bg-field-hover focus-visible:bg-field-hover disabled:pointer-events-none disabled:opacity-50",
      )}
      disabled={disabled}
      onClick={onClick}
    >
      {leading}
      <RowText title={title} detail={detail} />
      {children && (
        <span className="flex shrink-0 items-center gap-2">{children}</span>
      )}
      <Trailing
        className={cn(
          "shrink-0 text-muted-foreground",
          external ? "size-3.5" : "size-4",
        )}
        aria-hidden="true"
      />
    </button>
  );
}

export function BlockTile({ children }: { children: ReactNode }) {
  return (
    <span
      className="flex size-[30px] shrink-0 items-center justify-center rounded-[10px] bg-tile text-muted-foreground"
      aria-hidden="true"
    >
      {children}
    </span>
  );
}

export function BlockIconTile({ icon: Icon }: { icon: LucideIcon }) {
  return (
    <BlockTile>
      <Icon className="size-4" />
    </BlockTile>
  );
}

export function BlockHeading({ children }: { children: ReactNode }) {
  const settings = useContext(WrapTextContext);
  return (
    <h3
      className={cn(
        settings ? "mt-[19px] mb-[7px] font-normal" : "mt-4 mb-1.5",
        "px-[3px] text-[11px] text-muted-foreground first:mt-0",
      )}
    >
      {children}
    </h3>
  );
}

/** The line under a block that says what it means; it never carries an action. */
export function BlockNote({ children }: { children: ReactNode }) {
  const settings = useContext(WrapTextContext);
  return (
    <p
      className={cn(
        settings
          ? "mt-[7px] font-normal text-muted-foreground"
          : "mt-1.5 text-faint",
        "px-[3px] text-[11px] leading-[1.5]",
      )}
    >
      {children}
    </p>
  );
}

export function BlockHint({ children }: { children: ReactNode }) {
  return (
    <div className="mt-3.5 flex items-start gap-2.5 rounded-row bg-field px-[13px] py-[11px]">
      <Info
        className="mt-px size-4 shrink-0 text-muted-foreground"
        aria-hidden="true"
      />
      <p className="text-[11px] leading-[1.5] text-muted-foreground">
        {children}
      </p>
    </div>
  );
}
