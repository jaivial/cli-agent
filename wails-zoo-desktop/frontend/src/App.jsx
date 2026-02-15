import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import {
  ArrowUp,
  ChevronDown,
  ChevronUp,
  File,
  Folder,
  RefreshCw,
  User,
} from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import * as runtime from "../wailsjs/runtime/runtime.js";
import { useStickToBottomContext } from "use-stick-to-bottom";

import {
  Conversation,
  ConversationContent,
} from "@/components/ai-elements/conversation";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Textarea } from "@/components/ui/textarea";
import {
  Message,
  MessageBubble,
  MessageCode,
  MessageMeta,
  MessageText,
} from "@/components/ai-elements/chat-message";
import { PanelLeftOpenIcon } from "@/components/icons/PanelLeftOpenIcon";
import { SquarePenIcon } from "@/components/icons/SquarePenIcon";
import { CopyIcon } from "@/components/icons/CopyIcon";
import { CheckIcon } from "@/components/icons/CheckIcon";
import { SessionSidebar } from "@/components/SessionSidebar";

function getBackend() {
  const app = window?.go?.main?.App;
  return app ? app : null;
}

function nowTime() {
  return new Date().toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function sanitizeText(value) {
  return String(value || "").trim();
}

function renderBoldSanitized(text) {
  const raw = String(text ?? "");
  if (!raw) {
    return null;
  }

  // Treat both **bold** and *bold* as bold (per desktop UI rules).
  const pattern = /\*\*([^*]+)\*\*|\*([^*]+)\*/g;
  const out = [];
  let lastIndex = 0;
  let match;
  let key = 0;

  while ((match = pattern.exec(raw)) !== null) {
    const start = match.index;
    const end = start + match[0].length;
    if (start > lastIndex) {
      out.push(raw.slice(lastIndex, start));
    }
    const boldText = match[1] ?? match[2] ?? "";
    out.push(
      <strong key={`b-${key++}`} className="font-semibold">
        {boldText}
      </strong>
    );
    lastIndex = end;
  }

  if (lastIndex < raw.length) {
    out.push(raw.slice(lastIndex));
  }

  return out;
}

function stripOrchestrateShardMarkers(value) {
  const text = String(value || "");
  return text
    .replace(/^\[Shard [^\]]+ Error\]\s*/gm, "")
    .replace(/^\[Shard [^\]]+\]\s*$/gm, "")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
}

function cleanKind(event) {
  return String(event?.kind || "").toLowerCase();
}

function isTool(event) {
  return cleanKind(event) === "tool";
}

function isToolOutput(event) {
  return cleanKind(event) === "tool_output";
}

function isFileEdit(event) {
  return cleanKind(event) === "file_edit";
}

function isCompanion(event) {
  return (
    cleanKind(event) === "orchestrate_companions" ||
    cleanKind(event) === "orchestrate_companions_peak"
  );
}

function companionLabelFromEvent(event) {
  return sanitizeText(
    event?.companion_label ?? event?.CompanionLabel ?? event?.companionLabel
  );
}

function parseEventText(event) {
  return sanitizeText(event?.text || event?.Text || event?.output);
}

function trimThinkingPrefix(value) {
  const text = sanitizeText(value);
  return text.replace(/^\s*\[?\s*thinking\s*\]?\s*:?\s*/i, "").trim();
}

function newTurnTemplate() {
  return {
    statusText: "ready",
    events: [],
    running: false,
    final: "",
  };
}

function newEventID() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

const THINKING_KINDS = new Set([
  "thinking",
  "chain_of_thought",
  "chain-of-thought",
  "cot",
]);

function isThinking(kind) {
  return THINKING_KINDS.has(kind);
}

function CompanionDots({ count, max }) {
  const active = Math.max(0, Number(count) || 0);
  const total = Math.max(max || 0, active);

  return (
    <span
      aria-label={`companions ${active} of ${Math.max(total, 1)}`}
      className="inline-flex items-center gap-1"
    >
      <AnimatePresence initial={false}>
        {Array.from({ length: active }).map((_, idx) => (
          <motion.span
            key={`companion-${idx}`}
            initial={{ opacity: 0, scale: 0.94 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.94 }}
            transition={{ duration: 0.5, ease: "easeInOut" }}
          >
            <User className="size-4 text-[#b4b6b9c9]" />
          </motion.span>
        ))}
      </AnimatePresence>
    </span>
  );
}

const MESSAGE_FADE = {
  initial: { opacity: 0 },
  animate: { opacity: 1 },
  transition: { duration: 2, ease: "easeOut" },
};

function CubeDotsSpinner({ active }) {
  return (
    <span
      aria-hidden="true"
      className="inline-grid grid-cols-3 grid-rows-2 gap-1 align-middle"
    >
      {Array.from({ length: 6 }).map((_, idx) => (
        <span
          key={`cube-dot-${idx}`}
          className={[
            "h-1.5 w-1.5 rounded-[2px] bg-white/30",
            active ? "animate-pulse" : "",
            active ? `[animation-delay:${idx * 120}ms]` : "",
            "motion-reduce:animate-none",
          ]
            .filter(Boolean)
            .join(" ")}
        />
      ))}
    </span>
  );
}

function isPlanningApproachText(text) {
  return sanitizeText(text).toLowerCase() === "planning approach";
}

function extractPercentLeft(usage) {
  const raw = usage?.percent_left ?? usage?.PercentLeft ?? usage?.percentLeft;
  const n = Number(raw);
  if (!Number.isFinite(n)) {
    return null;
  }
  return Math.max(0, Math.min(100, n));
}

function isCompactingStartText(text) {
  const t = sanitizeText(text).toLowerCase();
  return t.includes("compacting old session context");
}

function isCompactingDoneText(text) {
  const t = sanitizeText(text).toLowerCase();
  return t.includes("context compacted") || t.includes("compaction failed");
}

function normalizeSlashInput(raw) {
  const text = String(raw ?? "");
  const trimmedLeft = text.replace(/^[ \t]+/, "");
  if (!trimmedLeft.startsWith("/")) {
    return "";
  }
  if (trimmedLeft.includes("\n") || trimmedLeft.includes("\r")) {
    return "";
  }
  return trimmedLeft;
}

function buildSlashPopupState(rawInput) {
  const raw = normalizeSlashInput(rawInput);
  if (!raw) {
    return { key: "", items: [] };
  }

  const trimmed = raw.trim();
  if (!trimmed || !trimmed.startsWith("/")) {
    return { key: "", items: [] };
  }

  const hasSpace = /[ \t]/.test(raw);
  const endsWithSpace = raw.endsWith(" ") || raw.endsWith("\t");
  const parts = trimmed.split(/\s+/).filter(Boolean);
  if (parts.length === 0) {
    return { key: "", items: [] };
  }

  let cmdToken = parts[0];
  if (cmdToken === "/") {
    cmdToken = "";
  }

  const base = [
    { label: "/new", description: "start a new session", insertText: "/new" },
    { label: "/clear", description: "clear chat (alias: /new)", insertText: "/clear" },
    { label: "/connect", description: "setup provider + API key", insertText: "/connect" },
    { label: "/logs", description: "show recent warn/error logs", insertText: "/logs" },
    { label: "/model", description: "choose model", insertText: "/model" },
    { label: "/resume", description: "resume a previous session", insertText: "/resume" },
    { label: "/permissions", description: "show/set permissions mode", insertText: "/permissions" },
  ];

  if (parts.length === 1 && !hasSpace) {
    const prefix = cmdToken.toLowerCase();
    return {
      key: `cmd:${prefix}`,
      items: base.filter((c) => c.label.toLowerCase().startsWith(prefix)),
    };
  }

  if (
    cmdToken.toLowerCase() === "/permissions" &&
    (parts.length === 2 || (parts.length === 1 && endsWithSpace))
  ) {
    const argPrefix = (parts.length === 2 ? parts[1] : "").trim().toLowerCase();
    const opts = [
      { value: "full-access", desc: "prompt before risky actions" },
      { value: "dangerously-full-access", desc: "run directly; auto-elevate on permission errors" },
    ];
    return {
      key: `perm:${argPrefix}`,
      items: opts
        .filter((o) => !argPrefix || o.value.toLowerCase().startsWith(argPrefix))
        .map((o) => ({
          label: o.value,
          description: o.desc,
          insertText: `/permissions ${o.value}`,
        })),
    };
  }

  return { key: "", items: [] };
}

const CODE_TRUNCATE_LIMIT = 300;
const DIFF_TRUNCATE_LIMIT = 300;

