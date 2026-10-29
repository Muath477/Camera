"use client";

import { AnimatePresence, motion } from "motion/react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Check, Cpu, Loader2, MapPin, Plus, QrCode, RadioTower, Video } from "lucide-react";
import { useToast } from "@/components/Toast";
import { Button, PageHeader } from "@/components/ui";
import { api, type Site } from "@/lib/api";
import { useApi } from "@/lib/hooks";

type Found = { host: string; vendor?: string; model?: string; name?: string; profile_id?: string };

const STEPS = [
  { id: 0, title: "ربط الجهاز", icon: QrCode },
  { id: 1, title: "اكتشاف الكاميرات", icon: RadioTower },
  { id: 2, title: "إضافة الكاميرات", icon: Video },
  { id: 3, title: "جاهز", icon: Check },
];

export default function NewDevicePage() {
  const router = useRouter();
  const toast = useToast();
  const sites = useApi<{ items: Site[] | null }>("/v1/sites");
  const [step, setStep] = useState(0);
  const [form, setForm] = useState({ serial: "", claim_code: "", site_id: "" });
  const [deviceId, setDeviceId] = useState("");
  const [busy, setBusy] = useState(false);
  const [found, setFound] = useState<Found[]>([]);
  const [creds, setCreds] = useState<Record<string, { name: string; username: string; password: string; zone: string; selected: boolean }>>({});
  const [added, setAdded] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);

  async function claim(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const r = await api<{ device_id: string }>("/v1/devices/claim", { method: "POST", body: JSON.stringify(form) });
      setDeviceId(r.device_id);
      setStep(1);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  async function discover() {
    setBusy(true);
    setError(null);
    try {
      const r = await api<{ cameras: Found[] | null }>(`/v1/devices/${deviceId}/discover`, { method: "POST" });
      const list = r.cameras ?? [];
      setFound(list);
      setCreds(
        Object.fromEntries(
          list.map((f, i) => [f.host, { name: f.name?.replace(/[^a-zA-Z0-9_-]/g, "") || `cam${i + 1}`, username: "admin", password: "", zone: "", selected: true }]),
        ),
      );
      setStep(2);
    } catch (err) {
      setError(err instanceof Error ? err.message : "الجهاز لم يجب. تأكد أنه موصول بالكهرباء والإنترنت.");
    } finally {
      setBusy(false);
    }
  }

  async function addAll() {
    setBusy(true);
    const done: string[] = [];
    for (const f of found) {
      const c = creds[f.host];
      if (!c?.selected) continue;
      try {
        await api("/v1/cameras", {
          method: "POST",
          body: JSON.stringify({
            device_id: deviceId,
            name: c.name,
            display_name: c.name,
            host: f.host,
            vendor: f.vendor,
            model: f.model,
            profile_id: f.profile_id,
            username: c.username,
            password: c.password,
            article2_zone: c.zone,
          }),
        });
        done.push(f.host);
        setAdded([...done]);
      } catch (err) {
        toast.push({ tone: "critical", title: `تعذّر إضافة ${c.name}`, body: err instanceof Error ? err.message : String(err) });
      }
    }
    setBusy(false);
    setStep(3);
  }

  return (
    <div className="max-w-3xl">
      <PageHeader crumbs={[{ href: "/devices", label: "الأجهزة" }]} title="إضافة جهاز تسجيل" subtitle="عشر دقائق من فتح الصندوق إلى أول كاميرا تعمل." />

      <ol className="flex items-center gap-2 mb-6 overflow-x-auto">
        {STEPS.map((s, i) => {
          const on = step === i;
          const done = step > i;
          return (
            <li key={s.id} className="flex items-center gap-2 shrink-0">
              <motion.span
                animate={{ scale: on ? 1.08 : 1 }}
                className={`size-9 rounded-full flex items-center justify-center border-2 transition-colors ${done ? "bg-brand-600 border-brand-600 text-white" : on ? "border-brand-600 text-brand-700" : "border-[var(--border)] muted"}`}
              >
                {done ? <Check className="size-4" /> : <s.icon className="size-4" />}
              </motion.span>
              <span className={`text-sm ${on ? "font-semibold" : "muted"}`}>{s.title}</span>
              {i < STEPS.length - 1 && <span className={`w-8 h-0.5 rounded ${done ? "bg-brand-600" : "bg-[var(--border)]"}`} />}
            </li>
          );
        })}
      </ol>

      <AnimatePresence mode="wait">
        {step === 0 && (
          <motion.form key="claim" initial={{ opacity: 0, x: -16 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, x: 16 }} onSubmit={claim} className="card p-5 space-y-4">
            <div className="flex gap-4 items-start">
              <span className="size-12 rounded-xl bg-brand-100 text-brand-700 dark:bg-brand-900/50 dark:text-brand-300 flex items-center justify-center shrink-0">
                <QrCode className="size-6" />
              </span>
              <div className="text-sm">
                <div className="font-semibold">امسح الرمز على ظهر الجهاز أو أدخل بياناته</div>
                <div className="muted">الرقم التسلسلي ورمز التفعيل مطبوعان على الملصق. يُستخدم الرمز مرة واحدة فقط.</div>
              </div>
            </div>
            <div className="grid sm:grid-cols-2 gap-3">
              <label className="text-sm">
                <span className="muted text-xs">الرقم التسلسلي</span>
                <input dir="ltr" required value={form.serial} onChange={(e) => setForm({ ...form, serial: e.target.value.trim() })} placeholder="CAM-STOCK-001" className="input w-full mt-1 font-mono" />
              </label>
              <label className="text-sm">
                <span className="muted text-xs">رمز التفعيل</span>
                <input dir="ltr" required value={form.claim_code} onChange={(e) => setForm({ ...form, claim_code: e.target.value.trim() })} placeholder="123456" className="input w-full mt-1 font-mono" />
              </label>
            </div>
            <label className="text-sm block">
              <span className="muted text-xs flex items-center gap-1"><MapPin className="size-3" /> الموقع الذي سيُركّب فيه</span>
              <select required value={form.site_id} onChange={(e) => setForm({ ...form, site_id: e.target.value })} className="input w-full mt-1">
                <option value="">اختر الموقع</option>
                {(sites.data?.items ?? []).map((s) => (
                  <option key={s.id} value={s.id}>{s.name}</option>
                ))}
              </select>
            </label>
            {error && <p className="text-sm text-bad-500">{error}</p>}
            <div className="flex justify-end">
              <Button type="submit" loading={busy} icon={Cpu}>ربط الجهاز</Button>
            </div>
          </motion.form>
        )}

        {step === 1 && (
          <motion.div key="discover" initial={{ opacity: 0, x: -16 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, x: 16 }} className="card p-5 space-y-4 text-center">
            <motion.div animate={busy ? { scale: [1, 1.08, 1] } : {}} transition={{ repeat: Infinity, duration: 1.4 }} className="mx-auto size-20 rounded-full bg-brand-100 dark:bg-brand-900/50 text-brand-700 dark:text-brand-300 flex items-center justify-center relative">
              {busy && <span className="absolute inset-0 rounded-full border-2 border-brand-400 animate-ping" />}
              <RadioTower className="size-9" />
            </motion.div>
            <div>
              <div className="font-semibold">تم ربط الجهاز</div>
              <p className="muted text-sm mt-1">وصّل الكاميرات بمنفذ الشبكة الثاني على الجهاز، ثم اضغط اكتشاف. سيمسح الجهاز شبكته المحلية ويعرض ما وجده مع العلامة والطراز.</p>
            </div>
            {error && <p className="text-sm text-bad-500">{error}</p>}
            <Button loading={busy} onClick={discover} icon={RadioTower}>اكتشاف الكاميرات</Button>
          </motion.div>
        )}

        {step === 2 && (
          <motion.div key="add" initial={{ opacity: 0, x: -16 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, x: 16 }} className="space-y-3">
            {found.length === 0 && (
              <div className="card p-5 text-center">
                <div className="font-semibold">لم تُكتشف كاميرات</div>
                <p className="muted text-sm mt-1">تأكد من توصيل الكاميرات بالمنفذ الصحيح. تقدر تضيفها يدوياً لاحقاً بمسار RTSP.</p>
                <Button variant="secondary" className="mt-3" onClick={discover} loading={busy}>إعادة المحاولة</Button>
              </div>
            )}
            {found.map((f, i) => {
              const c = creds[f.host];
              return (
                <motion.div key={f.host} initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.05 * i }} className={`card p-4 transition-opacity ${c.selected ? "" : "opacity-50"}`}>
                  <div className="flex items-center gap-3 mb-3">
                    <input type="checkbox" checked={c.selected} onChange={(e) => setCreds({ ...creds, [f.host]: { ...c, selected: e.target.checked } })} className="size-4 accent-brand-600" />
                    <div className="flex-1 min-w-0">
                      <div className="font-medium">{[f.vendor, f.model].filter(Boolean).join(" ") || "كاميرا"}</div>
                      <div className="muted text-xs font-mono" dir="ltr">{f.host}{f.profile_id ? ` · profile ${f.profile_id}` : ""}</div>
                    </div>
                    {f.profile_id && f.profile_id !== "generic" && <span className="text-xs text-brand-700 bg-brand-100 dark:bg-brand-900/50 dark:text-brand-300 rounded-full px-2 py-0.5">مختبرة معنا</span>}
                  </div>
                  <div className="grid sm:grid-cols-4 gap-2">
                    <input value={c.name} onChange={(e) => setCreds({ ...creds, [f.host]: { ...c, name: e.target.value } })} placeholder="الاسم (لاتيني)" dir="ltr" className="input" />
                    <input value={c.username} onChange={(e) => setCreds({ ...creds, [f.host]: { ...c, username: e.target.value } })} placeholder="المستخدم" dir="ltr" className="input" />
                    <input type="password" value={c.password} onChange={(e) => setCreds({ ...creds, [f.host]: { ...c, password: e.target.value } })} placeholder="كلمة المرور" dir="ltr" className="input" />
                    <select value={c.zone} onChange={(e) => setCreds({ ...creds, [f.host]: { ...c, zone: e.target.value } })} className="input">
                      <option value="">المنطقة</option>
                      <option value="entrance">المداخل والمخارج</option>
                      <option value="cashier">منطقة المحاسبة</option>
                      <option value="parking">المواقف</option>
                      <option value="warehouse">المستودع</option>
                      <option value="corridor">الممرات</option>
                      <option value="perimeter">السور الخارجي</option>
                    </select>
                  </div>
                </motion.div>
              );
            })}
            {found.length > 0 && (
              <div className="flex items-center justify-between">
                <p className="muted text-xs">كلمات المرور تُرسل إلى الجهاز وتُخزَّن مشفّرة عليه فقط.</p>
                <Button onClick={addAll} loading={busy} icon={Plus}>
                  إضافة {Object.values(creds).filter((c) => c.selected).length} كاميرا
                </Button>
              </div>
            )}
          </motion.div>
        )}

        {step === 3 && (
          <motion.div key="done" initial={{ opacity: 0, scale: 0.96 }} animate={{ opacity: 1, scale: 1 }} className="card p-8 text-center space-y-4">
            <motion.span initial={{ scale: 0 }} animate={{ scale: 1 }} transition={{ type: "spring", stiffness: 300, damping: 18, delay: 0.1 }} className="mx-auto size-20 rounded-full bg-brand-600 text-white flex items-center justify-center">
              <Check className="size-10" />
            </motion.span>
            <div>
              <div className="text-xl font-bold">الجهاز يعمل</div>
              <p className="muted text-sm mt-1">أُضيفت {added.length} كاميرا وبدأ التسجيل محلياً. ضع كل كاميرا على الخريطة لتكتمل فحوصات التغطية.</p>
            </div>
            <div className="flex justify-center gap-2">
              <Button onClick={() => router.push(`/sites/${form.site_id}`)}>صفحة الموقع</Button>
              <Button variant="secondary" onClick={() => router.push("/cameras")}>الكاميرات</Button>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
      {busy && step === 2 && (
        <div className="muted text-xs mt-3 flex items-center gap-2"><Loader2 className="size-3.5 animate-spin" /> الجهاز يضبط الكاميرات ويعيد تشغيل محرك التسجيل…</div>
      )}
    </div>
  );
}
