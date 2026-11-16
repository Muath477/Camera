"use client";

import { animate, motion, useInView, useReducedMotion } from "motion/react";
import Link from "next/link";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { Inbox, type LucideIcon } from "lucide-react";
import Mascot from "../Mascot";

/* ---------- Button ---------- */

type ButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "secondary" | "ghost" | "danger";
  size?: "sm" | "md" | "lg";
  icon?: LucideIcon;
  loading?: boolean;
};

const variants = {
  primary: "btn-pop bg-brand-500 text-ink-900 hover:bg-brand-400 font-bold",
  secondary: "btn-pop bg-[var(--surface)] hover:bg-sun-100 dark:border-[var(--border-strong)] dark:hover:bg-[var(--surface-2)] font-semibold",
  ghost: "hover:bg-[var(--surface-2)] font-medium",
  danger: "btn-pop bg-bad-500 text-white hover:bg-bad-600 font-bold",
};

const sizes = { sm: "h-9 px-3.5 text-xs gap-1.5", md: "h-11 px-5 text-sm gap-2", lg: "h-12 px-7 text-base gap-2" };

export function Button({ variant = "primary", size = "md", icon: Icon, loading, className = "", children, ...rest }: ButtonProps) {
  return (
    <button
      {...rest}
      disabled={rest.disabled || loading}
      aria-busy={loading || undefined}
      className={`group inline-flex items-center justify-center rounded-full transition-colors duration-150 disabled:opacity-50 disabled:pointer-events-none ${variants[variant]} ${sizes[size]} ${className}`}
    >
      {loading ? <Spinner /> : Icon ? <Icon className="size-4 wiggle" /> : null}
      {children}
    </button>
  );
}

export function Spinner({ className = "size-4" }: { className?: string }) {
  return (
    <svg className={`animate-spin ${className}`} viewBox="0 0 24 24" fill="none" aria-hidden>
      <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="3" className="opacity-25" />
      <path d="M22 12a10 10 0 0 1-10 10" stroke="currentColor" strokeWidth="3" strokeLinecap="round" />
    </svg>
  );
}

/* ---------- Badge ---------- */

const OK = "bg-brand-100 text-brand-800 border-brand-300 dark:bg-brand-900/50 dark:text-brand-200 dark:border-brand-700";
const WARN = "bg-sun-100 text-sun-700 border-sun-300 dark:bg-sun-400/15 dark:text-sun-300 dark:border-sun-400/40";
const BAD = "bg-pop-100 text-pop-700 border-pop-400/60 dark:bg-pop-400/15 dark:text-pop-400 dark:border-pop-400/40";
const IDLE = "bg-ink-100 text-ink-700 border-ink-200 dark:bg-ink-800 dark:text-ink-300 dark:border-ink-700";

const STATUS_STYLE: Record<string, string> = {
  pass: OK,
  online: OK,
  ok: OK,
  warn: WARN,
  warning: WARN,
  pending: WARN,
  fail: BAD,
  offline: BAD,
  critical: BAD,
  info: IDLE,
  unknown: IDLE,
};

const STATUS_AR: Record<string, string> = {
  pass: "مستوفى",
  warn: "تنبيه",
  fail: "غير مستوفى",
  online: "متصل",
  offline: "منقطع",
  pending: "بانتظار التفعيل",
  unknown: "غير معروف",
  critical: "حرج",
  warning: "تحذير",
  info: "عادي",
  ok: "سليم",
};

export function Badge({ status, children, dot }: { status: string; children?: ReactNode; dot?: boolean }) {
  return (
    <span className={`inline-flex items-center gap-1.5 rounded-full border-[1.5px] px-2.5 py-0.5 text-xs font-semibold whitespace-nowrap ${STATUS_STYLE[status] ?? STATUS_STYLE.info}`}>
      {dot && <span className={`pulse-dot ${status === "critical" || status === "offline" || status === "fail" ? "bad" : ""}`} />}
      {children ?? STATUS_AR[status] ?? status}
    </span>
  );
}

/* ---------- Skeleton ---------- */

export function Skeleton({ className = "" }: { className?: string }) {
  return <div className={`skeleton ${className}`} aria-hidden />;
}

export function CardSkeleton({ lines = 3 }: { lines?: number }) {
  return (
    <div className="card p-4 space-y-3">
      <Skeleton className="h-4 w-1/3" />
      {Array.from({ length: lines }).map((_, i) => (
        <div key={i} className="skeleton h-3" style={{ width: `${90 - i * 15}%` }} />
      ))}
    </div>
  );
}

/* ---------- Animated number ---------- */

export function Counter({ value, className = "" }: { value: number; className?: string }) {
  const ref = useRef<HTMLSpanElement>(null);
  const inView = useInView(ref, { once: true });
  const reduce = useReducedMotion();
  useEffect(() => {
    const el = ref.current;
    if (!el || !inView) return;
    if (reduce) {
      el.textContent = String(value);
      return;
    }
    const from = Number(el.textContent) || 0;
    const controls = animate(from, value, {
      duration: 0.9,
      ease: [0.16, 1, 0.3, 1],
      onUpdate: (v) => (el.textContent = String(Math.round(v))),
    });
    return () => controls.stop();
  }, [value, inView, reduce]);
  return <span ref={ref} className={`tabular-nums ${className}`}>0</span>;
}

/* ---------- Stat card ---------- */

