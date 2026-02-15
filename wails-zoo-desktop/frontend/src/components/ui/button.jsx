import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva } from "class-variance-authority";

import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center rounded-full text-sm font-medium transition-all outline-none disabled:pointer-events-none disabled:opacity-50 disabled:cursor-not-allowed focus-visible:ring-2 focus-visible:ring-cyan-300/80 focus-visible:ring-offset-2 focus-visible:ring-offset-slate-950",
  {
    variants: {
      variant: {
        default:
          "bg-cyan-500 text-slate-950 hover:bg-cyan-400 active:bg-cyan-600",
        secondary:
          "bg-slate-800 text-slate-100 border border-slate-700 hover:bg-slate-700",
        ghost:
          "bg-transparent hover:bg-slate-800/50 text-slate-100",
        outline:
          "border border-slate-600 bg-transparent text-slate-100 hover:bg-slate-800/70",
        icon: "h-9 w-9 p-0 rounded-full",
      },
      size: {
        default: "h-10 px-4 py-2 gap-2",
        sm: "h-8 px-3 text-xs",
        lg: "h-11 px-8 text-base",
        icon: "h-9 w-9",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
);

const Button = React.forwardRef(function Button(
  {
    className,
    variant = "default",
    size = "default",
    asChild = false,
    ...props
  },
  ref
) {
  const Comp = asChild ? Slot : "button";
  return (
    <Comp
      className={cn(buttonVariants({ variant, size, className }))}
      ref={ref}
      {...props}
    />
  );
});

Button.displayName = "Button";

export { Button, buttonVariants };
