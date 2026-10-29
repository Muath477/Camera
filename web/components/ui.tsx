import { formatTime, label, type CameraEvent } from "@/lib/api";

export function Stat({ title, value, hint, tone }: { title: string; value: React.ReactNode; hint?: string; tone?: "ok" | "warn" | "bad" }) {
  const color = tone === "bad" ? "text-red-600" : tone === "warn" ? "text-amber-600" : "";
  return (
    <div className="card p-4">
      <div className="muted text-sm">{title}</div>
      <div className={`text-3xl font-bold mt-1 ${color}`}>{value}</div>
      {hint && <div className="muted text-xs mt-1">{hint}</div>}
    </div>
  );
}

const STATUS_STYLE: Record<string, string> = {
  pass: "bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-300",
  online: "bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-300",
  warn: "bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300",
  pending: "bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300",
  fail: "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300",
  offline: "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300",
  critical: "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300",
  warning: "bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300",
  info: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300",
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
};

export function Badge({ status }: { status: string }) {
  return (
    <span className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${STATUS_STYLE[status] ?? STATUS_STYLE.info}`}>
      {STATUS_AR[status] ?? status}
    </span>
  );
}

export function EventRow({ ev, siteName }: { ev: CameraEvent; siteName?: string }) {
  return (
    <li className="flex items-center gap-3 py-2.5 border-b border-[var(--border)] last:border-0">
      <Badge status={ev.severity} />
      <div className="min-w-0 flex-1">
        <div className="font-medium truncate">
          {label(ev.label)}
          {ev.score != null && <span className="muted text-xs font-normal"> · {Math.round(ev.score * 100)}%</span>}
        </div>
        <div className="muted text-xs truncate">
          {[ev.camera_name, siteName].filter(Boolean).join(" — ")}
        </div>
      </div>
      <time className="muted text-xs whitespace-nowrap">{formatTime(ev.time)}</time>
    </li>
  );
}

export function Loading() {
  return <div className="muted py-10 text-center">جارٍ التحميل…</div>;
}

export function ErrorBox({ error }: { error: string }) {
  return <div className="card p-4 text-red-600">تعذّر التحميل: {error}</div>;
}
