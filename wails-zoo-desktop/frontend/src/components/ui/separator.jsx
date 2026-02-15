import * as React from "react";

import { cn } from "@/lib/utils";

const Separator = React.forwardRef(function Separator(
  { className, orientation = "horizontal", ...props },
  ref
) {
  return (
    <div
      ref={ref}
      role="separator"
      aria-orientation={orientation}
      className={cn(
        "shrink-0 bg-slate-700/70",
        orientation === "horizontal" ? "h-px w-full" : "h-full w-px",
        className
      )}
      {...props}
    />
  );
});

Separator.displayName = "Separator";

export { Separator };
