import * as React from "react";

import { cn } from "@/lib/utils";

const Badge = React.forwardRef(function Badge({ className, variant = "default", ...props }, ref) {
  const variantStyles = {
    default: "bg-cyan-500/15 text-cyan-200 border border-cyan-400/25",
    outline: "bg-transparent text-slate-200 border border-slate-600",
    secondary: "bg-slate-700/60 text-slate-100 border border-slate-500/40",
    destructive: "bg-rose-600/20 text-rose-200 border border-rose-500/40",
  };

  return (
    <span
      ref={ref}
      className={cn(
        "inline-flex items-center rounded-full px-2.5 py-0.5 text-[11px] font-semibold",
        variantStyles[variant] || variantStyles.default,
        className
      )}
      {...props}
    />
  );
});

Badge.displayName = "Badge";

export { Badge };
