import { cn } from "cn";
import { ScrollArea as ScrollAreaPrimitive } from "radix-ui";
import type * as React from "react";

// Radix's `display: table` content wrapper breaks truncation and flex; the viewport makes it a flex column.
function ScrollArea({
  className,
  children,
  viewportRef,
  type = "hover",
  scrollHideDelay = 600,
  ...props
}: React.ComponentProps<typeof ScrollAreaPrimitive.Root> & {
  /** The element that scrolls. */
  viewportRef?: React.Ref<HTMLDivElement>;
}) {
  return (
    <ScrollAreaPrimitive.Root
      data-slot="scroll-area"
      type={type}
      scrollHideDelay={scrollHideDelay}
      className={cn("relative overflow-hidden", className)}
      {...props}
    >
      <ScrollAreaPrimitive.Viewport
        ref={viewportRef}
        data-slot="scroll-area-viewport"
        className="relative size-full rounded-[inherit] outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 [&>div]:!flex [&>div]:min-h-full [&>div]:flex-col"
      >
        {children}
      </ScrollAreaPrimitive.Viewport>
      <ScrollBar />
      <ScrollAreaPrimitive.Corner />
    </ScrollAreaPrimitive.Root>
  );
}

// Mounted while hidden, or the fade-out cannot run.
function ScrollBar({
  className,
  orientation = "vertical",
  ...props
}: React.ComponentProps<typeof ScrollAreaPrimitive.ScrollAreaScrollbar>) {
  return (
    <ScrollAreaPrimitive.ScrollAreaScrollbar
      data-slot="scroll-area-scrollbar"
      orientation={orientation}
      forceMount
      className={cn(
        "flex touch-none p-px transition-opacity duration-200 select-none data-[state=hidden]:pointer-events-none data-[state=hidden]:opacity-0 data-[state=visible]:opacity-100 motion-reduce:transition-none",
        orientation === "vertical" && "h-full w-1.5",
        orientation === "horizontal" && "h-1.5 flex-col",
        className,
      )}
      {...props}
    >
      <ScrollAreaPrimitive.ScrollAreaThumb
        data-slot="scroll-area-thumb"
        className="relative flex-1 rounded-full bg-foreground/14 transition-colors hover:bg-foreground/26 motion-reduce:transition-none"
      />
    </ScrollAreaPrimitive.ScrollAreaScrollbar>
  );
}

export { ScrollArea, ScrollBar };
