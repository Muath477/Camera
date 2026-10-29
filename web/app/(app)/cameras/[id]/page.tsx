"use client";

import { AnimatePresence, motion } from "motion/react";
import { use, useEffect, useMemo, useState } from "react";
import { Camera as CameraIcon, ChevronRight, Compass, Download, MapPin, Play, Save, Shield } from "lucide-react";
import { Thumb } from "@/components/CameraTile";
import { EventList } from "@/components/EventFeed";
import LiveView from "@/components/LiveView";
import SiteMap from "@/components/SiteMap";
import Timeline from "@/components/Timeline";
import { useToast } from "@/components/Toast";
import { Badge, Button, ErrorBox, Loading, PageHeader } from "@/components/ui";
import { api, type Camera, type CameraEvent, type Site } from "@/lib/api";
import { fmtDay, fmtTime } from "@/lib/format";
import { useApi } from "@/lib/hooks";

const ZONES: Record<string, string> = {
  entrance: "المداخل والمخارج",
  parking: "المواقف",
  cashier: "منطقة المحاسبة",
  atm: "أجهزة الصرف",
  warehouse: "المستودع",
  corridor: "الممرات",
  lobby: "الاستقبال",
  perimeter: "السور الخارجي",
  fuel_pumps: "مضخات الوقود",
  vault: "الخزنة",
  elevator: "المصاعد",
};

function startOfRiyadhDay(d: Date): Date {
  const parts = new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Riyadh", year: "numeric", month: "2-digit", day: "2-digit" }).formatToParts(d);
  const get = (t: string) => parts.find((p) => p.type === t)!.value;
  return new Date(`${get("year")}-${get("month")}-${get("day")}T00:00:00+03:00`);
}

