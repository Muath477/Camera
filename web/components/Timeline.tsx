"use client";

import { motion } from "motion/react";
import { useMemo, useRef, useState } from "react";
import { label, type CameraEvent } from "@/lib/api";
import { fmtTime } from "@/lib/format";

const DAY = 86_400_000;

const COLORS: Record<string, string> = {
  critical: "var(--color-bad-500)",
  warning: "var(--color-warn-500)",
  info: "var(--color-brand-500)",
};

// Timeline shows one day of a camera: event markers along the hours and a
// scrub handle. Dragging the handle picks the playback time.
export default function Timeline({
  day,
  events,
  value,
  onChange,
}: {
  day: Date; // start of the day in Riyadh time
  events: CameraEvent[];
  value: Date | null;
  onChange: (t: Date) => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [hover, setHover] = useState<number | null>(null);
  const start = day.getTime();
  const pos = (t: number) => Math.min(1, Math.max(0, (t - start) / DAY));
  const marks = useMemo(
    () =>
      events
        .filter((e) => Date.parse(e.time) >= start && Date.parse(e.time) < start + DAY)
        .map((e) => ({ e, x: pos(Date.parse(e.time)), w: e.end_time ? Math.max(0.002, (Date.parse(e.end_time) - Date.parse(e.time)) / DAY) : 0.002 })),
    [events, start],
  );
  const now = pos(Date.now());

  const pick = (clientX: number) => {
    const el = ref.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    // RTL: the right edge is 00:00.
    const frac = Math.min(1, Math.max(0, (r.right - clientX) / r.width));
    onChange(new Date(start + frac * DAY));
  };

  const hovered = hover != null ? marks.find((m) => Math.abs(m.x - hover) < 0.004) : null;

  return (
    <div className="select-none">
      <div
        ref={ref}
        className="relative h-16 rounded-lg surface-2 border border-[var(--border)] cursor-crosshair overflow-hidden"
        onMouseMove={(e) => {
          const r = ref.current!.getBoundingClientRect();
          setHover((r.right - e.clientX) / r.width);
        }}
        onMouseLeave={() => setHover(null)}
        onClick={(e) => pick(e.clientX)}
        onTouchMove={(e) => pick(e.touches[0].clientX)}
        onTouchStart={(e) => pick(e.touches[0].clientX)}
        role="slider"
        aria-label="وقت التشغيل"
        aria-valuemin={0}
        aria-valuemax={24}
        aria-valuenow={value ? (value.getTime() - start) / 36e5 : 0}
      >
        {/* hour grid */}
        {Array.from({ length: 25 }).map((_, h) => (
          <div key={h} className="absolute top-0 bottom-0 border-r border-[var(--border)]" style={{ right: `${(h / 24) * 100}%`, opacity: h % 6 === 0 ? 1 : 0.4 }} />
        ))}
        {/* future */}
        {now < 1 && <div className="absolute top-0 bottom-0 left-0 bg-[var(--border)]/40" style={{ width: `${(1 - now) * 100}%` }} />}
        {/* recorded band */}
        <div className="absolute bottom-0 h-2 right-0 bg-brand-200 dark:bg-brand-900/60" style={{ width: `${now * 100}%` }} />
        {/* events */}
        {marks.map((m, i) => (
          <motion.div
            key={m.e.device_id + m.e.edge_id}
            initial={{ scaleY: 0 }}
            animate={{ scaleY: 1 }}
            transition={{ delay: Math.min(i * 0.004, 0.4), duration: 0.3 }}
            className="absolute top-3 bottom-3 rounded-sm origin-bottom"
            style={{ right: `${m.x * 100}%`, width: `max(3px, ${m.w * 100}%)`, background: COLORS[m.e.severity] ?? COLORS.info, opacity: m.e.severity === "info" ? 0.55 : 0.95 }}
          />
        ))}
        {/* hover line */}
        {hover != null && <div className="absolute top-0 bottom-0 w-px bg-ink-500 pointer-events-none" style={{ right: `${hover * 100}%` }} />}
        {/* handle */}
        {value && (
          <motion.div
            layout
            transition={{ type: "spring", stiffness: 500, damping: 40 }}
            className="absolute top-0 bottom-0 w-0.5 bg-brand-600 pointer-events-none"
            style={{ right: `${pos(value.getTime()) * 100}%` }}
          >
            <span className="absolute -top-0 -right-1.5 size-3.5 rounded-full bg-brand-600 border-2 border-white shadow" />
          </motion.div>
        )}
        {/* hover tooltip */}
        {hover != null && (
          <div className="absolute top-1 text-[10px] px-1.5 py-0.5 rounded bg-ink-900 text-white pointer-events-none whitespace-nowrap" style={{ right: `calc(${hover * 100}% + 6px)` }}>
            {fmtTime(new Date(start + hover * DAY))}
            {hovered && <> · {label(hovered.e.label)}</>}
          </div>
        )}
      </div>
      <div className="flex justify-between muted text-[10px] mt-1 px-0.5" dir="rtl">
        {[0, 6, 12, 18, 24].map((h) => (
          <span key={h}>{String(h).padStart(2, "0")}:00</span>
        ))}
      </div>
    </div>
  );
}
