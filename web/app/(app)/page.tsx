"use client";

import { motion } from "motion/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import { AlertTriangle, ArrowLeft, BellRing, Building2, Clock, Plus, ShieldCheck, Video, VideoOff } from "lucide-react";
import { EventList } from "@/components/EventFeed";
import { useLive } from "@/components/Live";
import SiteMap from "@/components/SiteMap";
import { Badge, Button, Counter, EmptyState, ErrorBox, Ring, Skeleton, Stat, fadeUp, stagger } from "@/components/ui";
import Mascot from "@/components/Mascot";
import { CATEGORIES, api, type Camera, type CameraEvent, type ComplianceReport, type Me, type Site } from "@/lib/api";
import { alertsAR, fmtRelative, offlineCamsAR } from "@/lib/format";
import { useApi } from "@/lib/hooks";

export default function Dashboard() {
  const router = useRouter();
  const sites = useApi<{ items: Site[] | null }>("/v1/sites");
  const cameras = useApi<{ items: Camera[] | null }>("/v1/cameras");
  const recent = useApi<{ items: CameraEvent[] | null }>("/v1/events?limit=20");
  const me = useApi<Me>("/v1/me");
  const live = useLive();
  const [feed, setFeed] = useState<CameraEvent[]>([]);
  const [reports, setReports] = useState<ComplianceReport[] | null>(null);

  // One readiness score for the whole account, from the same per-site checks
  // the compliance page shows.
  useEffect(() => {
    const list = sites.data?.items ?? [];
    if (list.length === 0) return;
    let cancelled = false;
    Promise.all(list.map((s) => api<ComplianceReport>(`/v1/sites/${s.id}/compliance`).catch(() => null))).then((r) => {
      if (!cancelled) setReports(r.filter((x): x is ComplianceReport => x !== null));
    });
    return () => {
      cancelled = true;
    };
  }, [sites.data]);

  useEffect(() => {
    if (recent.data) setFeed(recent.data.items ?? []);
  }, [recent.data]);

  // Merge pushed events on top of the loaded ones.
  const merged = useMemo(() => {
    const seen = new Set<string>();
    const out: CameraEvent[] = [];
    for (const ev of [...live.events, ...feed]) {
      const k = ev.device_id + ev.edge_id;
      if (seen.has(k)) continue;
      seen.add(k);
      out.push(ev);
    }
    return out.slice(0, 20);
  }, [live.events, feed]);

  const siteList = sites.data?.items ?? [];
  const camList = cameras.data?.items ?? [];
  const siteName = useMemo(() => Object.fromEntries(siteList.map((s) => [s.id, s.name])), [siteList]);
  const online = camList.filter((c) => c.status === "online").length;
  const dayAgo = Date.now() - 864e5;
  const critical24h = merged.filter((e) => e.severity === "critical" && Date.parse(e.time) > dayAgo).length;
  const criticalSites = useMemo(
    () => new Set(merged.filter((e) => e.severity === "critical" && Date.parse(e.time) > Date.now() - 36e5).map((e) => e.site_id)),
    [merged],
  );
  const unacked = merged.filter((e) => e.severity === "critical" && !e.acked_at).length;
  const offline = camList.length - online;
  const avgScore = reports && reports.length ? Math.round(reports.reduce((a, r) => a + r.score, 0) / reports.length) : null;
  // Newest unacknowledged critical event per camera in the last hour, for
  // the map bubbles.
  const alerts = useMemo(() => {
    const cutoff = Date.now() - 36e5;
    const out: Record<string, CameraEvent> = {};
    for (const e of merged) {
      if (e.severity !== "critical" || e.acked_at || !e.camera_id || Date.parse(e.time) < cutoff) continue;
      if (!out[e.camera_id] || Date.parse(e.time) > Date.parse(out[e.camera_id].time)) out[e.camera_id] = e;
    }
    return out;
  }, [merged]);
  const selectSite = useCallback((id: string) => router.push(`/sites/${id}`), [router]);
  const selectCamera = useCallback((id: string) => router.push(`/cameras/${id}`), [router]);

  if (sites.error) return <ErrorBox error={sites.error} retry={sites.reload} />;

  const loading = sites.loading && !sites.data;

  const hour = new Date().getHours();
  const greeting = hour < 12 ? "صباح الخير" : "مساء الخير";
  const firstName = me.data?.name.split(" ")[0];
  const needsYou = unacked > 0 || offline > 0;
  const mood = unacked > 0 ? "alert" : "happy";
  const headline = unacked > 0 ? `عندك ${alertsAR(unacked)} بانتظار إقرارك` : offline > 0 ? `كل شيء تقريباً تمام، عدا ${offlineCamsAR(offline)}` : "كل شيء تمام، الكاميرات كلها تسجّل";

  return (
    <div className="space-y-8">
      {/* Greeting: the mascot's mood and one sentence say how things are. */}
      <motion.section
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ type: "spring", stiffness: 300, damping: 28 }}
        className="card relative overflow-hidden p-5 md:p-7"
        aria-labelledby="dash-title"
      >
        <div className="flex flex-wrap md:flex-nowrap items-center gap-4 md:gap-5">
          <Mascot size={92} mood={loading ? "happy" : mood} follow className="shrink-0 -rotate-3 size-16 md:size-[92px]" />
          <div className="min-w-0 flex-1">
            <p className="muted font-medium">
              {greeting}
              {firstName ? `، ${firstName}` : ""}
            </p>
            <h1 id="dash-title" className="text-3xl md:text-4xl font-extrabold mt-1">
              {loading ? "نجهّز لك الصورة…" : <span className={needsYou ? "mark" : ""}>{headline}</span>}
            </h1>
            <p className="muted mt-2">كل الفروع في شاشة واحدة، والأحداث تصلك فور رصدها.</p>
          </div>
          <Link href="/devices/new" className="hidden sm:block shrink-0 self-start md:self-center">
            <Button icon={Plus} variant="secondary">أضف جهاز تسجيل</Button>
          </Link>
        </div>

        {!loading && siteList.length > 0 && (
          <div className="mt-6 grid sm:grid-cols-3 gap-3">
            <Link href="/compliance" className="group panel flex items-center gap-3 p-3 hover:bg-sun-100 dark:hover:bg-sun-400/10 transition-colors">
              {avgScore === null ? <Skeleton className="size-12 rounded-full" /> : <Ring value={avgScore} size={48} stroke={6} />}
              <div className="min-w-0">
                <div className="font-bold text-sm flex items-center gap-1.5"><ShieldCheck className="size-4 wiggle" /> جاهزية الامتثال</div>
                <div className="muted text-xs truncate">{avgScore === null ? "نفحص الفروع الآن…" : avgScore >= 80 ? "جاهز للتفتيش في معظم الفروع" : "بنود تحتاج تصحيحاً قبل التفتيش"}</div>
              </div>
            </Link>
            <Link href="/cameras?status=offline" className="group panel flex items-center gap-3 p-3 hover:bg-aqua-100 dark:hover:bg-aqua-400/10 transition-colors">
              <span className={`size-12 rounded-full border-2 border-[var(--line)] text-ink-900 flex items-center justify-center shrink-0 ${offline > 0 ? "bg-sun-400" : "bg-brand-400"}`}>
                <VideoOff className="size-5 wiggle" />
              </span>
              <div className="min-w-0">
                <div className="font-bold text-sm">كاميرات منقطعة الآن: <Counter value={offline} /></div>
                <div className="muted text-xs truncate">{offline > 0 ? "بلّغ عن العطل خلال المدة النظامية" : "كل الكاميرات تسجّل"}</div>
              </div>
            </Link>
            <Link href="/events?severity=critical" className="group panel flex items-center gap-3 p-3 hover:bg-pop-100 dark:hover:bg-pop-400/10 transition-colors">
              <span className={`size-12 rounded-full border-2 border-[var(--line)] text-ink-900 flex items-center justify-center shrink-0 ${unacked > 0 ? "bg-pop-400" : "bg-brand-400"}`}>
                <BellRing className="size-5 wiggle" />
              </span>
              <div className="min-w-0">
                <div className="font-bold text-sm">تنبيهات بانتظار الإقرار: <Counter value={unacked} /></div>
                <div className="muted text-xs truncate">{unacked > 0 ? "أقرّ بالاستلام ليُسجَّل في سجل التدقيق" : "لا تنبيهات معلّقة"}</div>
              </div>
            </Link>
          </div>
        )}
      </motion.section>

      {loading ? (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
          {[0, 1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-24" />
          ))}
        </div>
      ) : (
        <motion.div variants={stagger} initial="hidden" animate="show" className="grid grid-cols-2 lg:grid-cols-4 gap-3">
          <motion.div variants={fadeUp}>
            <Stat title="المواقع" value={<Counter value={siteList.length} />} icon={Building2} href="/compliance" />
          </motion.div>
          <motion.div variants={fadeUp}>
            <Stat
              title="الكاميرات المتصلة"
              value={
                <>
                  <Counter value={online} /> <span className="muted text-lg font-medium">/ {camList.length}</span>
                </>
              }
              icon={Video}
              tone={online < camList.length ? "warn" : "ok"}
              hint={online < camList.length ? `${camList.length - online} كاميرا منقطعة` : "كل الكاميرات تعمل"}
              href="/cameras"
            />
          </motion.div>
          <motion.div variants={fadeUp}>
            <Stat title="أحداث حرجة (24 ساعة)" value={<Counter value={critical24h} />} icon={AlertTriangle} tone={critical24h > 0 ? "bad" : "ok"} href="/events" />
          </motion.div>
          <motion.div variants={fadeUp}>
            <Stat title="آخر حدث" value={merged[0] ? fmtRelative(merged[0].time) : "—"} icon={Clock} hint={merged[0]?.camera_name ?? undefined} />
          </motion.div>
        </motion.div>
      )}

      <div className="grid lg:grid-cols-3 gap-4">
        <motion.div initial={{ opacity: 0, scale: 0.985 }} animate={{ opacity: 1, scale: 1 }} transition={{ delay: 0.15, duration: 0.4 }} className="lg:col-span-2 card p-1.5 relative">
          {loading ? (
            <Skeleton className="h-[420px]" />
          ) : siteList.length === 0 ? (
            <EmptyState
              icon={Building2}
              title="لا توجد مواقع بعد"
              hint="أضف أول موقع ثم اربط جهاز التسجيل به لتظهر الكاميرات على الخريطة."
              action={
                <Link href="/devices/new">
                  <Button icon={Plus}>ابدأ الإعداد</Button>
                </Link>
              }
            />
          ) : (
            <>
              <SiteMap sites={siteList} cameras={camList} critical={criticalSites} alerts={alerts} onSelectSite={selectSite} onSelectCamera={selectCamera} />
              <div className="absolute bottom-4 start-4 card px-3 py-2 text-xs font-medium flex flex-wrap items-center gap-3">
                <span className="flex items-center gap-1.5"><span className="map-marker ok !w-3 !h-3 !border" /> سليم</span>
                <span className="flex items-center gap-1.5"><span className="map-marker degraded !w-3 !h-3 !border" /> كاميرا منقطعة</span>
                <span className="flex items-center gap-1.5"><span className="inline-block w-3 h-3 rounded-sm border border-dashed border-ink-400 bg-ink-200/50" /> بلا تسجيل</span>
                <span className="flex items-center gap-1.5"><span className="pulse-dot bad" /> حدث حرج</span>
              </div>
            </>
          )}
        </motion.div>

        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.2, duration: 0.4 }} className="card p-4 flex flex-col">
          <div className="flex items-center justify-between mb-2">
            <h2 className="text-xl font-extrabold flex items-center gap-2">
              <span className={`pulse-dot ${live.connected ? "" : "bad"}`} /> يحدث الآن
            </h2>
            <Link href="/events" className="group min-h-11 px-2 text-sm font-semibold text-brand-700 dark:text-brand-300 flex items-center gap-1 hover:underline">
              كل الأحداث <ArrowLeft className="size-4 transition-transform group-hover:-translate-x-1" />
            </Link>
          </div>
          <div className="max-h-[400px] overflow-y-auto -mx-1 px-1">
            {recent.loading && !recent.data ? (
              <div className="space-y-3 py-2">
                {[0, 1, 2, 3, 4].map((i) => (
                  <div key={i} className="flex gap-3">
                    <Skeleton className="size-9 rounded-lg" />
                    <div className="flex-1 space-y-2">
                      <Skeleton className="h-3 w-2/3" />
                      <Skeleton className="h-2.5 w-1/2" />
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <EventList events={merged} siteNames={siteName} compact ackable />
            )}
          </div>
        </motion.div>
      </div>

      <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.3, duration: 0.4 }}>
        <div className="flex items-baseline justify-between mb-4">
          <h2 className="text-2xl font-extrabold">
            <span className="squiggle">فروعك</span>
          </h2>
          <span className="muted text-sm tabular-nums">{siteList.length} موقع</span>
        </div>
        <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-3">
          {siteList.map((s, i) => {
            const health = s.cameras === 0 ? "unknown" : s.cameras_online === s.cameras ? "online" : s.cameras_online === 0 ? "offline" : "warning";
            return (
              <motion.div key={s.id} initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.05 * i }}>
                <Link href={`/sites/${s.id}`} className="card card-hover group p-4 block h-full">
                  <div className="flex items-start justify-between gap-2">
                    <span className={`shrink-0 size-11 rounded-xl border-2 border-[var(--line)] text-ink-900 flex items-center justify-center ${i % 2 ? "rotate-6" : "-rotate-6"} ${["bg-aqua-400", "bg-sun-400", "bg-lilac-400", "bg-brand-400"][i % 4]}`}>
                      <Building2 className="size-5 wiggle" />
                    </span>
                    <div className="min-w-0 flex-1">
                      <div className="font-display text-lg font-bold truncate">{s.name}</div>
                      <div className="muted text-xs truncate">{CATEGORIES[s.facility_category] ?? s.facility_category} · {s.address}</div>
                    </div>
                    <Badge status={health} dot={criticalSites.has(s.id)}>
                      {health === "online" ? "سليم" : health === "warning" ? "كاميرا منقطعة" : health === "offline" ? "منقطع" : "بلا كاميرات"}
                    </Badge>
                  </div>
                  <div className="mt-4 flex items-end justify-between">
                    <div>
                      <div className="font-display text-3xl font-extrabold leading-none tabular-nums">
                        {s.cameras_online}
                        <span className="muted text-sm font-medium"> / {s.cameras}</span>
                      </div>
                      <div className="muted text-xs mt-1">كاميرا تعمل</div>
                    </div>
                    <div className="text-end">
                      <div className="text-sm font-bold">{s.retention_days_required} يوماً</div>
                      <div className="muted text-xs">الحفظ المطلوب</div>
                    </div>
                  </div>
                  <div className="mt-4 h-3 rounded-full border-2 border-[var(--card-line)] surface-2 overflow-hidden" role="img" aria-label={`${s.cameras_online} من ${s.cameras} كاميرا تعمل`}>
                    <motion.div
                      className={`h-full rounded-full ${health === "online" ? "bg-brand-400" : health === "warning" ? "bg-sun-400" : "bg-pop-400"}`}
                      initial={{ width: 0 }}
                      animate={{ width: s.cameras ? `${(s.cameras_online / s.cameras) * 100}%` : "0%" }}
                      transition={{ duration: 0.8, delay: 0.2 + 0.05 * i, ease: [0.16, 1, 0.3, 1] }}
                    />
                  </div>
                </Link>
              </motion.div>
            );
          })}
        </div>
      </motion.div>
    </div>
  );
}
