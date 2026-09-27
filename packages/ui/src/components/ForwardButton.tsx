import { cn } from "cn";
import type { ComponentProps } from "react";
import { ForwardArrow } from "../motion/ForwardArrow.tsx";
import { Button } from "./ui/button.tsx";

/** ForwardButton is the action that moves a walk on: the emphasised fill with an arrow. */
export function ForwardButton({
  className,
  children,
  ...props
}: Omit<ComponentProps<typeof Button>, "variant" | "size" | "asChild">) {
  return (
    <Button
      variant="raised"
      size="pill"
      className={cn("group gap-2 pr-3.5 pl-[18px]", className)}
      {...props}
    >
      {children}
      <ForwardArrow />
    </Button>
  );
}