function easeInOutCubic(t) {
  return t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2;
}

function animateScrollTop(el, targetTop, durationMs, rafRef) {
  if (!el) {
    return;
  }
  if (rafRef?.current) {
    cancelAnimationFrame(rafRef.current);
  }

  const startTop = el.scrollTop;
  const nextTarget = Math.max(0, Number(targetTop) || 0);
  const delta = nextTarget - startTop;
  if (Math.abs(delta) < 1) {
    return;
  }

  const startAt = performance.now();
  const duration = Math.max(0, Number(durationMs) || 0);

  const step = (now) => {
    const elapsed = now - startAt;
    const t = duration > 0 ? Math.min(1, elapsed / duration) : 1;
    el.scrollTop = startTop + delta * easeInOutCubic(t);
    if (t < 1) {
      rafRef.current = requestAnimationFrame(step);
    }
  };

  rafRef.current = requestAnimationFrame(step);
}

function AutoScrollToBottom({ trigger }) {
  const { scrollRef, contentRef } = useStickToBottomContext();
  const rafRef = useRef(0);
  const scheduleRef = useRef(0);

  const requestScroll = useCallback(() => {
    const el = scrollRef?.current;
    if (!el) {
      return;
    }

    if (scheduleRef.current) {
      cancelAnimationFrame(scheduleRef.current);
    }
    scheduleRef.current = requestAnimationFrame(() => {
      scheduleRef.current = 0;
      const target = el.scrollHeight - el.clientHeight;
      animateScrollTop(el, target, 2000, rafRef);
    });
  }, [scrollRef]);

  useLayoutEffect(() => {
    requestScroll();
    return () => {
      if (scheduleRef.current) {
        cancelAnimationFrame(scheduleRef.current);
        scheduleRef.current = 0;
      }
      if (rafRef.current) {
        cancelAnimationFrame(rafRef.current);
        rafRef.current = 0;
      }
    };
  }, [requestScroll, trigger]);

  useEffect(() => {
    const node = contentRef?.current;
    if (!node || typeof ResizeObserver === "undefined") {
      return undefined;
    }

    let pending = false;
    const observer = new ResizeObserver(() => {
      // Coalesce rapid resize bursts (Framer Motion, images, code blocks, etc.)
      // to a single scroll request per animation frame.
      if (pending) {
        return;
      }
      pending = true;
      requestAnimationFrame(() => {
        pending = false;
        requestScroll();
      });
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, [contentRef, requestScroll]);

  return null;
}

function looksLikeGitDiff(text) {
  const s = String(text ?? "");
  if (!s) {
    return false;
  }
  if (s.includes("diff --git")) {
    return true;
  }
  if (s.includes("\n--- a/") && s.includes("\n+++ b/")) {
    return true;
  }
  return false;
}

function TruncatableCodeBlock({ text, limit = CODE_TRUNCATE_LIMIT, className }) {
  const raw = String(text ?? "");
  const [expanded, setExpanded] = useState(false);
  const [copied, setCopied] = useState(false);
  const copyResetTimerRef = useRef(null);

  const cap = Math.max(0, Number(limit) || 0);
  const isTruncated = cap > 0 && raw.length > cap;

  const ellipsis = "...";
  const shown =
    !isTruncated || expanded
      ? raw
      : cap <= 1
        ? ellipsis.slice(0, cap)
        : cap <= ellipsis.length
          ? ellipsis
          : `${raw.slice(0, cap - ellipsis.length)}${ellipsis}`;

  useEffect(() => {
    return () => {
      if (copyResetTimerRef.current) {
        clearTimeout(copyResetTimerRef.current);
        copyResetTimerRef.current = null;
      }
    };
  }, []);

  const copyToClipboard = async () => {
    const payload = raw;
    if (!payload) {
      return;
    }

    let ok = false;
    try {
      await runtime.ClipboardSetText(payload);
      ok = true;
    } catch (_) {
      ok = false;
    }

    if (!ok) {
      try {
        await navigator.clipboard.writeText(payload);
        ok = true;
      } catch (_) {
        ok = false;
      }
    }

    if (!ok) {
      return;
    }

    setCopied(true);
    if (copyResetTimerRef.current) {
      clearTimeout(copyResetTimerRef.current);
    }
    copyResetTimerRef.current = setTimeout(() => {
      setCopied(false);
      copyResetTimerRef.current = null;
    }, 10000);
  };

  return (
    <div className="relative">
      <MessageCode className={[className, "pr-12"].filter(Boolean).join(" ")}>
        {shown}
      </MessageCode>

      {raw ? (
        <button
          type="button"
          aria-label={copied ? "Copied" : "Copy"}
          onClick={copyToClipboard}
          className="absolute right-2 top-2 rounded-full p-1.5 text-[#b4b6b9c9] transition-colors hover:bg-white/5 hover:text-[#b4b6b9]"
        >
          <AnimatePresence initial={false} mode="wait">
            {copied ? (
              <motion.div
                key="check"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.2, ease: "easeInOut" }}
              >
                <CheckIcon size={18} />
              </motion.div>
            ) : (
              <motion.div
                key="copy"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.2, ease: "easeInOut" }}
              >
                <CopyIcon size={18} />
              </motion.div>
            )}
          </AnimatePresence>
        </button>
      ) : null}

      {isTruncated ? (
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label={expanded ? "Collapse" : "Expand"}
          onClick={() => setExpanded((v) => !v)}
          className="absolute bottom-2 right-2 h-8 w-8 rounded-full bg-transparent p-0 text-white/40 hover:bg-white/10 hover:text-white/70"
        >
          {expanded ? <ChevronUp className="size-4" /> : <ChevronDown className="size-4" />}
        </Button>
      ) : null}
    </div>
  );
}

function CompanionBadgeRow({ label }) {
  const txt = sanitizeText(label);
  if (!txt) {
    return null;
  }
  return (
    <div className="pb-1">
      <Badge
        variant="secondary"
        className="bg-[rgba(39,41,45,0.92)] text-[rgba(180,182,185,0.95)] border border-white/10"
      >
        {txt}
      </Badge>
    </div>
  );
}

function ComposerProgressDots({ active }) {
  return (
    <div className="pointer-events-none absolute -top-5 right-4 flex items-center gap-1">
      {Array.from({ length: 4 }).map((_, idx) => (
        <motion.span
          key={`composer-dot-${idx}`}
          className="h-1.5 w-1.5 rounded-full bg-white/40"
          initial={false}
          animate={
            active
              ? { opacity: 1, y: [0, -3, 0] }
              : { opacity: 0, y: 0 }
          }
          transition={
            active
              ? {
                  duration: 0.9,
                  repeat: Infinity,
                  ease: "easeInOut",
                  delay: idx * 0.12,
                }
              : { duration: 0.2, ease: "easeOut" }
          }
        />
      ))}
    </div>
  );
}

function fileNameFromPath(path) {
  const p = sanitizeText(path);
  if (!p) {
    return "";
  }
  const cleaned = p.replace(/[\\/]+$/, "");
  const parts = cleaned.split(/[\\/]/);
  return parts[parts.length - 1] || cleaned;
}

function isKnownExtensionlessFile(name) {
  const n = sanitizeText(name).toLowerCase();
  return (
    n === "makefile" ||
    n === "dockerfile" ||
    n === "license" ||
    n === "readme" ||
    n === "go.mod" ||
    n === "go.sum"
  );
}

function deliverableIconFor(path) {
  const name = fileNameFromPath(path);
  if (!name) {
    return File;
  }
  if (sanitizeText(path).endsWith("/")) {
    return Folder;
  }
  if (!name.includes(".") && !isKnownExtensionlessFile(name)) {
    return Folder;
  }
  return File;
}

