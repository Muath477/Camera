"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import SiteMap from "@/components/SiteMap";
import { ErrorBox, EventRow, Loading, Stat } from "@/components/ui";
import { CATEGORIES, type Camera, type CameraEvent, type Site } from "@/lib/api";
import { useApi, useEventStream } from "@/lib/hooks";

export default function Dashboard() {
  const router = useRouter();
  const sites = useApi<{ items: Site[] | null }>("/v1/sites");
  const cameras = useApi<{ items: Camera[] | null }>("/v1/cameras");
  const recent = useApi<{ items: CameraEvent[] | null }>("/v1/events?limit=20");
  const [live, setLive] = useState<CameraEvent[]>([]);

  useEffect(() => {
    if (recent.data) setLive(recent.data.items ?? []);
  }, [recent.data]);

  useEventStream(
    useCallback((ev: CameraEvent) => {
      setLive((prev) => [ev, ...prev.filter((p) => !(p.device_id === ev.device_id && p.edge_id === ev.edge_id))].slice(0, 20));
    }, []),
  );

  const siteList = sites.data?.items ?? [];
  const camList = cameras.data?.items ?? [];
  const siteName = useMemo(() => Object.fromEntries(siteList.map((s) => [s.id, s.name])), [siteList]);
  const online = camList.filter((c) => c.status === "online").length;
  const critical24h = live.filter((e) => e.severity === "critical" && Date.now() - Date.parse(e.time) < 864e5).length;
  const selectSite = useCallback((id: string) => router.push(`/sites/${id}`), [router]);

  if (sites.error) return <ErrorBox error={sites.error} />;
  if (sites.loading && !sites.data) return <Loading />;

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">نظرة عامة</h1>
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <Stat title="المواقع" value={siteList.length} />
        <Stat
          title="الكاميرات المتصلة"
          value={`${online} / ${camList.length}`}
          tone={online < camList.length ? "warn" : "ok"}
        />
        <Stat title="أحداث حرجة (24 ساعة)" value={critical24h} tone={critical24h > 0 ? "bad" : "ok"} />
        <Stat title="آخر حدث" value={live[0] ? new Date(live[0].time).toLocaleTimeString("ar-SA-u-nu-latn", { timeZone: "Asia/Riyadh", hour: "2-digit", minute: "2-digit" }) : "—"} />
      </div>

      <div className="grid lg:grid-cols-3 gap-4">
        <div className="lg:col-span-2 card p-2">
          <SiteMap sites={siteList} cameras={camList} onSelectSite={selectSite} />
        </div>
        <div className="card p-4">
          <div className="flex items-center justify-between mb-2">
            <h2 className="font-semibold">الأحداث المباشرة</h2>
            <Link href="/events" className="text-sm text-brand-600">الكل</Link>
          </div>
          {live.length === 0 ? (
            <p className="muted text-sm py-6 text-center">لا توجد أحداث بعد</p>
          ) : (
            <ul className="max-h-[380px] overflow-y-auto">
              {live.map((ev) => (
                <EventRow key={ev.device_id + ev.edge_id} ev={ev} siteName={siteName[ev.site_id]} />
              ))}
            </ul>
          )}
        </div>
      </div>

      <div className="card overflow-x-auto">
        <table className="w-full text-sm whitespace-nowrap">
          <thead className="muted text-right">
            <tr className="border-b border-[var(--border)]">
              <th className="p-3 font-medium">الموقع</th>
              <th className="p-3 font-medium">الفئة</th>
              <th className="p-3 font-medium">الكاميرات</th>
              <th className="p-3 font-medium">مدة الحفظ المطلوبة</th>
            </tr>
          </thead>
          <tbody>
            {siteList.map((s) => (
              <tr key={s.id} className="border-b border-[var(--border)] last:border-0 hover:bg-black/[.02]">
                <td className="p-3">
                  <Link href={`/sites/${s.id}`} className="font-medium hover:text-brand-600">{s.name}</Link>
                  <div className="muted text-xs">{s.address}</div>
                </td>
                <td className="p-3">{CATEGORIES[s.facility_category] ?? s.facility_category}</td>
                <td className="p-3">
                  <span className={s.cameras_online < s.cameras ? "text-amber-600 font-medium" : ""}>
                    {s.cameras_online} / {s.cameras}
                  </span>
                </td>
                <td className="p-3">{s.retention_days_required} يوماً</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
