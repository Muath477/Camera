"use client";

import { AnimatePresence, motion } from "motion/react";
import { useEffect, useMemo, useState } from "react";
import { Search, Sparkles, X } from "lucide-react";
import { EventList } from "@/components/EventFeed";
import { useLive } from "@/components/Live";
import { Button, EmptyState, ErrorBox, PageHeader, Skeleton } from "@/components/ui";
import { LABELS, type CameraEvent, type Site } from "@/lib/api";
import { dayKey, fmtDay } from "@/lib/format";
import { useApi } from "@/lib/hooks";

const SEVERITIES = [
  { id: "", label: "الكل" },
  { id: "critical", label: "حرج" },
  { id: "warning", label: "تحذير" },
  { id: "info", label: "عادي" },
];

const QUICK = ["شخص عند الباب بعد منتصف الليل", "سيارة تقف أمام المدخل", "شخص يحمل صندوقاً", "دراجة نارية في الممر"];

export default function EventsPage() {
  const [site, setSite] = useState("");
  const [lbl, setLbl] = useState("");
  const [severity, setSeverity] = useState("");

  useEffect(() => {
    const sev = new URLSearchParams(location.search).get("severity");
    if (sev && SEVERITIES.some((x) => x.id === sev)) setSeverity(sev);
  }, []);
  const [query, setQuery] = useState("");
  const [submitted, setSubmitted] = useState("");
  const sites = useApi<{ items: Site[] | null }>("/v1/sites");
  const live = useLive();

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
    return [...pushed, ...base].filter((e) => {
      const k = e.device_id + e.edge_id;
      if (seen.has(k) || (severity && e.severity !== severity)) return false;
      seen.add(k);
      return true;
    });
  }, [events.data, live.events, submitted, site, lbl, severity]);

  const groups = useMemo(() => {
    const m = new Map<string, CameraEvent[]>();
    for (const e of list) {
      const k = dayKey(e.time);
      if (!m.has(k)) m.set(k, []);
      m.get(k)!.push(e);
    }
    return [...m.entries()];
  }, [list]);

  const chip = "rounded-full px-3 py-1 text-xs font-medium border transition-colors";

  return (
    <div>
      <PageHeader title="الأحداث" subtitle="كل ما رصدته الكاميرات، مباشرة ومؤرشفاً، مع بحث بالوصف عبر الفروع." />

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
                  <EventList events={evs} siteNames={siteName} ackable />
                </div>
              </motion.section>
            ))}
          </AnimatePresence>
        )}
      </div>
    </div>
  );
}