function ChatComposer({
  input,
  setInput,
  isRunning,
  onSubmit,
  onCancel,
  bubbleRef,
  onInputKeyDown,
  contextLeftPct,
}) {
  return (
    <form
      ref={bubbleRef}
      className="relative flex w-full items-end gap-2 rounded-3xl px-3 py-2"
      style={{
        background: "rgba(39, 41, 45, 0.92)",
        boxShadow: "0 8px 10px rgba(0, 0, 0, 0.18)",
      }}
      onSubmit={onSubmit}
    >
      <ComposerProgressDots active={Boolean(isRunning)} />
      <Textarea
        rows={2}
        value={input}
        onChange={(event) => setInput(event.target.value)}
        onKeyDown={(event) => {
          if (typeof onInputKeyDown === "function") {
            const handled = onInputKeyDown(event);
            if (handled) {
              return;
            }
          }
          if (event.key === "Enter" && !event.shiftKey) {
            event.preventDefault();
            event.currentTarget.form?.requestSubmit();
          }
        }}
        disabled={isRunning}
        placeholder="Message E AI..."
        className="min-h-12 flex-1 resize-none border-0 bg-transparent px-3 py-3 text-sm text-[rgba(180,182,185,0.95)] placeholder:text-[rgba(180,182,185,0.55)] focus-visible:ring-0"
      />
      <div className="relative">
        {typeof contextLeftPct === "number" ? (
          <div className="pointer-events-none absolute -top-5 right-0 whitespace-nowrap text-[11px] text-[rgba(180,182,185,0.55)]">
            {Math.round(contextLeftPct)}%
          </div>
        ) : null}

        {isRunning ? (
          <Button
            type="button"
            onClick={onCancel}
            className="h-11 w-11 rounded-full bg-[rgba(180,182,185,0.95)] text-[rgba(30,31,33,0.90)] hover:bg-[rgba(180,182,185,0.90)]"
          >
            <RefreshCw className="size-4" />
          </Button>
        ) : (
          <Button
            type="submit"
            disabled={!sanitizeText(input)}
            className="h-11 w-11 rounded-full bg-[rgba(180,182,185,0.95)] px-4 text-[rgba(30,31,33,0.90)] hover:bg-[rgba(180,182,185,0.90)] disabled:bg-[rgba(180,182,185,0.35)] disabled:text-[rgba(30,31,33,0.55)]"
          >
            <ArrowUp className="size-4" />
          </Button>
        )}
      </div>
    </form>
  );
}

function TurnCommandRow({ command }) {
  return (
    <MessageBubble>
      <MessageMeta>Ejecutando</MessageMeta>
      <TruncatableCodeBlock
        className="max-w-[72ch]"
        text={command.command || "(sin comando)"}
      />
      <TruncatableCodeBlock
        className="max-w-[72ch]"
        text={command.output || "(sin salida)"}
      />
      <MessageMeta>{command.status || "running"}</MessageMeta>
    </MessageBubble>
  );
}

function TurnDiffRow({ diff, onOpenPath }) {
  const Icon = deliverableIconFor(diff.path);
  const label = fileNameFromPath(diff.path) || diff.path || "deliverable";
  const body = diff.newContent ? diff.newContent : diff.text;
  const canOpen = typeof onOpenPath === "function" && hasText(diff.path);

  return (
    <MessageBubble>
      <Button
        type="button"
        variant="ghost"
        disabled={!canOpen}
        onClick={() => (canOpen ? onOpenPath(diff.path) : null)}
        className="h-auto w-fit justify-start gap-2 rounded-none bg-transparent px-0 py-0 text-white/80 hover:bg-transparent hover:text-white/80 disabled:opacity-60"
      >
        <Icon className="size-4 text-white/60" />
        <span className="truncate">{label}</span>
      </Button>
      {hasText(body) ? (
        <TruncatableCodeBlock text={body} limit={DIFF_TRUNCATE_LIMIT} />
      ) : null}
    </MessageBubble>
  );
}

function TurnErrorRows({ errors }) {
  return <MessageText className="rounded-none text-[#b91c1c]">{errors.join(" / ")}</MessageText>;
}

function TurnReasoningRows({ reasoning }) {
  return (
    <MessageBubble>
      {reasoning.map((row) => (
        <MessageText className="rounded-none text-white/60 text-[13px]" key={`${row.time}-${row.kind}`}>
          {row.text}
        </MessageText>
      ))}
    </MessageBubble>
  );
}

function TurnMeta({ children }) {
  if (!children) {
    return null;
  }

  return <MessageMeta>{children}</MessageMeta>;
}

function hasText(value) {
  return sanitizeText(value).length > 0;
}