export function Stat({
  title,
  value,
  hint,
  tone,
  icon: Icon,
  href,
}: {
  title: string;
  value: ReactNode;
  hint?: string;
  tone?: "ok" | "warn" | "bad";
  icon?: LucideIcon;
  href?: string;
}) {
  // Each tone gets its own sticker colour so the row reads at a glance.
  const chip = tone === "bad" ? "bg-pop-400 text-ink-900" : tone === "warn" ? "bg-sun-400 text-ink-900" : tone === "ok" ? "bg-brand-400 text-ink-900" : "bg-aqua-400 text-ink-900";
  const body = (
    <div className="card card-hover group p-4 h-full relative overflow-hidden">
      {Icon && (
        <span className={`absolute top-3 end-3 size-10 rounded-xl border-2 border-[var(--line)] rotate-[-8deg] flex items-center justify-center ${chip}`}>
          <Icon className="size-5 wiggle" />
        </span>
      )}
      <div className="min-w-0 pe-12">
        <div className="muted text-sm font-medium">{title}</div>
      </div>
      <div className="font-display text-3xl md:text-4xl font-extrabold mt-2 leading-none tabular-nums">{value}</div>
      {hint && <div className="muted text-xs mt-2 truncate">{hint}</div>}
    </div>
  );
  return href ? <Link href={href} className="block h-full rounded-[var(--radius-md)]">{body}</Link> : body;
}

/* ---------- Empty, error, loading ---------- */

export function EmptyState({ icon: Icon = Inbox, title, hint, action }: { icon?: LucideIcon; title: string; hint?: string; action?: ReactNode }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      className="flex flex-col items-center justify-center text-center py-14 px-6 gap-2"
    >
      <span className="relative mb-2">
        <Mascot size={72} mood="sleepy" />
        <span className="absolute -bottom-1 -end-2 size-8 rounded-full border-2 border-[var(--line)] bg-sun-300 text-ink-900 flex items-center justify-center rotate-12">
          <Icon className="size-4" />
        </span>
      </span>
      <div className="font-display text-lg font-bold">{title}</div>
      {hint && <div className="muted text-sm max-w-sm">{hint}</div>}
      {action && <div className="mt-3">{action}</div>}
    </motion.div>
  );
}

export function ErrorBox({ error, retry }: { error: string; retry?: () => void }) {
  return (
    <div role="alert" className="card p-4 flex items-center justify-between gap-3">
      <Mascot size={44} mood="alert" className="shrink-0" />
      <div className="text-sm flex-1 min-w-0">
        <div className="font-bold text-bad-600 dark:text-pop-400">تعذّر التحميل</div>
        <div className="muted text-xs" dir="auto">{error}</div>
      </div>
      {retry && (
        <Button variant="secondary" size="sm" onClick={retry}>
          إعادة المحاولة
        </Button>
      )}
    </div>
  );
}

export function Loading({ label = "جارٍ التحميل…" }: { label?: string }) {
  return (
    <div className="muted py-10 text-center text-sm flex items-center justify-center gap-2">
      <Spinner /> {label}
    </div>
  );
}

/* ---------- Page header ---------- */

export function PageHeader({ title, subtitle, actions, crumbs }: { title: ReactNode; subtitle?: ReactNode; actions?: ReactNode; crumbs?: { href: string; label: string }[] }) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-3 mb-8">
      <div className="min-w-0">
        {crumbs && (
          <nav aria-label="مسار التنقل" className="muted text-xs mb-1 flex items-center gap-1">
            {crumbs.map((c, i) => (
              <span key={c.href} className="flex items-center gap-1">
                {i > 0 && <span>/</span>}
                <Link href={c.href} className="hover:text-brand-700 dark:hover:text-brand-300">{c.label}</Link>
              </span>
            ))}
          </nav>
        )}
        <h1 className="text-3xl md:text-4xl font-extrabold">
          <span className="squiggle">{title}</span>
        </h1>
        {subtitle && <p className="muted text-sm mt-3">{subtitle}</p>}
      </div>
      {actions && <div className="flex items-center gap-2 shrink-0">{actions}</div>}
    </div>
  );
}

/* ---------- Progress ring ---------- */

export function Ring({ value, size = 96, stroke = 9, label }: { value: number; size?: number; stroke?: number; label?: ReactNode }) {
  const r = (size - stroke) / 2;
  const c = 2 * Math.PI * r;
  const tone = value >= 80 ? "var(--color-brand-500)" : value >= 50 ? "var(--color-warn-500)" : "var(--color-bad-500)";
  const [mounted, setMounted] = useState(false);
  useEffect(() => setMounted(true), []);
  return (
    <div className="relative inline-flex items-center justify-center" style={{ width: size, height: size }}>
      <svg width={size} height={size} className="-rotate-90">
        <circle cx={size / 2} cy={size / 2} r={r} stroke="var(--border)" strokeWidth={stroke} fill="none" />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          stroke={tone}
          strokeWidth={stroke}
          strokeLinecap="round"
          fill="none"
          strokeDasharray={c}
          strokeDashoffset={mounted ? c * (1 - value / 100) : c}
          style={{ transition: "stroke-dashoffset 1.1s var(--ease-out)" }}
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className="font-bold text-xl leading-none">
          <Counter value={value} />%
        </span>
        {label && <span className="muted text-[10px] mt-1">{label}</span>}
      </div>
    </div>
  );
}

/* ---------- Motion presets ---------- */

export const fadeUp = {
  hidden: { opacity: 0, y: 10 },
  show: { opacity: 1, y: 0, transition: { duration: 0.35, ease: [0.16, 1, 0.3, 1] as const } },
};

export const stagger = {
  hidden: {},
  show: { transition: { staggerChildren: 0.05 } },
};

export function Reveal({ children, className = "", delay = 0 }: { children: ReactNode; className?: string; delay?: number }) {
  return (
    <motion.div
      className={className}
      initial={{ opacity: 0, y: 12 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, amount: 0.15 }}
      transition={{ duration: 0.45, delay, ease: [0.16, 1, 0.3, 1] }}
    >
      {children}
    </motion.div>
  );
}
