import { cn } from "@/lib/utils";

export function Message({ role, className, ...props }) {
  return (
    <div
      className={cn(
        "group flex w-full flex-col",
        role === "user" ? "items-end" : "items-start",
        className
      )}
      {...props}
    />
  );
}

export function MessageBubble({ className, ...props }) {
  return <div className={cn("w-full space-y-2 text-sm leading-relaxed", className)} {...props} />;
}

export function MessageMeta({ className, ...props }) {
  return <div className={cn("text-xs text-white/30", className)} {...props} />;
}

export function MessageText({ className, ...props }) {
  return (
    <div
      className={cn(
        "whitespace-pre-wrap rounded-2xl text-white/80",
        className
      )}
      {...props}
    />
  );
}

export function MessageCode({ className, ...props }) {
  return (
    <pre
      className={cn(
        "whitespace-pre-wrap rounded-lg border border-white/10 bg-[rgba(39,41,45,0.92)] px-3 py-2 pr-10 text-[12px] text-[#b4b6b9c9]",
        className
      )}
      {...props}
    />
  );
}
