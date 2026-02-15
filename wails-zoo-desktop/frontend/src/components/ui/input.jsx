import * as React from "react";

import { cn } from "@/lib/utils";

const Input = React.forwardRef(function Input({ className, ...props }, ref) {
  return (
    <input
      ref={ref}
      className={cn(
        "h-10 w-full rounded-md border border-slate-700 bg-slate-900/70 px-3 text-sm text-slate-100 placeholder:text-slate-500 outline-none focus-visible:ring-2 focus-visible:ring-cyan-300/70",
        className
      )}
      {...props}
    />
  );
});

Input.displayName = "Input";

export { Input };
