"use client";

import { AnimatePresence, motion } from "motion/react";
import { useMemo } from "react";

import { DeleteIcon } from "@/components/icons/DeleteIcon";
import { cn } from "@/lib/utils";

function formatSessionTime(value) {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return date.toLocaleString([], {
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function SessionSidebar({
  open,
  sessions,
  activeSessionID,
  onClose,
  onSelectSession,
  onDeleteSession,
  className,
}) {
  const rows = useMemo(() => {
    if (!Array.isArray(sessions)) {
      return [];
    }
    return sessions.slice(0, 10);
  }, [sessions]);

  return (
    <AnimatePresence>
      {open ? (
        <motion.div
          className={cn("fixed inset-0 z-30", className)}
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.35, ease: "easeOut" }}
        >
          <button
            type="button"
            aria-label="Close sidebar"
            className="absolute inset-0 cursor-default bg-black/35"
            onClick={onClose}
          />

          <motion.aside
            className="absolute left-0 top-0 flex h-full w-[360px] max-w-[86vw] flex-col border-r border-slate-800 bg-slate-950 px-4 py-5 text-slate-100 shadow-xl"
            initial={{ x: -420 }}
            animate={{ x: 0 }}
            exit={{ x: -420 }}
            transition={{ duration: 0.5, ease: "easeInOut" }}
          >
            <div className="mb-4 flex items-center justify-between">
              <div className="text-sm font-semibold tracking-wide text-slate-100/90">
                Sessions
              </div>
              <button
                type="button"
                className="rounded-full px-2 py-1 text-xs text-slate-300/80 transition-colors hover:bg-white/5 hover:text-slate-100"
                onClick={onClose}
              >
                Close
              </button>
            </div>

            <div className="flex-1 overflow-y-auto pr-1">
              <AnimatePresence initial={false}>
                {rows.map((s) => {
                  const title = String(s?.title || "").trim() || "New Chat";
                  const last = formatSessionTime(s?.last_activity || s?.LastActivity);
                  const id = String(s?.id || s?.ID || "");
                  const isActive = id && id === String(activeSessionID || "");

                  return (
                    <motion.div
                      key={id || title}
                      layout
                      initial={{ opacity: 0, y: 8 }}
                      animate={{ opacity: 1, y: 0 }}
                      exit={{ opacity: 0, y: -12, height: 0, marginBottom: 0 }}
                      transition={{ duration: 1.5, ease: "easeInOut" }}
                      className="mb-3"
                    >
                      <button
                        type="button"
                        onClick={() => (id ? onSelectSession?.(id) : null)}
                        className={cn(
                          "group relative w-full rounded-2xl border bg-slate-900 px-4 py-3 text-left transition-colors",
                          "border-slate-800 hover:border-slate-700 hover:bg-slate-900/95",
                          isActive ? "ring-1 ring-cyan-400/60" : ""
                        )}
                      >
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0 flex-1">
                            <AnimatePresence initial={false} mode="popLayout">
                              <motion.div
                                key={title}
                                initial={{ opacity: 0, x: -10 }}
                                animate={{ opacity: 1, x: 0 }}
                                exit={{ opacity: 0, x: 10 }}
                                transition={{ duration: 2, ease: "easeInOut" }}
                                className="truncate text-sm font-semibold text-slate-100"
                              >
                                {title}
                              </motion.div>
                            </AnimatePresence>
                            <div className="mt-1 flex items-center justify-between text-xs text-slate-400">
                              <span className="truncate">
                                {typeof s?.message_count === "number"
                                  ? `${s.message_count} msg`
                                  : ""}
                              </span>
                              <span className="shrink-0 text-right text-slate-400/90">
                                {last}
                              </span>
                            </div>
                          </div>

                          <button
                            type="button"
                            aria-label="Delete session"
                            onClick={(e) => {
                              e.preventDefault();
                              e.stopPropagation();
                              if (id) {
                                onDeleteSession?.(id);
                              }
                            }}
                            className={cn(
                              "mt-0.5 inline-flex items-center justify-center rounded-full p-2 text-slate-300/80",
                              "opacity-0 transition-opacity duration-[2000ms] ease-in-out group-hover:opacity-100 hover:bg-white/5 hover:text-slate-100"
                            )}
                          >
                            <DeleteIcon size={18} />
                          </button>
                        </div>
                      </button>
                    </motion.div>
                  );
                })}
              </AnimatePresence>

              {rows.length === 0 ? (
                <div className="mt-10 text-center text-xs text-slate-400">
                  No sessions yet.
                </div>
              ) : null}
            </div>
          </motion.aside>
        </motion.div>
      ) : null}
    </AnimatePresence>
  );
}
