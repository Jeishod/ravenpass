import { cn } from "cn";
import type { ComponentProps } from "react";
import { Input } from "./ui/input.tsx";

/** AccessField is the single tall, centred field of an access screen; the caller sets its typography. */
export function AccessField({
  className,
  ...props
}: ComponentProps<typeof Input>) {
  return (
    <Input
      className={cn(
        "h-[46px] rounded-row border-0 bg-field text-center shadow-none placeholder:font-sans placeholder:text-[13px] placeholder:tracking-normal focus-visible:border-0 focus-visible:ring-0 focus-visible:inset-ring focus-visible:inset-ring-foreground/14 aria-invalid:inset-ring aria-invalid:inset-ring-destructive/60",
        className,
      )}
      {...props}
    />
  );
}
