"use client";

import { AnimatePresence, motion } from "motion/react";
import Link from "next/link";
import { useState } from "react";
import { Bell, Car, Check, Flame, MessageSquareText, PersonStanding, Siren, Volume2, type LucideIcon } from "lucide-react";
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

export function ackable(ev: CameraEvent) {
  return ev.severity !== "info" && !ev.acked_at;
}

export function EventRow({
  ev,
  siteName,
  onAck,
  compact,
  selected,
  onSelect,
}: {
  ev: CameraEvent;
  siteName?: string;
  onAck?: (ev: CameraEvent) => void;
  compact?: boolean;
  // When onSelect is set the row shows a checkbox instead of the ack button.
  selected?: boolean;
  onSelect?: (ev: CameraEvent) => void;
}) {
  const Icon = ICONS[ev.label] ?? Bell;
  // Sticker chip per severity: coral acts now, sun looks soon, aqua is info.
  const tone = ev.severity === "critical" ? "bg-pop-400 -rotate-6" : ev.severity === "warning" ? "bg-sun-400 rotate-3" : "bg-aqua-100 dark:bg-aqua-400";
  const selectable = !!onSelect && ackable(ev);
  return (
    <motion.li
      layout
      initial={{ opacity: 0, x: 16 }}
      animate={{ opacity: 1, x: 0 }}
      exit={{ opacity: 0, height: 0 }}
      transition={{ type: "spring", stiffness: 420, damping: 36 }}
      onClick={selectable ? () => onSelect!(ev) : undefined}
      className={`flex items-center gap-3 ${compact ? "py-2" : "py-3"} border-b border-[var(--border)] last:border-0 ${selectable ? "cursor-pointer" : ""} ${selected ? "bg-brand-50/60 dark:bg-brand-900/20 -mx-4 px-4" : ""}`}
    >
      {onSelect && (
        <span className={`shrink-0 size-5 rounded-md border flex items-center justify-center transition-colors ${selected ? "bg-brand-600 border-brand-600 text-white" : selectable ? "border-[var(--border-strong)]" : "border-[var(--border)] opacity-30"}`} aria-hidden>
          {selected && <Check className="size-3.5" />}
        </span>
      )}
      <span className={`shrink-0 size-10 rounded-xl border-2 border-[var(--line)] text-ink-900 flex items-center justify-center ${tone}`}>
        <Icon className="size-4" />
      </span>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="font-medium truncate">{label(ev.label)}</span>
          {ev.score != null && <span className="muted text-xs">{Math.round(ev.score * 100)}%</span>}
          {!ev.end_time && ev.kind !== "system" && <span className="pulse-dot" title="مستمر" />}
          {ev.acked_at && <Check className="size-3.5 text-brand-700 dark:text-brand-300" />}
        </div>
        <div className="muted text-xs truncate">
          {ev.camera_id ? (
            <Link href={`/cameras/${ev.camera_id}`} onClick={(e) => e.stopPropagation()} className="hover:text-brand-700 dark:hover:text-brand-300">{ev.camera_name}</Link>
          ) : (
            ev.camera_name
          )}
          {siteName && <> — {siteName}</>}
        </div>
        {ev.acked_at && (ev.acked_by_name || ev.ack_note) && !compact && (
          <div className="muted text-[11px] mt-0.5 flex items-center gap-1 truncate">
            <MessageSquareText className="size-3 shrink-0" />
            {ev.acked_by_name && <span>أقرّه {ev.acked_by_name}</span>}
            {ev.ack_note && <span className="truncate">· {ev.ack_note}</span>}
          </div>
        )}
      </div>
      <div className="text-end shrink-0">
        <time className="muted text-xs block" title={fmtTime(ev.time)}>
          {fmtRelative(ev.time)}
        </time>
        {!compact && <Badge status={ev.severity} />}
      </div>
      {onAck && !onSelect && ackable(ev) && (
        <button
          onClick={(e) => {
            e.stopPropagation();
            onAck(ev);
          }}
          className="group shrink-0 size-10 rounded-full border-2 border-[var(--card-line)] bg-[var(--surface)] hover:bg-brand-400 hover:text-ink-900 hover:border-[var(--line)] transition-colors flex items-center justify-center"
          title="إقرار"
          aria-label="إقرار"
        >
          <Check className="size-4 wiggle" />
        </button>
      )}
    </motion.li>
  );
}

export function EventList({
  events,
  siteNames = {},
  compact,
  ackable: canAck,
  onChanged,
  emptyHint,
  selected,
  onSelect,
}: {
  events: CameraEvent[];
  siteNames?: Record<string, string>;
  compact?: boolean;
  ackable?: boolean;
  onChanged?: (ev: CameraEvent) => void;
  emptyHint?: string;
  selected?: Set<string>;
  onSelect?: (ev: CameraEvent) => void;
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
  if (events.length === 0) return <EmptyState icon={Bell} title="هدوء تام، لا أحداث" hint={emptyHint ?? "ستظهر الأحداث هنا لحظة رصدها من الكاميرات."} />;
  return (
    <ul>
      <AnimatePresence initial={false}>
        {events.map((ev) => (
          <EventRow
            key={eventKey(ev)}
            ev={acked[eventKey(ev)] ? { ...ev, acked_at: acked[eventKey(ev)] } : ev}
            siteName={siteNames[ev.site_id]}
            compact={compact}
            onAck={canAck ? ack : undefined}
            selected={selected?.has(eventKey(ev))}
            onSelect={onSelect}
          />
        ))}
      </AnimatePresence>
    </ul>
  );
}
