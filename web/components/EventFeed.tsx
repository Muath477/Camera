"use client";

import { AnimatePresence, motion } from "motion/react";
import Link from "next/link";
import { useState } from "react";
import { Bell, Car, Check, Flame, PersonStanding, Siren, Volume2, type LucideIcon } from "lucide-react";
import { api, label, type CameraEvent } from "@/lib/api";
import { fmtRelative, fmtTime } from "@/lib/format";
import { Badge, EmptyState } from "./ui";

const ICONS: Record<string, LucideIcon> = {
  person: PersonStanding,
  car: Car,
  motorcycle: Car,
  bicycle: Car,
  fire_alarm: Flame,
  smoke_detector: Flame,
  glass: Volume2,
  scream: Volume2,
  yell: Volume2,
  siren: Siren,
  gunshot: Siren,
};

export function eventKey(ev: CameraEvent) {
  return ev.device_id + "/" + ev.edge_id;
}

export function EventRow({
  ev,
  siteName,
  onAck,
  compact,
}: {
  ev: CameraEvent;
  siteName?: string;
  onAck?: (ev: CameraEvent) => void;
  compact?: boolean;
}) {
  const Icon = ICONS[ev.label] ?? Bell;
  const tone = ev.severity === "critical" ? "text-bad-500 bg-rose-50 dark:bg-rose-900/30" : ev.severity === "warning" ? "text-warn-600 bg-amber-50 dark:bg-amber-900/30" : "text-ink-500 bg-ink-100 dark:bg-ink-800";
  return (
    <motion.li
      layout
      initial={{ opacity: 0, x: 16 }}
      animate={{ opacity: 1, x: 0 }}
      exit={{ opacity: 0, height: 0 }}
      transition={{ type: "spring", stiffness: 420, damping: 36 }}
      className={`flex items-center gap-3 ${compact ? "py-2" : "py-3"} border-b border-[var(--border)] last:border-0`}
    >
      <span className={`shrink-0 size-9 rounded-lg flex items-center justify-center ${tone}`}>
        <Icon className="size-4" />
      </span>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="font-medium truncate">{label(ev.label)}</span>
          {ev.score != null && <span className="muted text-xs">{Math.round(ev.score * 100)}%</span>}
          {!ev.end_time && ev.kind !== "system" && <span className="pulse-dot" title="مستمر" />}
          {ev.acked_at && <Check className="size-3.5 text-brand-600" />}
        </div>
        <div className="muted text-xs truncate">
          {ev.camera_id ? (
            <Link href={`/cameras/${ev.camera_id}`} className="hover:text-brand-600">{ev.camera_name}</Link>
          ) : (
            ev.camera_name
          )}
          {siteName && <> — {siteName}</>}
        </div>
      </div>
      <div className="text-left shrink-0">
        <time className="muted text-xs block" title={fmtTime(ev.time)}>
          {fmtRelative(ev.time)}
        </time>
        {!compact && <Badge status={ev.severity} />}
      </div>
      {onAck && ev.severity !== "info" && !ev.acked_at && (
        <button
          onClick={() => onAck(ev)}
          className="shrink-0 size-8 rounded-lg border border-[var(--border)] hover:bg-brand-50 hover:border-brand-300 dark:hover:bg-brand-900/30 flex items-center justify-center"
          title="إقرار"
          aria-label="إقرار"
        >
          <Check className="size-4" />
        </button>
      )}
    </motion.li>
  );
}

export function EventList({
  events,
  siteNames = {},
  compact,
  ackable,
  onChanged,
  emptyHint,
}: {
  events: CameraEvent[];
  siteNames?: Record<string, string>;
  compact?: boolean;
  ackable?: boolean;
  onChanged?: (ev: CameraEvent) => void;
  emptyHint?: string;
}) {
  const [acked, setAcked] = useState<Record<string, string>>({});
  const ack = async (ev: CameraEvent) => {
    const now = new Date().toISOString();
    setAcked((m) => ({ ...m, [eventKey(ev)]: now }));
    try {
      await api(`/v1/events/${ev.device_id}/${ev.edge_id}/ack`, { method: "POST" });
      onChanged?.({ ...ev, acked_at: now });
    } catch {
      setAcked((m) => {
        const n = { ...m };
        delete n[eventKey(ev)];
        return n;
      });
    }
  };
  if (events.length === 0) return <EmptyState icon={Bell} title="لا توجد أحداث" hint={emptyHint ?? "ستظهر الأحداث هنا فور رصدها من الكاميرات."} />;
  return (
    <ul>
      <AnimatePresence initial={false}>
        {events.map((ev) => (
          <EventRow
            key={eventKey(ev)}
            ev={acked[eventKey(ev)] ? { ...ev, acked_at: acked[eventKey(ev)] } : ev}
            siteName={siteNames[ev.site_id]}
            compact={compact}
            onAck={ackable ? ack : undefined}
          />
        ))}
      </AnimatePresence>
    </ul>
  );
}