export default function CameraPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const cam = useApi<Camera>(`/v1/cameras/${id}`);
  const toast = useToast();
  const [day, setDay] = useState(() => startOfRiyadhDay(new Date()));
  const [at, setAt] = useState<Date | null>(null);
  const [live, setLive] = useState(false);
  const site = useApi<Site>(cam.data ? `/v1/sites/${cam.data.site_id}` : null);
  const from = day.toISOString();
  const to = new Date(day.getTime() + 864e5).toISOString();
  const events = useApi<{ items: CameraEvent[] | null }>(cam.data ? `/v1/events?camera_id=${id}&from=${from}&to=${to}&limit=500` : null);

  // Placement editor state
  const [edit, setEdit] = useState(false);
  const [place, setPlace] = useState<{ lat?: number; lng?: number; heading: number; fov: number; zone: string; name: string }>({ heading: 0, fov: 90, zone: "", name: "" });
  const [saving, setSaving] = useState(false);
  useEffect(() => {
    if (cam.data) setPlace({ lat: cam.data.lat, lng: cam.data.lng, heading: cam.data.heading_deg ?? 0, fov: cam.data.fov_deg ?? 90, zone: cam.data.article2_zone ?? "", name: cam.data.display_name });
  }, [cam.data]);

  const preview = useMemo<Camera | null>(() => (cam.data ? { ...cam.data, lat: place.lat, lng: place.lng, heading_deg: place.heading, fov_deg: place.fov } : null), [cam.data, place]);

  async function save() {
    setSaving(true);
    try {
      await api(`/v1/cameras/${id}/placement`, {
        method: "PATCH",
        body: JSON.stringify({ lat: place.lat, lng: place.lng, heading_deg: place.heading, fov_deg: place.fov, article2_zone: place.zone || null, display_name: place.name }),
      });
      toast.push({ tone: "ok", title: "تم حفظ موضع الكاميرا" });
      setEdit(false);
      cam.reload();
    } catch (e) {
      toast.push({ tone: "critical", title: "تعذّر الحفظ", body: e instanceof Error ? e.message : String(e) });
    } finally {
      setSaving(false);
    }
  }

  async function exportClip() {
    if (!at) return;
    const reason = prompt("سبب التصدير (يُسجَّل في سجل التدقيق):");
    if (!reason) return;
    try {
      await api(`/v1/cameras/${id}/exports`, {
        method: "POST",
        body: JSON.stringify({ start: new Date(at.getTime() - 60e3).toISOString(), end: new Date(at.getTime() + 60e3).toISOString(), reason }),
      });
      toast.push({ tone: "ok", title: "بدأ التصدير", body: "سيُرفع المقطع من الجهاز خلال دقائق." });
    } catch (e) {
      toast.push({ tone: "critical", title: "تعذّر التصدير", body: e instanceof Error ? e.message : String(e) });
    }
  }

  if (cam.error) return <ErrorBox error={cam.error} retry={cam.reload} />;
  if (!cam.data || !preview) return <Loading />;
  const c = cam.data;
  const today = startOfRiyadhDay(new Date()).getTime() === day.getTime();

  return (
    <div className="space-y-6">
      <PageHeader
        crumbs={[{ href: "/cameras", label: "الكاميرات" }, ...(site.data ? [{ href: `/sites/${site.data.id}`, label: site.data.name }] : [])]}
        title={
          <span className="flex items-center gap-3">
            {c.display_name} <Badge status={c.status} dot={c.status !== "online"} />
          </span>
        }
        subtitle={[c.vendor, c.model, c.article2_zone && ZONES[c.article2_zone]].filter(Boolean).join(" · ") || "كاميرا"}
        actions={
          <>
            <Button icon={Play} onClick={() => setLive(true)}>بث حي</Button>
            <Button variant="secondary" icon={MapPin} onClick={() => setEdit((v) => !v)}>
              {edit ? "إلغاء التعديل" : "تعديل الموضع"}
            </Button>
          </>
        }
      />

      <div className="grid lg:grid-cols-3 gap-4">
        <div className="lg:col-span-2 space-y-4">
          <button onClick={() => setLive(true)} className="card overflow-hidden block w-full aspect-video bg-slate-900 relative group">
            <Thumb cameraId={c.id} every={10_000} />
            <span className="absolute inset-0 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity bg-black/30">
              <span className="size-14 rounded-full bg-white/90 text-brand-700 flex items-center justify-center shadow-lg">
                <Play className="size-6" fill="currentColor" />
              </span>
            </span>
            <span className="absolute bottom-3 right-3 text-xs text-white/90 bg-black/50 rounded px-2 py-1">آخر صورة · تتجدد كل 10 ثوان</span>
          </button>

          <div className="card p-4">
            <div className="flex items-center justify-between gap-2 mb-3 flex-wrap">
              <h2 className="font-semibold">التسجيلات</h2>
              <div className="flex items-center gap-1">
                <button onClick={() => setDay(new Date(day.getTime() - 864e5))} className="size-8 rounded-lg hover:bg-[var(--surface-2)] flex items-center justify-center" aria-label="اليوم السابق">
                  <ChevronRight className="size-4" />
                </button>
                <span className="text-sm font-medium min-w-[9rem] text-center">{fmtDay(day)}</span>
                <button onClick={() => setDay(new Date(day.getTime() + 864e5))} disabled={today} className="size-8 rounded-lg hover:bg-[var(--surface-2)] flex items-center justify-center disabled:opacity-30 rotate-180" aria-label="اليوم التالي">
                  <ChevronRight className="size-4" />
                </button>
              </div>
            </div>
            <Timeline day={day} events={events.data?.items ?? []} value={at} onChange={setAt} />
            <div className="flex items-center justify-between mt-3 text-sm">
              <div className="muted">
                {at ? (
                  <>
                    التشغيل من <span className="font-medium text-[var(--text)]">{fmtTime(at)}</span> · يُخدم من قرص الجهاز عبر النفق
                  </>
                ) : (
                  "اختر وقتاً على الشريط للتشغيل"
                )}
              </div>
              <Button variant="secondary" size="sm" icon={Download} disabled={!at} onClick={exportClip}>
                تصدير دقيقتين
              </Button>
            </div>
          </div>

          <div className="card p-4">
            <h2 className="font-semibold mb-2">أحداث اليوم ({events.data?.items?.length ?? 0})</h2>
            {events.loading && !events.data ? <Loading /> : <EventList events={events.data?.items ?? []} compact ackable emptyHint="لم تُرصد أحداث على هذه الكاميرا في هذا اليوم." />}
          </div>
        </div>

        <div className="space-y-4">
          <div className="card p-1.5 relative">
            {site.data ? (
              <SiteMap
                sites={[site.data]}
                cameras={[preview]}
                className="h-72 w-full rounded-lg overflow-hidden"
                editable={edit ? { cameraId: c.id, onMove: (lng, lat) => setPlace((p) => ({ ...p, lng, lat })) } : undefined}
              />
            ) : (
              <div className="h-72 skeleton" />
            )}
            {edit && !place.lat && site.data?.lat && (
              <button
                onClick={() => setPlace((p) => ({ ...p, lat: site.data!.lat, lng: site.data!.lng }))}
                className="absolute bottom-4 inset-x-4 card px-3 py-2 text-xs text-center hover:border-brand-400"
              >
                ضع الكاميرا عند موقع الفرع ثم اسحبها
              </button>
            )}
          </div>

          <AnimatePresence initial={false}>
            {edit ? (
              <motion.div key="edit" initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: 8 }} className="card p-4 space-y-4">
                <h3 className="font-semibold flex items-center gap-2"><Compass className="size-4" /> الموضع والاتجاه</h3>
                <label className="block text-sm">
                  <span className="muted text-xs">الاسم</span>
                  <input value={place.name} onChange={(e) => setPlace((p) => ({ ...p, name: e.target.value }))} className="input w-full mt-1" />
                </label>
                <label className="block text-sm">
                  <div className="flex justify-between"><span className="muted text-xs">الاتجاه</span><span className="text-xs">{Math.round(place.heading)}°</span></div>
                  <input type="range" min={0} max={359} value={place.heading} onChange={(e) => setPlace((p) => ({ ...p, heading: Number(e.target.value) }))} className="w-full accent-brand-600" dir="ltr" />
                </label>
                <label className="block text-sm">
                  <div className="flex justify-between"><span className="muted text-xs">زاوية الرؤية</span><span className="text-xs">{place.fov}°</span></div>
                  <input type="range" min={30} max={180} value={place.fov} onChange={(e) => setPlace((p) => ({ ...p, fov: Number(e.target.value) }))} className="w-full accent-brand-600" dir="ltr" />
                </label>
                <label className="block text-sm">
                  <span className="muted text-xs">المنطقة حسب اللائحة</span>
                  <select value={place.zone} onChange={(e) => setPlace((p) => ({ ...p, zone: e.target.value }))} className="input w-full mt-1">
                    <option value="">بدون</option>
                    {Object.entries(ZONES).map(([k, v]) => (
                      <option key={k} value={k}>{v}</option>
                    ))}
                  </select>
                </label>
                <div className="flex gap-2">
                  <Button icon={Save} loading={saving} onClick={save} className="flex-1">حفظ</Button>
                  <Button variant="secondary" onClick={() => setEdit(false)}>إلغاء</Button>
                </div>
              </motion.div>
            ) : (
              <motion.div key="info" initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: 8 }} className="card p-4 text-sm space-y-3">
                <h3 className="font-semibold flex items-center gap-2"><CameraIcon className="size-4" /> البيانات</h3>
                <dl className="grid grid-cols-2 gap-y-2 gap-x-3">
                  <dt className="muted">المعرّف على الجهاز</dt><dd dir="ltr" className="text-left font-mono text-xs">{c.name}</dd>
                  <dt className="muted">العلامة</dt><dd>{c.vendor || "—"}</dd>
                  <dt className="muted">الطراز</dt><dd>{c.model || "—"}</dd>
                  <dt className="muted">PTZ</dt><dd>{c.ptz ? "نعم" : "لا"}</dd>
                  <dt className="muted">الاتجاه</dt><dd>{c.heading_deg != null ? `${Math.round(c.heading_deg)}°` : "غير محدد"}</dd>
                  <dt className="muted">المنطقة</dt><dd>{c.article2_zone ? ZONES[c.article2_zone] : "—"}</dd>
                </dl>
                <div className="flex items-start gap-2 rounded-lg surface-2 p-3 text-xs muted">
                  <Shield className="size-4 shrink-0 text-brand-600" />
                  بيانات اعتماد هذه الكاميرا مشفّرة على جهاز الموقع ولا تُخزَّن في السحابة.
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      </div>

      <AnimatePresence>{live && <LiveView camera={c} onClose={() => setLive(false)} />}</AnimatePresence>
    </div>
  );
}
