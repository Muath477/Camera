"use client";

import { AnimatePresence, motion } from "motion/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { use, useCallback, useMemo, useState } from "react";
import { ArrowLeft, Check, Cpu, ShieldCheck, Video } from "lucide-react";
import { CameraTile } from "@/components/CameraTile";
import { EventList } from "@/components/EventFeed";
import { useLive } from "@/components/Live";
import LiveView from "@/components/LiveView";
import SiteMap from "@/components/SiteMap";
import { Badge, Button, ErrorBox, Loading, PageHeader, Ring, Skeleton } from "@/components/ui";
import { CATEGORIES, type Camera, type CameraEvent, type ComplianceReport, type Device, type Site } from "@/lib/api";
import { fmtRelative } from "@/lib/format";
import { useApi } from "@/lib/hooks";

export default function SitePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const router = useRouter();
  const site = useApi<Site>(`/v1/sites/${id}`);
  const cameras = useApi<{ items: Camera[] | null }>(`/v1/cameras?site_id=${id}`);
  const devices = useApi<{ items: Device[] | null }>("/v1/devices");
  const report = useApi<ComplianceReport>(`/v1/sites/${id}/compliance`);
  const events = useApi<{ items: CameraEvent[] | null }>(`/v1/events?site_id=${id}&limit=30`);
  const live = useLive();
  const [liveCam, setLiveCam] = useState<Camera | null>(null);

  const merged = useMemo(() => {
    const seen = new Set<string>();
    return [...live.events.filter((e) => e.site_id === id), ...(events.data?.items ?? [])].filter((e) => {
      const k = e.device_id + e.edge_id;
      if (seen.has(k)) return false;
      seen.add(k);
      return true;
    });
  }, [live.events, events.data, id]);

  const selectCamera = useCallback((cid: string) => router.push(`/cameras/${cid}`), [router]);

  if (site.error) return <ErrorBox error={site.error} retry={site.reload} />;
  if (!site.data) return <Loading />;
  const s = site.data;
  const cams = cameras.data?.items ?? [];
  const siteDevices = (devices.data?.items ?? []).filter((d) => d.site_id === id);
  const failing = report.data?.checks.filter((c) => c.status === "fail") ?? [];

  return (
    <div className="space-y-6">
      <PageHeader
        crumbs={[{ href: "/", label: "الرئيسية" }]}
        title={s.name}
        subtitle={`${CATEGORIES[s.facility_category] ?? s.facility_category} · ${s.address ?? ""} · الحفظ المطلوب ${s.retention_days_required} يوماً`}
        actions={
          <Link href={`/compliance#${s.id}`}>
            <Button variant="secondary" icon={ShieldCheck}>تقرير الامتثال</Button>
          </Link>
        }
      />

      <div className="grid lg:grid-cols-3 gap-4">
        <motion.div initial={{ opacity: 0, scale: 0.985 }} animate={{ opacity: 1, scale: 1 }} className="lg:col-span-2 card p-1.5">
          <SiteMap sites={[s]} cameras={cams} className="h-[360px] w-full rounded-lg overflow-hidden" onSelectCamera={selectCamera} />
        </motion.div>

        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.1 }} className="card p-4 flex flex-col">
          <div className="flex items-center justify-between mb-3">
            <h2 className="font-semibold">جاهزية الامتثال</h2>
            {report.data && <Ring value={report.data.score} size={72} stroke={7} />}
          </div>
          {report.data ? (
            <>
              <ul className="space-y-2 flex-1">
                {report.data.checks.map((c, i) => (
                  <motion.li key={c.type} initial={{ opacity: 0, x: 8 }} animate={{ opacity: 1, x: 0 }} transition={{ delay: 0.15 + i * 0.05 }} className="text-sm">
                    <div className="flex items-center justify-between gap-2">
                      <span className="font-medium">{c.title_ar}</span>
                      <Badge status={c.status} />
                    </div>
                    <div className="muted text-xs mt-0.5">{c.detail_ar}</div>
                  </motion.li>
                ))}
              </ul>
              {failing.length > 0 && (
                <div className="mt-3 rounded-lg bg-rose-50 dark:bg-rose-900/20 text-rose-700 dark:text-rose-300 text-xs p-3">
                  {failing.length} بند غير مستوفى. عالجه قبل التفتيش لتجنّب الغرامة.
                </div>
              )}
            </>
          ) : (
            <div className="space-y-3">
              {[0, 1, 2, 3].map((i) => (
                <Skeleton key={i} className="h-8" />
              ))}
            </div>
          )}
        </motion.div>
      </div>

      <section>
        <div className="flex items-center justify-between mb-3">
          <h2 className="font-semibold flex items-center gap-2"><Video className="size-4" /> الكاميرات ({cams.length})</h2>
          <Link href="/cameras" className="text-sm text-brand-600 flex items-center gap-1 hover:underline">
            كل الكاميرات <ArrowLeft className="size-3.5" />
          </Link>
        </div>
        {cameras.loading && !cameras.data ? (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
            {[0, 1, 2, 3].map((i) => (
              <Skeleton key={i} className="aspect-video" />
            ))}
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
            {cams.map((c, i) => (
              <motion.div key={c.id} initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.05 * i }}>
                <CameraTile camera={c} onLive={() => setLiveCam(c)} />
              </motion.div>
            ))}
          </div>
        )}
      </section>

      <div className="grid lg:grid-cols-3 gap-4">
        <section className="card p-4 lg:col-span-2">
          <h2 className="font-semibold mb-2">آخر الأحداث</h2>
          {events.loading && !events.data ? <Loading /> : <EventList events={merged} compact ackable />}
        </section>
        <section className="card p-4">
          <h2 className="font-semibold mb-3 flex items-center gap-2"><Cpu className="size-4" /> أجهزة التسجيل</h2>
          {siteDevices.length === 0 ? (
            <p className="muted text-sm">لا يوجد جهاز مرتبط. <Link href="/devices/new" className="text-brand-600 underline">أضف جهازاً</Link></p>
          ) : (
            <ul className="space-y-3">
              {siteDevices.map((d) => (
                <li key={d.id} className="text-sm">
                  <div className="flex items-center justify-between">
                    <span className="font-mono text-xs" dir="ltr">{d.serial}</span>
                    <Badge status={d.status} dot={d.status !== "online"} />
                  </div>
                  <div className="muted text-xs mt-1 flex justify-between">
                    <span>{d.retention_days != null ? `يكفي لمدة ${Math.floor(d.retention_days)} يوماً` : "—"}</span>
                    <span>{d.last_seen_at ? fmtRelative(d.last_seen_at) : ""}</span>
                  </div>
                  {d.disk_total && (
                    <div className="mt-2 h-1.5 rounded-full surface-2 overflow-hidden">
                      <motion.div className="h-full bg-brand-500 rounded-full" initial={{ width: 0 }} animate={{ width: `${((d.disk_used ?? 0) / d.disk_total) * 100}%` }} transition={{ duration: 0.8 }} />
                    </div>
                  )}
                </li>
              ))}
            </ul>
          )}
          {report.data && report.data.score === 100 && (
            <div className="mt-4 flex items-center gap-2 text-brand-700 text-sm"><Check className="size-4" /> الموقع جاهز للتفتيش</div>
          )}
        </section>
      </div>

      <AnimatePresence>{liveCam && <LiveView camera={liveCam} onClose={() => setLiveCam(null)} />}</AnimatePresence>
    </div>
  );
}
