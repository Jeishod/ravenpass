import { cn } from "cn";
import { ArrowRight } from "lucide-react";
import type { ComponentProps } from "react";

const arrow =
  "absolute inset-0 size-full transition-transform duration-350 ease-[cubic-bezier(0.22,1,0.36,1)] motion-reduce:transition-none";

/** ForwardArrow points onward from inside a `group` button and cycles while it is hovered. */
export function ForwardArrow({ className, ...props }: ComponentProps<"span">) {
  return (
    <span
      aria-hidden="true"
      className={cn("relative inline-flex size-4 overflow-hidden", className)}
      {...props}
    >
      <ArrowRight className={cn(arrow, "group-hover:translate-x-[120%]")} />
      <ArrowRight
        className={cn(arrow, "-translate-x-[120%] group-hover:translate-x-0")}
      />
    </span>
  );
}
