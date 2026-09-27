import { cn } from "cn";
import { Switch as SwitchPrimitive } from "radix-ui";
import type * as React from "react";

function Switch({
  className,
  size = "default",
  ...props
}: React.ComponentProps<typeof SwitchPrimitive.Root> & {
  size?: "sm" | "default";
}) {
  return (
    <SwitchPrimitive.Root
      data-slot="switch"
      data-size={size}
      className={cn(
        "peer group/switch inline-flex shrink-0 items-center rounded-full border border-transparent transition-all outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-35 data-[size=default]:h-5 data-[size=default]:w-[34px] data-[size=sm]:h-3.5 data-[size=sm]:w-6 data-[state=checked]:bg-primary data-[state=unchecked]:bg-quiet",
        className,
      )}
      {...props}
    >
      <SwitchPrimitive.Thumb
        data-slot="switch-thumb"
        className={cn(
          "pointer-events-none block rounded-full ring-0 transition-transform group-data-[size=default]/switch:size-4 group-data-[size=sm]/switch:size-3 data-[state=checked]:translate-x-full data-[state=checked]:bg-primary-foreground data-[state=unchecked]:translate-x-0 data-[state=unchecked]:bg-foreground group-disabled/switch:data-[state=unchecked]:bg-faint",
        )}
      />
    </SwitchPrimitive.Root>
  );
}

export { Switch };