export default function App() {
  const backendRef = useRef(getBackend());
  const activeTurnRef = useRef(-1);
  const composerBubbleRef = useRef(null);

  const COMPOSER_BOTTOM_MARGIN = 24;

  const [turns, setTurns] = useState([
    // Intentionally starts empty. Welcome bubble is rendered separately.
  ]);

  const [hasStarted, setHasStarted] = useState(false);
  const [composerCenterOffsetY, setComposerCenterOffsetY] = useState(0);

  const [input, setInput] = useState("");
  const [isRunning, setIsRunning] = useState(false);
  const [activeTurnIndex, setActiveTurnIndex] = useState(-1);
  const [activeCompanions, setActiveCompanions] = useState(0);
  const [maxCompanions, setMaxCompanions] = useState(20);
  const [sessions, setSessions] = useState([]);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [activeSessionID, setActiveSessionID] = useState("");
  const [contextLeftPct, setContextLeftPct] = useState(null);
  const [scrollTick, setScrollTick] = useState(0);

  const [slashKey, setSlashKey] = useState("");
  const [slashIndex, setSlashIndex] = useState(0);

  const [connectOpen, setConnectOpen] = useState(false);
  const [connectApiKey, setConnectApiKey] = useState("");
  const [connectModel, setConnectModel] = useState("");
  const [connectBaseURL, setConnectBaseURL] = useState("");

  const [modelPickerOpen, setModelPickerOpen] = useState(false);
  const [supportedModels, setSupportedModels] = useState([]);

  const [permissionsOpen, setPermissionsOpen] = useState(false);
  const [permissionsMode, setPermissionsMode] = useState("full-access");

  const [resettingSession, setResettingSession] = useState(false);

  const contextUpdateTimerRef = useRef(0);

  activeTurnRef.current = activeTurnIndex;

  useEffect(() => {
    const backend = backendRef.current;
    if (!backend || typeof backend.GetChatHistory !== "function") {
      return undefined;
    }

    let cancelled = false;
    backend
      .GetChatHistory()
      .then((history) => {
        if (cancelled) {
          return;
        }
        if (!Array.isArray(history) || history.length === 0) {
          return;
        }
        setTurns(
          history.map((m) => ({
            id: String(m?.id || newEventID()),
            role: sanitizeText(m?.role || "assistant"),
            content: sanitizeText(m?.content || ""),
            created_at: m?.created_at ? String(m.created_at) : nowTime(),
            turn: null,
          }))
        );
        setHasStarted(true);
        setScrollTick((v) => v + 1);
      })
      .catch((err) => {
        console.error("[GetChatHistory]", err);
      });

    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    const backend = backendRef.current;
    if (!backend) {
      return undefined;
    }

    let cancelled = false;

    if (typeof backend.ListSessions === "function") {
      backend
        .ListSessions(10)
        .then((list) => {
          if (cancelled) {
            return;
          }
          if (Array.isArray(list)) {
            setSessions(list);
          }
        })
        .catch(() => {});
    }

    if (typeof backend.GetSupportedModels === "function") {
      backend
        .GetSupportedModels()
        .then((models) => {
          if (cancelled) {
            return;
          }
          if (Array.isArray(models)) {
            setSupportedModels(models.map((m) => String(m || "").trim()).filter(Boolean));
          }
        })
        .catch(() => {});
    }

    if (typeof backend.GetStatus === "function") {
      backend
        .GetStatus()
        .then((st) => {
          if (cancelled || !st) {
            return;
          }
          if (hasText(st?.model)) {
            setConnectModel(String(st.model));
          }
          if (hasText(st?.base_url)) {
            setConnectBaseURL(String(st.base_url));
          }
        })
        .catch(() => {});
    }

    const sessionsUnsub = runtime.EventsOn("desktop:sessions", (payload) => {
      if (!payload) {
        return;
      }
      if (Array.isArray(payload)) {
        setSessions(payload);
      }
    });

    return () => {
      cancelled = true;
      if (sessionsUnsub) {
        sessionsUnsub();
      }
    };
  }, []);

  const renderCount = (() => {
    let n = 0;
    for (const t of turns) {
      n += 1;
      if (t?.role === "assistant") {
        const detail = t.turn || null;
        if (detail?.events?.length) {
          n += detail.events.length;
        }
        if (hasText(t.content)) {
          n += 1;
        }
      }
    }
    return n;
  })();

  useEffect(() => {
    const backend = backendRef.current;
    if (!backend || typeof backend.GetContextUsage !== "function") {
      return undefined;
    }

    if (contextUpdateTimerRef.current) {
      clearTimeout(contextUpdateTimerRef.current);
    }

    const draft = String(input || "");
    contextUpdateTimerRef.current = setTimeout(() => {
      backend
        .GetContextUsage(draft)
        .then((usage) => {
          const left = extractPercentLeft(usage);
          setContextLeftPct(left);
        })
        .catch(() => {});
    }, 220);

    return () => {
      if (contextUpdateTimerRef.current) {
        clearTimeout(contextUpdateTimerRef.current);
      }
    };
  }, [input, scrollTick]);

  const openInFileManager = async (path) => {
    const backend = backendRef.current;
    const p = sanitizeText(path);
    if (!backend || !p || typeof backend.OpenInFileManager !== "function") {
      return;
    }
    try {
      await backend.OpenInFileManager(p);
    } catch (err) {
      console.error("[OpenInFileManager]", err);
    }
  };

  useLayoutEffect(() => {
    const computeOffset = () => {
      const rect = composerBubbleRef.current?.getBoundingClientRect?.();
      const height = rect?.height ? rect.height : 0;
      // Anchor composer at bottom and translate it so the bubble itself sits centered initially.
      const next = Math.round(COMPOSER_BOTTOM_MARGIN + height / 2 - window.innerHeight / 2);
      setComposerCenterOffsetY(Number.isFinite(next) ? next : 0);
    };

    computeOffset();
    window.addEventListener("resize", computeOffset);
    return () => window.removeEventListener("resize", computeOffset);
  }, []);

  useEffect(() => {
    let statusUnsub;
    let progressUnsub;
    const backend = backendRef.current;

    if (!backend) {
      return undefined;
    }

    backend
      .GetStatus()
      .then((next) => {
        if (!next) {
          return;
        }

        if (typeof next?.max_companions === "number") {
          setMaxCompanions(next.max_companions);
        }
        if (hasText(next?.model)) {
          setConnectModel(String(next.model));
        }
        if (hasText(next?.base_url)) {
          setConnectBaseURL(String(next.base_url));
        }
      })
      .catch(() => {});

    statusUnsub = runtime.EventsOn("desktop:status", (next) => {
      if (!next) {
        return;
      }

      if (typeof next?.max_companions === "number") {
        setMaxCompanions(next.max_companions);
      }
      if (hasText(next?.model)) {
        setConnectModel(String(next.model));
      }
      if (hasText(next?.base_url)) {
        setConnectBaseURL(String(next.base_url));
      }
    });

    progressUnsub = runtime.EventsOn("desktop:progress", (payload) => {
      if (!payload) {
        return;
      }

      const event = { ...payload };
      const kind = cleanKind(event);

      const activeIndex = activeTurnRef.current;
      if (activeIndex < 0) {
        return;
      }

      if (isCompanion(event)) {
        const nextActive = Number(
          event.active_companions ?? event.ActiveCompanions ?? 0
        );
        const nextMax = Number(event.max_companions ?? event.MaxCompanions ?? maxCompanions);

        if (!Number.isNaN(nextActive) && nextActive >= 0) {
          setActiveCompanions(nextActive);
        }
        if (!Number.isNaN(nextMax) && nextMax > 0) {
          setMaxCompanions(nextMax);
        }
        setScrollTick((v) => v + 1);
        return;
      }

      // Hide orchestrate shard-level chatter from the UI (only keep companions via isCompanion()).
      if (kind.startsWith("orchestrate_")) {
        return;
      }

	      setTurns((previousTurns) => {
	        if (previousTurns.length === 0) {
	          return previousTurns;
	        }

        const idx = Math.min(Math.max(activeIndex, 0), previousTurns.length - 1);
        const turn = previousTurns[idx];

        if (!turn || turn.role !== "assistant") {
          return previousTurns;
        }

        const nextTurn = {
          ...newTurnTemplate(),
          ...(turn.turn || {}),
        };
	        if (!Array.isArray(nextTurn.events)) {
	          nextTurn.events = [];
	        }
	        let events = [...nextTurn.events];

	        const copy = [...previousTurns];

	        // "Planning approach" is an ephemeral banner shown immediately after the user submits.
	        // Fade it out as soon as the first non-thinking output arrives.
	        if (kind !== "run_state" && !isThinking(kind)) {
	          events = events.filter((ev) => !(ev?.type === "thinking" && isPlanningApproachText(ev?.text)));
	        }

	        if (kind === "run_state") {
	          const text = parseEventText(event).toLowerCase();
	          if (text) {
	            nextTurn.statusText = text;
          }

          if (["completed", "failed", "canceled", "cancelled"].includes(text)) {
            nextTurn.running = false;
          }

          nextTurn.events = events;
          copy[idx] = { ...turn, turn: nextTurn };
          return copy;
        }

        if (isTool(event)) {
          const tool = sanitizeText(event.tool || event.Tool || "exec");
          const callId = sanitizeText(event.tool_call_id || event.ToolCallID);
          const statusValue = sanitizeText(event.tool_status || event.ToolStatus || "running");
          const command = sanitizeText(event.command || event.Command);
          const path = sanitizeText(event.path || event.Path);
          const companionLabel = companionLabelFromEvent(event);

          const existingIndex = callId
            ? events.findIndex((e) => e.type === "tool" && e.callId === callId)
            : -1;

          const payload = {
            id: existingIndex >= 0 ? events[existingIndex].id : newEventID(),
            type: "tool",
            time: nowTime(),
            callId,
            tool,
            status: statusValue,
            command,
            path,
            companionLabel,
          };

          if (existingIndex >= 0) {
            events[existingIndex] = { ...events[existingIndex], ...payload };
          } else {
            events.push(payload);
          }

          nextTurn.events = events;
          nextTurn.running = true;
          copy[idx] = { ...turn, turn: nextTurn };
          return copy;
        }

        if (isToolOutput(event)) {
          const callId = sanitizeText(event.tool_call_id || event.ToolCallID);
          const rawText = parseEventText(event);
          const text = trimThinkingPrefix(rawText);

          if (!text) {
            return previousTurns;
          }

          const tool = sanitizeText(event.tool || event.Tool || "exec");
          const statusValue = sanitizeText(event.tool_status || event.ToolStatus || "");
          const companionLabel = companionLabelFromEvent(event);
          const last = events.length > 0 ? events[events.length - 1] : null;

          if (
            callId &&
            last &&
            last.type === "tool_output" &&
            sanitizeText(last.callId) === callId
          ) {
            events[events.length - 1] = {
              ...last,
              text: `${last.text || ""}${last.text ? "\n" : ""}${text}`,
              status: statusValue || last.status,
              time: nowTime(),
              companionLabel: companionLabel || last.companionLabel,
            };
          } else {
            events.push({
              id: newEventID(),
              type: "tool_output",
              time: nowTime(),
              callId,
              tool,
              status: statusValue,
              text,
              companionLabel,
            });
          }

          nextTurn.events = events;
          nextTurn.running = true;
          copy[idx] = { ...turn, turn: nextTurn };
          return copy;
        }

        if (isFileEdit(event)) {
          const path = sanitizeText(event.path || event.Path);
          const changeType = sanitizeText(event.change_type || event.ChangeType || "modify");
          events.push({
            id: newEventID(),
            type: "file_edit",
            time: nowTime(),
            path,
            changeType,
            text: sanitizeText(event.text || event.Text),
            oldContent: sanitizeText(event.old_content || event.OldContent),
            newContent: sanitizeText(event.new_content || event.NewContent),
            toolCallID: sanitizeText(event.tool_call_id || event.ToolCallID),
            companionLabel: companionLabelFromEvent(event),
          });

          nextTurn.events = events;
          nextTurn.running = true;
          copy[idx] = { ...turn, turn: nextTurn };
          return copy;
        }

	        const rawText = parseEventText(event);
	        const normalizedText = trimThinkingPrefix(rawText);
	        if (normalizedText) {
	          // Autocompaction: show a centered spinner row while it runs.
	          if (isThinking(kind) && isCompactingStartText(normalizedText)) {
	            events = events.filter((ev) => ev?.type !== "autocompacting");
	            events.push({
	              id: newEventID(),
	              type: "autocompacting",
	              time: nowTime(),
	              kind: kind || "thinking",
	              text: "Autocompacting conversation",
	            });
	            nextTurn.events = events;
	            nextTurn.running = true;
	            copy[idx] = { ...turn, turn: nextTurn };
	            return copy;
	          }
	          if (isThinking(kind) && isCompactingDoneText(normalizedText)) {
	            events = events.filter((ev) => ev?.type !== "autocompacting");
	            nextTurn.events = events;
	            nextTurn.running = true;
	            copy[idx] = { ...turn, turn: nextTurn };
	            return copy;
	          }
	          if (!isThinking(kind) && isCompactingDoneText(normalizedText)) {
	            events = events.filter((ev) => ev?.type !== "autocompacting");
	          }

	          const eventType = isThinking(kind)
	            ? "thinking"
	            : kind === "error"
	              ? "error"
	              : "reasoning";

          const companionLabel = companionLabelFromEvent(event);
          const payload = {
            id: newEventID(),
            type: eventType,
            time: nowTime(),
            kind: kind || "log",
            text: normalizedText,
            companionLabel,
          };

          // Streamy companions can emit lots of small deltas. Keep one growing
          // block per companion+type to avoid flooding the chat log.
          if (
            (eventType === "reasoning" || eventType === "thinking") &&
            companionLabel
          ) {
            let existingIndex = -1;
            for (let j = events.length - 1; j >= 0; j -= 1) {
              const candidate = events[j];
              if (
                candidate &&
                candidate.type === eventType &&
                sanitizeText(candidate.companionLabel) === companionLabel
              ) {
                existingIndex = j;
                break;
              }
            }

            if (existingIndex >= 0) {
              const prev = events[existingIndex];
              events[existingIndex] = {
                ...prev,
                kind: payload.kind || prev.kind,
                time: payload.time,
                text: `${prev.text || ""}${payload.text || ""}`,
                companionLabel,
              };
            } else {
              events.push(payload);
            }
          } else {
            events.push(payload);
          }

          if (kind === "error") {
            nextTurn.events = events;
            nextTurn.running = false;
            nextTurn.statusText = "failed";
            copy[idx] = { ...turn, turn: nextTurn };
            return copy;
          }
        }

        nextTurn.events = events;
        nextTurn.running = true;
	        copy[idx] = { ...turn, turn: nextTurn };
	        return copy;
	      });
	      setScrollTick((v) => v + 1);
	    });

    return () => {
      if (statusUnsub) {
        statusUnsub();
      }
      if (progressUnsub) {
        progressUnsub();
      }
    };
  }, [maxCompanions]);

  const slashState = buildSlashPopupState(input);
  const slashItems = Array.isArray(slashState.items) ? slashState.items : [];
  const slashOpen = slashItems.length > 0;

  useEffect(() => {
    if (slashState.key !== slashKey) {
      setSlashKey(slashState.key);
      setSlashIndex(0);
      return;
    }
    if (slashIndex >= slashItems.length) {
      setSlashIndex(0);
    }
  }, [slashState.key, slashItems.length, slashIndex, slashKey]);

  const appendLocalAssistantMessage = (content) => {
    const text = sanitizeText(content);
    if (!text) {
      return;
    }
    setTurns((prev) => [
      ...prev,
      {
        id: crypto.randomUUID(),
        role: "assistant",
        content: text,
        created_at: nowTime(),
        turn: null,
      },
    ]);
    setHasStarted(true);
    setScrollTick((v) => v + 1);
  };

  const startNewSession = async () => {
    const backend = backendRef.current;
    if (!backend || typeof backend.CreateNewSession !== "function") {
      return;
    }
    if (resettingSession) {
      return;
    }

    setResettingSession(true);
    setSidebarOpen(false);

    // Fade out diagonally (top-right -> bottom-left), pause briefly, then reset.
    setTimeout(async () => {
      try {
        const sid = await backend.CreateNewSession();
        setActiveSessionID(String(sid || ""));
      } catch (err) {
        console.error("[CreateNewSession]", err);
      }

      setTurns([]);
      setHasStarted(false);
      setInput("");
      setIsRunning(false);
      setActiveTurnIndex(-1);
      setActiveCompanions(0);
      setScrollTick((v) => v + 1);
      setResettingSession(false);
    }, 2500);
  };

  const handleSelectSession = async (sessionID) => {
    const backend = backendRef.current;
    const sid = sanitizeText(sessionID);
    if (!backend || !sid || typeof backend.SwitchSession !== "function") {
      return;
    }
    try {
      const history = await backend.SwitchSession(sid);
      if (!Array.isArray(history)) {
        return;
      }
      setTurns(
        history.map((m) => ({
          id: String(m?.id || newEventID()),
          role: sanitizeText(m?.role || "assistant"),
          content: sanitizeText(m?.content || ""),
          created_at: m?.created_at ? String(m.created_at) : nowTime(),
          turn: null,
        }))
      );
      setHasStarted(history.length > 0);
      setActiveSessionID(sid);
      setSidebarOpen(false);
      setScrollTick((v) => v + 1);
    } catch (err) {
      console.error("[SwitchSession]", err);
    }
  };

  const handleDeleteSession = async (sessionID) => {
    const backend = backendRef.current;
    const sid = sanitizeText(sessionID);
    if (!backend || !sid || typeof backend.DeleteSession !== "function") {
      return;
    }

    setSessions((prev) =>
      Array.isArray(prev)
        ? prev.filter((s) => String(s?.id || s?.ID || "") !== sid)
        : prev
    );

    try {
      await backend.DeleteSession(sid);
    } catch (err) {
      console.error("[DeleteSession]", err);
    }

    // If we deleted the active session, refresh the chat view.
    if (sanitizeText(activeSessionID) === sid) {
      backend
        .GetChatHistory?.()
        .then((history) => {
          if (Array.isArray(history)) {
            setTurns(
              history.map((m) => ({
                id: String(m?.id || newEventID()),
                role: sanitizeText(m?.role || "assistant"),
                content: sanitizeText(m?.content || ""),
                created_at: m?.created_at ? String(m.created_at) : nowTime(),
                turn: null,
              }))
            );
            setHasStarted(history.length > 0);
            setScrollTick((v) => v + 1);
          }
        })
        .catch(() => {});
    }
  };

  const runSlashCommand = async (raw) => {
    const backend = backendRef.current;
    const normalized = normalizeSlashInput(raw);
    if (!normalized) {
      return false;
    }

    const trimmed = normalized.trim();
    const fields = trimmed.split(/\s+/).filter(Boolean);
    const cmd = String(fields[0] || "").toLowerCase();

    if (cmd === "/new" || cmd === "/clear") {
      await startNewSession();
      return true;
    }

    if (cmd === "/resume") {
      setSidebarOpen(true);
      return true;
    }

    if (cmd === "/connect") {
      setConnectOpen(true);
      return true;
    }

    if (cmd === "/model") {
      setModelPickerOpen(true);
      return true;
    }

    if (cmd === "/permissions") {
      const arg = sanitizeText(fields.slice(1).join(" "));
      if (!arg) {
        setPermissionsOpen(true);
        return true;
      }
      if (!backend || typeof backend.SetPermissions !== "function") {
        appendLocalAssistantMessage("Permissions command unavailable.");
        return true;
      }
      try {
        const res = await backend.SetPermissions(arg);
        setPermissionsMode(arg);
        appendLocalAssistantMessage(`Permissions: ${sanitizeText(res) || "stored"}`);
      } catch (err) {
        appendLocalAssistantMessage(`Permissions error: ${sanitizeText(err?.message || String(err))}`);
      }
      return true;
    }

    if (cmd === "/logs") {
      const limitRaw = fields.length > 1 ? fields[1] : "40";
      let limit = Number.parseInt(String(limitRaw), 10);
      if (!Number.isFinite(limit) || limit <= 0) {
        limit = 40;
      }
      if (!backend || typeof backend.GetRecentLogs !== "function") {
        appendLocalAssistantMessage("Logs command unavailable.");
        return true;
      }
      try {
        const out = await backend.GetRecentLogs(limit);
        appendLocalAssistantMessage(out);
      } catch (err) {
        appendLocalAssistantMessage(`Logs error: ${sanitizeText(err?.message || String(err))}`);
      }
      return true;
    }

    appendLocalAssistantMessage(`Unknown command: ${trimmed}`);
    return true;
  };

  const handleComposerKeyDown = (event) => {
    if (!slashOpen) {
      return false;
    }
    if (event.key === "ArrowUp") {
      event.preventDefault();
      setSlashIndex((idx) => {
        const next = idx - 1;
        return next < 0 ? slashItems.length - 1 : next;
      });
      return true;
    }
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setSlashIndex((idx) => (idx + 1) % slashItems.length);
      return true;
    }
    if (event.key === "Tab") {
      event.preventDefault();
      const selected = slashItems[slashIndex] || null;
      const insert = sanitizeText(selected?.insertText);
      if (insert) {
        setInput(insert);
      }
      return true;
    }
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      const selected = slashItems[slashIndex] || null;
      const insert = sanitizeText(selected?.insertText);
      if (insert) {
        runSlashCommand(insert);
        setInput("");
      }
      return true;
    }
    if (event.key === "Escape") {
      event.preventDefault();
      setInput("");
      return true;
    }
    return false;
  };

  const saveConnectSettings = async () => {
    const backend = backendRef.current;
    if (!backend) {
      return;
    }
    try {
      if (typeof backend.SetApiKey === "function") {
        await backend.SetApiKey(connectApiKey);
      }
      if (typeof backend.SetModel === "function" && hasText(connectModel)) {
        await backend.SetModel(connectModel);
      }
      if (typeof backend.SetBaseURL === "function" && hasText(connectBaseURL)) {
        await backend.SetBaseURL(connectBaseURL);
      }
      setConnectOpen(false);
      appendLocalAssistantMessage("Connection settings stored.");
    } catch (err) {
      appendLocalAssistantMessage(`Connect error: ${sanitizeText(err?.message || String(err))}`);
    }
  };

  const applyModelChoice = async (model) => {
    const backend = backendRef.current;
    const next = sanitizeText(model);
    if (!backend || !next || typeof backend.SetModel !== "function") {
      return;
    }
    try {
      await backend.SetModel(next);
      setConnectModel(next);
      setModelPickerOpen(false);
      appendLocalAssistantMessage(`Model set: ${next}`);
      setScrollTick((v) => v + 1);
    } catch (err) {
      appendLocalAssistantMessage(`Model error: ${sanitizeText(err?.message || String(err))}`);
    }
  };

  const applyPermissionsChoice = async (mode) => {
    const backend = backendRef.current;
    const next = sanitizeText(mode);
    if (!backend || !next || typeof backend.SetPermissions !== "function") {
      return;
    }
    try {
      const res = await backend.SetPermissions(next);
      setPermissionsMode(next);
      setPermissionsOpen(false);
      appendLocalAssistantMessage(`Permissions: ${sanitizeText(res) || "stored"}`);
    } catch (err) {
      appendLocalAssistantMessage(`Permissions error: ${sanitizeText(err?.message || String(err))}`);
    }
  };

  async function sendPrompt(event) {
    event.preventDefault();
    const prompt = sanitizeText(input);
    if (!prompt || isRunning || !backendRef.current) {
      return;
    }

    // Slash commands (TUI parity).
    if (normalizeSlashInput(prompt)) {
      setInput("");
      await runSlashCommand(prompt);
      return;
    }

    if (!hasStarted) {
      setHasStarted(true);
    }

    const updatedTurns = [...turns];
    updatedTurns.push({
      id: crypto.randomUUID(),
      role: "user",
      content: prompt,
      created_at: nowTime(),
      turn: null,
    });

    const assistantIndex = updatedTurns.length;
    updatedTurns.push({
      id: crypto.randomUUID(),
      role: "assistant",
      content: "",
      created_at: nowTime(),
      turn: {
        ...newTurnTemplate(),
        running: true,
        statusText: "starting",
        events: [
          {
            id: newEventID(),
            type: "thinking",
            time: nowTime(),
            kind: "thinking",
            text: "Planning approach",
          },
        ],
      },
    });

    setTurns(updatedTurns);
    setActiveTurnIndex(assistantIndex);
    setInput("");
    setIsRunning(true);
    setActiveCompanions(0);
    setScrollTick((v) => v + 1);

    try {
      const output = await backendRef.current.SendPrompt(prompt);
      const cleanedOutput = stripOrchestrateShardMarkers(output);

      setTurns((previous) => {
        const idx = previous.length > assistantIndex ? assistantIndex : previous.length - 1;
        if (idx < 0) {
          return previous;
        }

        const selected = previous[idx];
        if (!selected || selected.role !== "assistant") {
          return previous;
        }

        const nextTurn = {
          ...(selected.turn || newTurnTemplate()),
          running: false,
          final: sanitizeText(cleanedOutput),
          statusText: "completed",
          events: Array.isArray(selected?.turn?.events)
            ? selected.turn.events.filter(
                (ev) =>
                  !(ev?.type === "thinking" && isPlanningApproachText(ev?.text)) &&
                  ev?.type !== "autocompacting"
              )
            : [],
        };

        return [
          ...previous.slice(0, idx),
          {
            ...selected,
            content: sanitizeText(cleanedOutput),
            turn: nextTurn,
          },
          ...previous.slice(idx + 1),
        ];
      });

      setIsRunning(false);
      setActiveTurnIndex(-1);
      setScrollTick((v) => v + 1);
    } catch (err) {
      const message =
        sanitizeText(err?.message || String(err)) || "Error al ejecutar la solicitud";

      console.error("[sendPrompt]", message);
      setTurns((previous) => {
        const idx = previous.length > assistantIndex ? assistantIndex : previous.length - 1;
        if (idx < 0) {
          return previous;
        }

        const selected = previous[idx];
        if (!selected || selected.role !== "assistant") {
          return previous;
        }

        return [
          ...previous.slice(0, idx),
          {
            ...selected,
            content: "",
            turn: (() => {
              const current = { ...newTurnTemplate(), ...(selected.turn || {}) };
              const evs = Array.isArray(current.events)
                ? current.events.filter(
                    (ev) =>
                      !(ev?.type === "thinking" && isPlanningApproachText(ev?.text)) &&
                      ev?.type !== "autocompacting"
                  )
                : [];
              evs.push({
                id: newEventID(),
                type: "error",
                time: nowTime(),
                kind: "error",
                text: message,
              });
              return {
                ...current,
                running: false,
                statusText: "failed",
                events: evs,
              };
            })(),
          },
          ...previous.slice(idx + 1),
        ];
      });

      setIsRunning(false);
      setActiveTurnIndex(-1);
      setScrollTick((v) => v + 1);
    }
  }

  function cancelRun() {
    if (!backendRef.current || !isRunning) {
      return;
    }

    backendRef.current.CancelCurrentRun().catch(() => {});
    setIsRunning(false);
    setActiveTurnIndex(-1);
  }

  return (
    <div
      className="relative min-h-screen w-full"
      style={{ background: "rgba(30, 31, 33, 0.95)" }}
    >
      <div className="fixed left-4 top-4 z-20 text-white/80">
        <button
          type="button"
          aria-label="Open sessions"
          onClick={() => setSidebarOpen((v) => !v)}
          className="rounded-full bg-slate-950/0 p-2 text-white/80 transition-colors hover:bg-white/5 hover:text-white"
        >
          <PanelLeftOpenIcon size={22} />
        </button>
      </div>

      <div className="fixed right-4 top-4 z-20 text-white/80">
        <button
          type="button"
          aria-label="New chat"
          onClick={() => startNewSession()}
          className="rounded-full bg-slate-950/0 p-2 text-white/80 transition-colors hover:bg-white/5 hover:text-white"
        >
          <SquarePenIcon size={22} />
        </button>
      </div>

      <SessionSidebar
        open={sidebarOpen}
        sessions={sessions}
        activeSessionID={activeSessionID}
        onClose={() => setSidebarOpen(false)}
        onSelectSession={handleSelectSession}
        onDeleteSession={handleDeleteSession}
      />

      <motion.div
        className="min-h-screen"
        initial={false}
        animate={
          resettingSession
            ? { opacity: 0 }
            : { opacity: 1 }
        }
        transition={
          resettingSession
            ? { duration: 1.5, ease: "easeInOut" }
            : { duration: 0.6, ease: "easeOut" }
        }
      >
        <Conversation className="min-h-screen bg-transparent text-white/80">
        <ConversationContent className="mx-auto w-full max-w-3xl gap-6 px-4 py-6 pb-40">
          {turns.map((turn) => {
            const detail = turn.role === "assistant" ? turn.turn || newTurnTemplate() : null;
            const turnType = turn.role === "user" ? "user" : "assistant";

            return (
              <div key={turn.id} className="w-full space-y-3">
                {turnType === "user" ? (
                  <Message role="user">
                    <motion.div {...MESSAGE_FADE} className="w-full">
                      <MessageBubble>
                        <MessageText className="ml-auto max-w-[92%] rounded-none px-0 py-0 text-right">
                          {renderBoldSanitized(turn.content)}
                        </MessageText>
                      </MessageBubble>
                    </motion.div>
                  </Message>
                ) : null}

                {turnType === "assistant" && detail ? (
	                  <>
	                    <AnimatePresence initial={false}>
	                      {(detail.events || []).map((ev) => {
	                        if (!ev || !ev.type) {
	                          return null;
	                        }

	                        const key = `${turn.id}-ev-${ev.id || `${ev.type}-${ev.time || "t"}`}`;
	                        const wrapperExit = {
	                          opacity: 0,
	                          height: 0,
	                          marginTop: 0,
	                          marginBottom: 0,
	                        };
	                        const wrapperTransition = { duration: 0.8, ease: "easeOut" };

	                        switch (ev.type) {
                          case "autocompacting":
                            return (
                              <motion.div
                                key={key}
                                className="w-full"
                                initial={false}
                                exit={wrapperExit}
                                transition={wrapperTransition}
                                style={{ overflow: "hidden" }}
                              >
                                <Message role="assistant">
                                  <motion.div {...MESSAGE_FADE} className="w-full">
                                    <div className="w-full text-center">
                                      <span className="inline-flex items-center gap-2 text-white/80 opacity-80">
                                        <CubeDotsSpinner active={Boolean(detail.running)} />
                                        <span className="text-xs">{ev.text}</span>
                                      </span>
                                    </div>
                                  </motion.div>
                                </Message>
                              </motion.div>
                            );

	                          case "thinking":
	                            return (
	                              <motion.div
	                                key={key}
	                                className="w-full"
	                                initial={false}
	                                exit={wrapperExit}
	                                transition={wrapperTransition}
	                                style={{ overflow: "hidden" }}
	                              >
	                                <Message role="assistant">
		                                  <MessageBubble>
		                                    <motion.div
		                                      className="w-full"
		                                      initial={{ opacity: 0, x: -16 }}
		                                      animate={{ opacity: 1, x: 0 }}
		                                      transition={{ duration: 2, ease: "easeOut" }}
		                                    >
		                                      <CompanionBadgeRow label={ev.companionLabel} />
		                                      <MessageText
		                                        className="max-w-[92%] rounded-none text-left text-[13px] text-[#ffffff59]"
		                                        style={{ fontStyle: "oblique" }}
		                                      >
		                                        {isPlanningApproachText(ev.text) ? (
		                                          <span className="inline-flex items-center gap-2">
		                                            <CubeDotsSpinner active={Boolean(detail.running)} />
		                                            {renderBoldSanitized(ev.text)}
		                                          </span>
		                                        ) : (
		                                          renderBoldSanitized(ev.text)
		                                        )}
		                                      </MessageText>
		                                    </motion.div>
		                                  </MessageBubble>
		                                </Message>
		                              </motion.div>
		                            );

	                          case "tool":
	                            return (
	                              <motion.div
	                                key={key}
	                                className="w-full"
	                                initial={false}
	                                exit={wrapperExit}
	                                transition={wrapperTransition}
	                                style={{ overflow: "hidden" }}
	                              >
		                                <Message role="assistant">
		                                  <motion.div {...MESSAGE_FADE} className="w-full">
		                                    <MessageBubble>
		                                      <CompanionBadgeRow label={ev.companionLabel} />
		                                      {hasText(ev.command) ? (
		                                        <TruncatableCodeBlock
		                                          className="max-w-[72ch]"
		                                          text={ev.command}
		                                        />
		                                      ) : null}
		                                      {hasText(ev.status) ? (
		                                        <MessageMeta>{ev.status}</MessageMeta>
		                                      ) : null}
		                                    </MessageBubble>
		                                  </motion.div>
		                                </Message>
	                              </motion.div>
	                            );

	                          case "tool_output":
	                            return (
	                              <motion.div
	                                key={key}
	                                className="w-full"
	                                initial={false}
	                                exit={wrapperExit}
	                                transition={wrapperTransition}
	                                style={{ overflow: "hidden" }}
	                              >
		                                <Message role="assistant">
		                                  <motion.div {...MESSAGE_FADE} className="w-full">
		                                    <MessageBubble>
		                                      <CompanionBadgeRow label={ev.companionLabel} />
		                                      <TruncatableCodeBlock
		                                        className="max-w-[72ch]"
		                                        text={ev.text}
		                                      />
		                                    </MessageBubble>
		                                  </motion.div>
		                                </Message>
	                              </motion.div>
	                            );

	                          case "file_edit":
	                            return (
	                              <motion.div
	                                key={key}
	                                className="w-full"
	                                initial={false}
	                                exit={wrapperExit}
	                                transition={wrapperTransition}
	                                style={{ overflow: "hidden" }}
	                              >
	                                <Message role="assistant">
	                                  <motion.div {...MESSAGE_FADE} className="w-full">
	                                    <TurnDiffRow
	                                      diff={ev}
	                                      onOpenPath={openInFileManager}
	                                    />
	                                  </motion.div>
	                                </Message>
	                              </motion.div>
	                            );

	                          case "error":
	                            return (
	                              <motion.div
	                                key={key}
	                                className="w-full"
	                                initial={false}
	                                exit={wrapperExit}
	                                transition={wrapperTransition}
	                                style={{ overflow: "hidden" }}
	                              >
		                                <Message role="assistant">
		                                  <motion.div {...MESSAGE_FADE} className="w-full">
		                                    <MessageBubble>
		                                      <CompanionBadgeRow label={ev.companionLabel} />
		                                      <MessageText className="rounded-none text-[#b91c1c]">
		                                        {renderBoldSanitized(ev.text)}
		                                      </MessageText>
		                                    </MessageBubble>
		                                  </motion.div>
		                                </Message>
	                              </motion.div>
	                            );

	                          case "reasoning":
	                          default:
	                            return (
	                              <motion.div
	                                key={key}
	                                className="w-full"
	                                initial={false}
	                                exit={wrapperExit}
	                                transition={wrapperTransition}
	                                style={{ overflow: "hidden" }}
	                              >
		                                <Message role="assistant">
		                                  <motion.div {...MESSAGE_FADE} className="w-full">
		                                    <MessageBubble>
		                                      <CompanionBadgeRow label={ev.companionLabel} />
		                                      <MessageText
		                                        className="max-w-[92%] rounded-none text-[13px] text-[#ffffff59]"
		                                        style={{ fontStyle: "oblique" }}
		                                      >
		                                        {renderBoldSanitized(ev.text)}
		                                      </MessageText>
		                                    </MessageBubble>
		                                  </motion.div>
		                                </Message>
	                              </motion.div>
	                            );
	                        }
	                      })}
	                    </AnimatePresence>

                    {hasText(turn.content) ? (
                      <Message role="assistant">
                        <motion.div {...MESSAGE_FADE} className="w-full">
                          <MessageBubble>
                            <MessageText className="max-w-[92%] rounded-none">
                              {renderBoldSanitized(turn.content)}
                            </MessageText>
                          </MessageBubble>
                        </motion.div>
                      </Message>
	                    ) : null}

                    {turn === turns[turns.length - 1] ? (
                      <AnimatePresence initial={false}>
                        {activeCompanions > 0 ? (
                          <motion.div
                            key="companions-meta"
                            className="w-full"
                            initial={{ opacity: 0 }}
                            animate={{ opacity: 1 }}
                            exit={{ opacity: 0 }}
                            transition={{ duration: 0.5, ease: "easeInOut" }}
                          >
                            <Message role="assistant">
                              <TurnMeta>
                                <CompanionDots count={activeCompanions} max={maxCompanions} />
                              </TurnMeta>
                            </Message>
                          </motion.div>
                        ) : null}
                      </AnimatePresence>
                    ) : null}
                  </>
                ) : null}
              </div>
            );
          })}
        </ConversationContent>
        <AutoScrollToBottom trigger={scrollTick} />
      </Conversation>
      </motion.div>

      <motion.div
        className="fixed inset-x-0 z-10 px-4"
        style={{
          bottom: COMPOSER_BOTTOM_MARGIN,
          pointerEvents: resettingSession ? "none" : "auto",
        }}
        initial={{ opacity: 0, y: composerCenterOffsetY }}
        animate={{
          opacity: resettingSession ? 0 : 1,
          y: hasStarted ? 0 : composerCenterOffsetY,
        }}
        transition={
          resettingSession
            ? { duration: 1.5, ease: "easeInOut" }
            : {
                y: { duration: hasStarted ? 3 : 1.2, ease: "easeInOut" },
                opacity: { duration: 1.2, ease: "easeOut" },
              }
        }
      >
        <div className="mx-auto w-full max-w-3xl">
          <motion.div
            className="mb-4 text-center text-4xl font-semibold tracking-tight"
            style={{
              color: "rgba(180, 182, 185, 0.95)",
              fontFamily:
                'ui-sans-serif, -apple-system, BlinkMacSystemFont, "SF Pro Display", "SF Pro Text", system-ui, "Helvetica Neue", Arial, sans-serif',
            }}
            initial={{ opacity: 0, y: 6 }}
            animate={{ opacity: hasStarted ? 0 : 1, y: 0 }}
            transition={{ duration: 1.2, ease: "easeOut" }}
          >
            E AI
          </motion.div>

          {slashOpen ? (
            <div className="mb-3 rounded-2xl border border-slate-800 bg-slate-950 px-4 py-3">
              <div className="flex items-baseline justify-between gap-3">
                <div className="text-xs font-semibold text-white/60">commands</div>
                <div className="text-[11px] text-white/35">
                  ↑/↓ select • tab complete • enter run
                </div>
              </div>
              <div className="mt-2 space-y-1">
                {slashItems.map((item, idx) => {
                  const active = idx === slashIndex;
                  return (
                    <button
                      key={`${slashState.key}-${item.label}`}
                      type="button"
                      onMouseEnter={() => setSlashIndex(idx)}
                      onClick={() => {
                        runSlashCommand(item.insertText);
                        setInput("");
                      }}
                      className={[
                        "w-full rounded-xl px-3 py-2 text-left transition-colors",
                        active ? "bg-white/5" : "bg-transparent hover:bg-white/5",
                      ].join(" ")}
                    >
                      <div className="flex items-baseline justify-between gap-3">
                        <span className={active ? "text-white/90 font-semibold" : "text-white/70"}>
                          {item.label}
                        </span>
                        <span className="text-[11px] text-white/35">{item.description}</span>
                      </div>
                    </button>
                  );
                })}
              </div>
            </div>
          ) : null}

          <ChatComposer
            bubbleRef={composerBubbleRef}
            input={input}
            setInput={setInput}
            isRunning={isRunning}
            onSubmit={sendPrompt}
            onCancel={cancelRun}
            onInputKeyDown={handleComposerKeyDown}
            contextLeftPct={contextLeftPct}
          />
        </div>
      </motion.div>

      <AnimatePresence>
        {connectOpen ? (
          <motion.div
            className="fixed inset-0 z-40 flex items-center justify-center p-4"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.3, ease: "easeOut" }}
          >
            <button
              type="button"
              aria-label="Close connect modal"
              className="absolute inset-0 bg-black/50"
              onClick={() => setConnectOpen(false)}
            />
            <motion.div
              className="relative w-full max-w-lg"
              initial={{ opacity: 0, scale: 0.98, y: 6 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.98, y: 6 }}
              transition={{ duration: 0.4, ease: "easeInOut" }}
            >
              <Card className="border-slate-800 bg-slate-950 p-0">
                <CardHeader className="px-5 pt-5">
                  <CardTitle>Connect</CardTitle>
                  <CardDescription>Provider + API key settings.</CardDescription>
                </CardHeader>
                <CardContent className="px-5 pb-5">
                  <div className="space-y-3">
                    <div className="space-y-1">
                      <div className="text-xs text-slate-300/80">API key</div>
                      <Input
                        type="password"
                        value={connectApiKey}
                        onChange={(e) => setConnectApiKey(e.target.value)}
                        placeholder="EAI_API_KEY / MINIMAX_API_KEY"
                        className="bg-slate-900"
                      />
                    </div>
                    <div className="space-y-1">
                      <div className="text-xs text-slate-300/80">Model</div>
                      <Input
                        value={connectModel}
                        onChange={(e) => setConnectModel(e.target.value)}
                        placeholder="glm-4.7 / glm-5 / codex-MiniMax-M2.5"
                        className="bg-slate-900"
                      />
                    </div>
                    <div className="space-y-1">
                      <div className="text-xs text-slate-300/80">Base URL</div>
                      <Input
                        value={connectBaseURL}
                        onChange={(e) => setConnectBaseURL(e.target.value)}
                        placeholder="https://..."
                        className="bg-slate-900"
                      />
                    </div>
                  </div>

                  <div className="mt-5 flex items-center justify-end gap-2">
                    <Button variant="ghost" type="button" onClick={() => setConnectOpen(false)}>
                      Cancel
                    </Button>
                    <Button type="button" onClick={saveConnectSettings}>
                      Save
                    </Button>
                  </div>
                </CardContent>
              </Card>
            </motion.div>
          </motion.div>
        ) : null}
      </AnimatePresence>

      <AnimatePresence>
        {modelPickerOpen ? (
          <motion.div
            className="fixed inset-0 z-40 flex items-center justify-center p-4"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.3, ease: "easeOut" }}
          >
            <button
              type="button"
              aria-label="Close model picker"
              className="absolute inset-0 bg-black/50"
              onClick={() => setModelPickerOpen(false)}
            />
            <motion.div
              className="relative w-full max-w-md"
              initial={{ opacity: 0, scale: 0.98, y: 6 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.98, y: 6 }}
              transition={{ duration: 0.4, ease: "easeInOut" }}
            >
              <Card className="border-slate-800 bg-slate-950 p-0">
                <CardHeader className="px-5 pt-5">
                  <CardTitle>Model</CardTitle>
                  <CardDescription>Choose the model for this desktop session.</CardDescription>
                </CardHeader>
                <CardContent className="px-5 pb-5">
                  <div className="space-y-2">
                    {(supportedModels.length ? supportedModels : ["glm-4.7", "glm-5", "codex-MiniMax-M2.5"]).map(
                      (m) => {
                        const active = sanitizeText(connectModel).toLowerCase() === sanitizeText(m).toLowerCase();
                        return (
                          <button
                            key={`model-${m}`}
                            type="button"
                            onClick={() => applyModelChoice(m)}
                            className={[
                              "w-full rounded-2xl border px-4 py-3 text-left transition-colors",
                              active
                                ? "border-cyan-400/60 bg-white/5 text-slate-100"
                                : "border-slate-800 bg-slate-900 text-slate-200 hover:bg-slate-900/95",
                            ].join(" ")}
                          >
                            <div className="text-sm font-semibold">{m}</div>
                          </button>
                        );
                      }
                    )}
                  </div>

                  <div className="mt-5 flex items-center justify-end">
                    <Button variant="ghost" type="button" onClick={() => setModelPickerOpen(false)}>
                      Close
                    </Button>
                  </div>
                </CardContent>
              </Card>
            </motion.div>
          </motion.div>
        ) : null}
      </AnimatePresence>

      <AnimatePresence>
        {permissionsOpen ? (
          <motion.div
            className="fixed inset-0 z-40 flex items-center justify-center p-4"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.3, ease: "easeOut" }}
          >
            <button
              type="button"
              aria-label="Close permissions picker"
              className="absolute inset-0 bg-black/50"
              onClick={() => setPermissionsOpen(false)}
            />
            <motion.div
              className="relative w-full max-w-md"
              initial={{ opacity: 0, scale: 0.98, y: 6 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.98, y: 6 }}
              transition={{ duration: 0.4, ease: "easeInOut" }}
            >
              <Card className="border-slate-800 bg-slate-950 p-0">
                <CardHeader className="px-5 pt-5">
                  <CardTitle>Permissions</CardTitle>
                  <CardDescription>Set the permission mode for tool actions.</CardDescription>
                </CardHeader>
                <CardContent className="px-5 pb-5">
                  <div className="space-y-2">
                    {[
                      { value: "full-access", desc: "prompt before risky actions" },
                      { value: "dangerously-full-access", desc: "run directly; auto-elevate when possible" },
                    ].map((opt) => {
                      const active = sanitizeText(permissionsMode) === opt.value;
                      return (
                        <button
                          key={`perm-${opt.value}`}
                          type="button"
                          onClick={() => applyPermissionsChoice(opt.value)}
                          className={[
                            "w-full rounded-2xl border px-4 py-3 text-left transition-colors",
                            active
                              ? "border-cyan-400/60 bg-white/5 text-slate-100"
                              : "border-slate-800 bg-slate-900 text-slate-200 hover:bg-slate-900/95",
                          ].join(" ")}
                        >
                          <div className="text-sm font-semibold">{opt.value}</div>
                          <div className="mt-1 text-xs text-slate-400">{opt.desc}</div>
                        </button>
                      );
                    })}
                  </div>

                  <div className="mt-5 flex items-center justify-end">
                    <Button variant="ghost" type="button" onClick={() => setPermissionsOpen(false)}>
                      Close
                    </Button>
                  </div>
                </CardContent>
              </Card>
            </motion.div>
          </motion.div>
        ) : null}
      </AnimatePresence>
    </div>
  );
}
