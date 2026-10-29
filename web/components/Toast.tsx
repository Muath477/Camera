"use client";

import { AnimatePresence, motion } from "motion/react";
import { createContext, useCallback, useContext, useMemo, useRef, useState, type ReactNode } from "react";
import { AlertTriangle, CheckCircle2, Info, X } from "lucide-react";

export type Toast = {
  id: number;
  title: string;
  body?: string;
  tone?: "info" | "ok" | "critical";
  href?: string;
  ttl?: number;
};

type Ctx = { push: (t: Omit<Toast, "id">) => void; dismiss: (id: number) => void };

const ToastCtx = createContext<Ctx>({ push: () => {}, dismiss: () => {} });

export const useToast = () => useContext(ToastCtx);

const icons = { info: Info, ok: CheckCircle2, critical: AlertTriangle };
const tones = {
  info: "border-[var(--border)]",
  ok: "border-brand-300 dark:border-brand-800",
  critical: "border-rose-300 dark:border-rose-900",
};

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<Toast[]>([]);
  const seq = useRef(0);

  const dismiss = useCallback((id: number) => setItems((l) => l.filter((t) => t.id !== id)), []);
  const push = useCallback(
    (t: Omit<Toast, "id">) => {
      const id = ++seq.current;
      setItems((l) => [{ ...t, id }, ...l].slice(0, 4));
      setTimeout(() => dismiss(id), t.ttl ?? 6000);
    },
    [dismiss],
  );
  const value = useMemo(() => ({ push, dismiss }), [push, dismiss]);

  return (
    <ToastCtx.Provider value={value}>
      {children}
      <div className="fixed bottom-20 md:bottom-6 left-4 z-[60] flex flex-col gap-2 w-[min(92vw,22rem)] pointer-events-none" dir="rtl">
        <AnimatePresence initial={false}>
          {items.map((t) => {
            const Icon = icons[t.tone ?? "info"];
            return (
              <motion.div
                key={t.id}
                layout
                initial={{ opacity: 0, x: -24, scale: 0.96 }}
                animate={{ opacity: 1, x: 0, scale: 1 }}
                exit={{ opacity: 0, x: -24, scale: 0.96 }}
                transition={{ type: "spring", stiffness: 420, damping: 32 }}
                className={`card pointer-events-auto p-3 flex items-start gap-3 border ${tones[t.tone ?? "info"]} shadow-[var(--shadow-lg)]`}
              >
                <Icon className={`size-5 shrink-0 ${t.tone === "critical" ? "text-bad-500" : t.tone === "ok" ? "text-brand-600" : "text-info-500"}`} />
                <div className="min-w-0 flex-1">
                  <div className="font-medium text-sm">{t.title}</div>
                  {t.body && <div className="muted text-xs mt-0.5">{t.body}</div>}
                  {t.href && (
                    <a href={t.href} className="text-xs text-brand-600 hover:underline mt-1 inline-block">
                      فتح
                    </a>
                  )}
                </div>
                <button onClick={() => dismiss(t.id)} className="muted hover:text-current" aria-label="إغلاق">
                  <X className="size-4" />
                </button>
              </motion.div>
            );
          })}
        </AnimatePresence>
      </div>
    </ToastCtx.Provider>
  );
}
