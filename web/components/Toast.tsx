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
const chips = {
  info: "bg-aqua-400",
  ok: "bg-brand-400",
  critical: "bg-pop-400",
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
      <div role="status" aria-live="polite" className="fixed bottom-28 md:bottom-6 end-4 z-[60] flex flex-col gap-3 w-[min(92vw,22rem)] pointer-events-none" dir="rtl">
        <AnimatePresence initial={false}>
          {items.map((t) => {
            const Icon = icons[t.tone ?? "info"];
            return (
              <motion.div
                key={t.id}
                layout
                initial={{ opacity: 0, x: -40, rotate: -4, scale: 0.9 }}
                animate={{ opacity: 1, x: 0, rotate: 0, scale: 1 }}
                exit={{ opacity: 0, x: -40, scale: 0.9 }}
                transition={{ type: "spring", stiffness: 480, damping: 24 }}
                className="card pointer-events-auto p-3 flex items-start gap-3"
              >
                <span className={`size-9 shrink-0 rounded-xl border-2 border-[var(--line)] text-ink-900 flex items-center justify-center -rotate-6 ${chips[t.tone ?? "info"]}`}>
                  <Icon className="size-5" />
                </span>
                <div className="min-w-0 flex-1">
                  <div className="font-bold text-sm">{t.title}</div>
                  {t.body && <div className="muted text-xs mt-0.5">{t.body}</div>}
                  {t.href && (
                    <a href={t.href} className="text-sm font-semibold text-brand-700 dark:text-brand-300 hover:underline mt-1 inline-block">
                      افتح الحدث
                    </a>
                  )}
                </div>
                <button onClick={() => dismiss(t.id)} className="muted hover:text-current size-9 -m-1 rounded-full hover:bg-[var(--surface-2)] flex items-center justify-center" aria-label="إغلاق التنبيه">
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
