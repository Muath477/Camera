"use client";

import { use, useEffect, useState } from "react";
import LiveView from "@/components/LiveView";
import SiteMap from "@/components/SiteMap";
import { Badge, ErrorBox, EventRow, Loading } from "@/components/ui";
import { CATEGORIES, thumbUrl, getToken, type Camera, type CameraEvent, type ComplianceReport, type Site } from "@/lib/api";
import { useApi } from "@/lib/hooks";

function Thumb({ camera }: { camera: Camera }) {
  const [src, setSrc] = useState<string | null>(null);
  // Thumbnails need the bearer token, so fetch them instead of using <img src>.
  useEffect(() => {
    let url: string | null = null;
    const load = () =>
      fetch(thumbUrl(camera.id), { headers: { Authorization: `Bearer ${getToken()}` } })
        .then((r) => (r.status === 200 ? r.blob() : null))
        .then((b) => {
          if (!b) return;
          if (url) URL.revokeObjectURL(url);
          url = URL.createObjectURL(b);
          setSrc(url);
        })
        .catch(() => {});
    load();
    const t = setInterval(load, 15_000);
    return () => {
      clearInterval(t);
      if (url) URL.revokeObjectURL(url);
    };
  }, [camera.id]);
  return src ? (
    // eslint-disable-next-line @next/next/no-img-element
    <img src={src} alt="" className="w-full h-full object-cover" />
  ) : (
    <div className="w-full h-full flex items-center justify-center text-slate-500 text-xs">لا توجد صورة حديثة</div>
  );
}

export default function SitePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const site = useApi<Site>(`/v1/sites/${id}`);
  const cameras = useApi<{ items: Camera[] | null }>(`/v1/cameras?site_id=${id}`);
  const report = useApi<ComplianceReport>(`/v1/sites/${id}/compliance`);
  const events = useApi<{ items: CameraEvent[] | null }>(`/v1/events?site_id=${id}&limit=30`);
  const [live, setLive] = useState<Camera | null>(null);

  if (site.error) return <ErrorBox error={site.error} />;
  if (!site.data) return <Loading />;
  const s = site.data;
  const cams = cameras.data?.items ?? [];

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">{s.name}</h1>
        <p className="muted text-sm">
          {CATEGORIES[s.facility_category] ?? s.facility_category} · {s.address} · الحفظ المطلوب {s.retention_days_required} يوماً
        </p>
      </div>

      <div className="grid lg:grid-cols-3 gap-4">
        <div className="lg:col-span-2 card p-2">
          <SiteMap sites={[s]} cameras={cams} className="h-[360px] w-full rounded-xl overflow-hidden" />
        </div>
        <div className="card p-4">
          <div className="flex items-center justify-between mb-3">
            <h2 className="font-semibold">جاهزية الامتثال</h2>
            {report.data && (
              <span className={`text-2xl font-bold ${report.data.score >= 80 ? "text-emerald-600" : report.data.score >= 50 ? "text-amber-600" : "text-red-600"}`}>
                {report.data.score}%
              </span>
            )}
          </div>
          {report.data ? (
            <ul className="space-y-2.5">
              {report.data.checks.map((c) => (
                <li key={c.type} className="text-sm">
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-medium">{c.title_ar}</span>
                    <Badge status={c.status} />
                  </div>
                  <div className="muted text-xs mt-0.5">{c.detail_ar}</div>
                </li>
              ))}
            </ul>
          ) : (
            <Loading />
          )}
        </div>
      </div>

      <section>
        <h2 className="font-semibold mb-3">الكاميرات ({cams.length})</h2>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
          {cams.map((c) => (
            <button key={c.id} onClick={() => setLive(c)} className="card overflow-hidden text-right hover:ring-2 hover:ring-brand-500">
              <div className="aspect-video bg-slate-900">
                <Thumb camera={c} />
              </div>
              <div className="p-3 flex items-center justify-between gap-2">
                <span className="font-medium truncate">{c.display_name}</span>
                <Badge status={c.status} />
              </div>
            </button>
          ))}
        </div>
      </section>

      <section className="card p-4">
        <h2 className="font-semibold mb-2">آخر الأحداث</h2>
        <ul>
          {(events.data?.items ?? []).map((ev) => (
            <EventRow key={ev.device_id + ev.edge_id} ev={ev} />
          ))}
        </ul>
      </section>

      {live && <LiveView camera={live} onClose={() => setLive(null)} />}
    </div>
  );
}
