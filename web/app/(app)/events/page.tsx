"use client";

import { AnimatePresence, motion } from "motion/react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { CheckCheck, ListChecks, Search, Sparkles, X } from "lucide-react";
import { EventList, ackable, eventKey } from "@/components/EventFeed";
import { useLive } from "@/components/Live";
import { useToast } from "@/components/Toast";
import { Button, EmptyState, ErrorBox, PageHeader, Skeleton } from "@/components/ui";
import { LABELS, api, type CameraEvent, type Site } from "@/lib/api";
import { dayKey, fmtDay } from "@/lib/format";
import { useApi } from "@/lib/hooks";

const SEVERITIES = [
  { id: "", label: "الكل" },
  { id: "critical", label: "حرج" },
  { id: "warning", label: "تحذير" },
  { id: "info", label: "عادي" },
];

const QUICK = ["شخص عند الباب بعد منتصف الليل", "سيارة تقف أمام المدخل", "شخص يحمل صندوقاً", "دراجة نارية في الممر"];

type Ack = { acked_at: string; acked_by_name?: string; ack_note?: string };

export default function EventsPage() {
  const [site, setSite] = useState("");
  const [lbl, setLbl] = useState("");
  const [severity, setSeverity] = useState("");
  const [query, setQuery] = useState("");
  const [submitted, setSubmitted] = useState("");
  const sites = useApi<{ items: Site[] | null }>("/v1/sites");
  const live = useLive();
  const toast = useToast();

  // Selection mode: pick several alerts and acknowledge them with one note.
  const [selecting, setSelecting] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  // Acknowledgements made on this page, applied over loaded and pushed events.
  const [acks, setAcks] = useState<Record<string, Ack>>({});

  useEffect(() => {
    const sev = new URLSearchParams(location.search).get("severity");
    if (sev && SEVERITIES.some((x) => x.id === sev)) setSeverity(sev);
  }, []);

  const params = new URLSearchParams({ limit: "200" });
  if (site) params.set("site_id", site);
  if (lbl) params.set("label", lbl);
  const path = submitted ? `/v1/search?q=${encodeURIComponent(submitted)}&${params}` : `/v1/events?${params}`;
  const events = useApi<{ items: CameraEvent[] | null }>(path);
  const siteName = useMemo(() => Object.fromEntries((sites.data?.items ?? []).map((s) => [s.id, s.name])), [sites.data]);

  const list = useMemo(() => {
    const base = events.data?.items ?? [];
    const pushed = submitted ? [] : live.events.filter((e) => (!site || e.site_id === site) && (!lbl || e.label === lbl));
    const seen = new Set<string>();
    return [...pushed, ...base]
      .filter((e) => {
        const k = eventKey(e);
        if (seen.has(k) || (severity && e.severity !== severity)) return false;
        seen.add(k);
        return true;
      })
      .map((e) => (acks[eventKey(e)] ? { ...e, ...acks[eventKey(e)] } : e));
  }, [events.data, live.events, submitted, site, lbl, severity, acks]);

  const groups = useMemo(() => {
    const m = new Map<string, CameraEvent[]>();
    for (const e of list) {
      const k = dayKey(e.time);
      if (!m.has(k)) m.set(k, []);
      m.get(k)!.push(e);
    }
    return [...m.entries()];
  }, [list]);

  const pending = useMemo(() => list.filter(ackable), [list]);

  const toggle = useCallback((ev: CameraEvent) => {
    setSelected((s) => {
      const n = new Set(s);
      const k = eventKey(ev);
      if (n.has(k)) n.delete(k);
      else n.add(k);
      return n;
    });
  }, []);

  const selectAll = useCallback(() => setSelected(new Set(pending.map(eventKey))), [pending]);
  const exit = useCallback(() => {
    setSelecting(false);
    setSelected(new Set());
    setNote("");
  }, []);

  const ackSelected = useCallback(async () => {
    const items = pending.filter((e) => selected.has(eventKey(e)));
    if (items.length === 0 || busy) return;
    setBusy(true);
    try {
      const res = await api<{ acknowledged: number }>("/v1/events/ack", {
        method: "POST",
        body: JSON.stringify({ items: items.map((e) => ({ device_id: e.device_id, edge_id: e.edge_id })), note: note.trim() }),
      });
      const now = new Date().toISOString();
      setAcks((m) => {
        const n = { ...m };
        for (const e of items) n[eventKey(e)] = { acked_at: now, ack_note: note.trim() || undefined };
        return n;
      });
      toast.push({ tone: "ok", title: `تم إقرار ${res.acknowledged} تنبيه`, body: note.trim() ? "الملاحظة محفوظة في سجل التدقيق." : "مسجّل في سجل التدقيق." });
      exit();
    } catch {
      toast.push({ tone: "critical", title: "تعذّر الإقرار", body: "تحقق من الاتصال وحاول مرة أخرى." });
    } finally {
      setBusy(false);
    }
  }, [pending, selected, busy, note, toast, exit]);

  // Keyboard: Ctrl+A selects every pending alert, R acknowledges, Esc leaves.
  useEffect(() => {
    if (!selecting) return;
    const onKey = (e: KeyboardEvent) => {
      const typing = (e.target as HTMLElement)?.tagName === "INPUT" || (e.target as HTMLElement)?.tagName === "TEXTAREA";
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "a" && !typing) {
        e.preventDefault();
        selectAll();
      } else if (e.key === "Escape") {
        exit();
      } else if (e.key.toLowerCase() === "r" && !typing && selected.size > 0) {
        e.preventDefault();
        ackSelected();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [selecting, selectAll, exit, ackSelected, selected.size]);

  const chip = "rounded-full px-3 py-1 text-xs font-medium border transition-colors";

  return (
    <div>
      <PageHeader
        title="الأحداث"
        subtitle="كل ما رصدته الكاميرات، مباشرة ومؤرشفاً، مع بحث بالوصف عبر الفروع."
        actions={
          pending.length > 0 && (
            <Button variant={selecting ? "primary" : "secondary"} icon={ListChecks} onClick={() => (selecting ? exit() : setSelecting(true))}>
              {selecting ? "إلغاء التحديد" : `تحديد (${pending.length} بانتظار الإقرار)`}
            </Button>
          )
        }
      />

      <form
        className="card p-3 space-y-3"
        onSubmit={(e) => {
          e.preventDefault();
          setSubmitted(query.trim());
        }}
      >
        <div className="flex gap-2">
          <div className="relative flex-1">
            <Search className="size-4 muted absolute right-3 top-1/2 -translate-y-1/2" />
            <input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="ابحث بالوصف: شخص يحمل صندوقاً عند البوابة" className="input w-full pr-9" />
            {query && (
              <button type="button" onClick={() => { setQuery(""); setSubmitted(""); }} className="absolute left-2 top-1/2 -translate-y-1/2 muted hover:text-current" aria-label="مسح">
                <X className="size-4" />
              </button>
            )}
          </div>
          <Button type="submit" icon={Sparkles}>بحث</Button>
        </div>
        <div className="flex flex-wrap gap-2 items-center">
          {SEVERITIES.map((s) => (
            <button key={s.id} type="button" onClick={() => setSeverity(s.id)} className={`${chip} ${severity === s.id ? "bg-ink-900 text-white border-ink-900 dark:bg-white dark:text-ink-900 dark:border-white" : "border-[var(--border)] hover:border-[var(--border-strong)]"}`}>
              {s.label}
            </button>
          ))}
          <span className="w-px h-5 bg-[var(--border)] mx-1" />
          <select value={site} onChange={(e) => setSite(e.target.value)} className="input !py-1 text-xs">
            <option value="">كل المواقع</option>
            {(sites.data?.items ?? []).map((s) => (
              <option key={s.id} value={s.id}>{s.name}</option>
            ))}
          </select>
          <select value={lbl} onChange={(e) => setLbl(e.target.value)} className="input !py-1 text-xs">
            <option value="">كل التصنيفات</option>
            {Object.entries(LABELS).map(([k, v]) => (
              <option key={k} value={k}>{v}</option>
            ))}
          </select>
        </div>
        {!submitted && !query && (
          <div className="flex flex-wrap gap-2 items-center text-xs">
            <span className="muted">جرّب:</span>
            {QUICK.map((q) => (
              <button key={q} type="button" onClick={() => { setQuery(q); setSubmitted(q); }} className={`${chip} border-dashed border-[var(--border-strong)] hover:bg-[var(--surface-2)]`}>
                {q}
              </button>
            ))}
          </div>
        )}
      </form>

      <div className="mt-4">
        {events.error ? (
          <ErrorBox error={events.error === "text search is not enabled" ? "البحث بالوصف يحتاج خدمة التضمين النصي (EMBEDDER_URL). الفلاتر العادية تعمل." : events.error} retry={events.reload} />
        ) : events.loading && !events.data ? (
          <div className="card p-4 space-y-3">
            {[0, 1, 2, 3, 4, 5].map((i) => (
              <Skeleton key={i} className="h-10" />
            ))}
          </div>
        ) : groups.length === 0 ? (
          <div className="card">
            <EmptyState title="لا توجد نتائج" hint={submitted ? "جرّب وصفاً آخر أو وسّع نطاق المواقع." : "لم تُرصد أحداث بهذه الفلاتر خلال آخر 7 أيام."} />
          </div>
        ) : (
          <AnimatePresence initial={false}>
            {groups.map(([day, evs]) => (
              <motion.section key={day} layout initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="mb-5">
                <h2 className="muted text-xs font-medium mb-2 sticky top-14 md:top-0 bg-[var(--bg)] py-1 z-10">
                  {fmtDay(evs[0].time)} · {evs.length}
                </h2>
                <div className="card px-4">
                  <EventList
                    events={evs}
                    siteNames={siteName}
                    ackable
                    selected={selecting ? selected : undefined}
                    onSelect={selecting ? toggle : undefined}
                    onChanged={(ev) => setAcks((m) => ({ ...m, [eventKey(ev)]: { acked_at: ev.acked_at! } }))}
                  />
                </div>
              </motion.section>
            ))}
          </AnimatePresence>
        )}
      </div>

      <AnimatePresence>
        {selecting && (
          <motion.div
            initial={{ opacity: 0, y: 24 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 24 }}
            transition={{ type: "spring", stiffness: 380, damping: 32 }}
            className="fixed bottom-20 md:bottom-6 inset-x-3 md:inset-x-auto md:right-[calc(16rem+2rem)] md:left-8 z-40 card shadow-[var(--shadow-lg)] p-3 flex flex-col sm:flex-row sm:items-center gap-2"
          >
            <div className="flex items-center gap-2 text-sm shrink-0">
              <span className="font-semibold">{selected.size}</span>
              <span className="muted">محدد من {pending.length}</span>
              <button type="button" onClick={selectAll} className="text-brand-600 text-xs hover:underline">
                تحديد الكل
              </button>
            </div>
            <input
              value={note}
              onChange={(e) => setNote(e.target.value)}
              placeholder="ملاحظة اختيارية: ما الذي تم؟ (تُحفظ في سجل التدقيق)"
              maxLength={500}
              className="input flex-1 text-sm"
            />
            <div className="flex gap-2 shrink-0">
              <Button variant="secondary" onClick={exit}>
                إلغاء
              </Button>
              <Button icon={CheckCheck} loading={busy} disabled={selected.size === 0} onClick={ackSelected}>
                إقرار {selected.size > 0 ? `(${selected.size})` : ""}
              </Button>
            </div>
            <span className="hidden lg:block muted text-[11px] shrink-0">Ctrl+A تحديد الكل · R إقرار · Esc خروج</span>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
