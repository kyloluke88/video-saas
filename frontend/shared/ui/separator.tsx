import * as React from "react";

import { cn } from "@/shared/lib/cn";

export const Separator = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn("h-px w-full bg-border/70", className)} role="separator" {...props} />
  ),
);

Separator.displayName = "Separator";
