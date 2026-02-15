import * as React from "react";

import { cn } from "@/lib/utils";

const Card = React.forwardRef(function Card({ className, ...props }, ref) {
  return (
    <div
      ref={ref}
      className={cn(
        "rounded-2xl border border-slate-700/80 bg-slate-900/80 p-4 shadow-sm",
        className
      )}
      {...props}
    />
  );
});

Card.displayName = "Card";

const CardHeader = React.forwardRef(function CardHeader({ className, ...props }, ref) {
  return <div ref={ref} className={cn("mb-3 space-y-1", className)} {...props} />;
});

CardHeader.displayName = "CardHeader";

const CardTitle = React.forwardRef(function CardTitle({ className, ...props }, ref) {
  return (
    <h3
      ref={ref}
      className={cn("text-sm font-semibold text-slate-100", className)}
      {...props}
    />
  );
});

CardTitle.displayName = "CardTitle";

const CardDescription = React.forwardRef(function CardDescription(
  { className, ...props },
  ref
) {
  return (
    <p
      ref={ref}
      className={cn("text-xs text-slate-400", className)}
      {...props}
    />
  );
});

CardDescription.displayName = "CardDescription";

const CardContent = React.forwardRef(function CardContent({ className, ...props }, ref) {
  return <div ref={ref} className={cn("space-y-3", className)} {...props} />;
});

CardContent.displayName = "CardContent";

export { Card, CardHeader, CardTitle, CardDescription, CardContent };
