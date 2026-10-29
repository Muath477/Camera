"use client";

import { useMemo, useState } from "react";
import { ErrorBox, EventRow, Loading } from "@/components/ui";
import { LABELS, type CameraEvent, type Site } from "@/lib/api";
import { useApi } from "@/lib/hooks";

export default function EventsPage() {
  const [site, setSite] = useState("");
  const [lbl, setLbl] = useState("");
  const [query, setQuery] = useState("");
  const [submitted, setSubmitted] = useState("");
  const sites = useApi<{ items: Site[] | null }>("/v1/sites");

  const params = new URLSearchParams({ limit: "100" });
  if (site) params.set("site_id", site);
  if (lbl) params.set("label", lbl);
  const path = submitted ? `/v1/search?q=${encodeURIComponent(submitted)}&${params}` : `/v1/events?${params}`;
  const events = useApi<{ items: CameraEvent[] | null }>(path);
  const siteName = useMemo(() => Object.fromEntries((sites.data?.items ?? []).map((s) => [s.id, s.name])), [sites.data]);

  const select = "rounded-lg border border-[var(--border)] bg-[var(--surface)] px-3 py-2 text-sm";

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">الأحداث والتصنيفات</h1>
      <form
        className="card p-3 flex flex-wrap gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          setSubmitted(query.trim());
        }}
      >
        <input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="ابحث بالوصف: شخص يحمل صندوقاً عند البوابة"
          className={`${select} flex-1 min-w-[220px]`}
        />
        <select value={site} onChange={(e) => setSite(e.target.value)} className={select}>
          <option value="">كل المواقع</option>
          {(sites.data?.items ?? []).map((s) => (
            <option key={s.id} value={s.id}>{s.name}</option>
          ))}
        </select>
        <select value={lbl} onChange={(e) => setLbl(e.target.value)} className={select}>
          <option value="">كل التصنيفات</option>
          {Object.entries(LABELS).map(([k, v]) => (
            <option key={k} value={k}>{v}</option>
          ))}
        </select>
        <button className="rounded-lg bg-brand-600 hover:bg-brand-700 text-white px-4 py-2 text-sm">بحث</button>
        {submitted && (
          <button type="button" onClick={() => { setQuery(""); setSubmitted(""); }} className="text-sm muted underline">
            مسح البحث
          </button>
        )}
      </form>
      {events.error ? (
        <ErrorBox error={events.error} />
      ) : events.loading && !events.data ? (
        <Loading />
      ) : (
        <ul className="card px-4">
          {(events.data?.items ?? []).map((ev) => (
            <EventRow key={ev.device_id + ev.edge_id} ev={ev} siteName={siteName[ev.site_id]} />
          ))}
          {(events.data?.items ?? []).length === 0 && <li className="muted py-8 text-center">لا توجد نتائج</li>}
        </ul>
      )}
    </div>
  );
}
