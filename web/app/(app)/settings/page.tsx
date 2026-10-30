"use client";

import { motion } from "motion/react";
import { useEffect, useState } from "react";
import { BellRing, Download, MessageCircle, Moon, MoonStar, Smartphone, Sun, Monitor, Vibrate } from "lucide-react";
import { usePWA } from "@/components/PWA";
import { useToast } from "@/components/Toast";
import { Badge, Button, ErrorBox, PageHeader, Skeleton } from "@/components/ui";
import { api, type NotificationSettings } from "@/lib/api";
import { useApi } from "@/lib/hooks";
import { currentSubscription, pushSupported, subscribePush, unsubscribePush } from "@/lib/push";
import { useTheme } from "@/lib/theme";

const CHANNELS = [
  { id: "push", label: "إشعار على الجوال", icon: BellRing, hint: "يصل حتى والتطبيق مغلق، ويتجاوز الوضع الصامت للحرج." },
  { id: "sms", label: "رسالة نصية", icon: Smartphone, hint: "على رقم جوالك المسجّل." },
  { id: "whatsapp", label: "واتساب", icon: MessageCircle, hint: "عبر حساب الأعمال المعتمد." },
];

export default function SettingsPage() {
  const settings = useApi<NotificationSettings>("/v1/me/notifications");
  const toast = useToast();
  const { theme, setTheme } = useTheme();
  const { canInstall, install, standalone } = usePWA();

  const [critical, setCritical] = useState<string[]>([]);
  const [warning, setWarning] = useState<string[]>([]);
  const [quiet, setQuiet] = useState(false);
  const [from, setFrom] = useState("22:00");
  const [to, setTo] = useState("07:00");
  const [subscribed, setSubscribed] = useState<boolean | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [dirty, setDirty] = useState(false);

  useEffect(() => {
    if (!settings.data) return;
    const p = settings.data.prefs;
    setCritical(p.critical_channels);
    setWarning(p.warning_channels);
    setQuiet(!!p.quiet_from);
    if (p.quiet_from) setFrom(p.quiet_from);
    if (p.quiet_to) setTo(p.quiet_to);
    setDirty(false);
  }, [settings.data]);

  useEffect(() => {
    if (!pushSupported()) {
      setSubscribed(false);
      return;
    }
    currentSubscription().then((s) => setSubscribed(!!s));
  }, []);

  const toggle = (list: string[], set: (v: string[]) => void, id: string) => {
    set(list.includes(id) ? list.filter((x) => x !== id) : [...list, id]);
    setDirty(true);
  };

  async function save() {
    setBusy("save");
    try {
      await api("/v1/me/notifications", {
        method: "PUT",
        body: JSON.stringify({ critical_channels: critical, warning_channels: warning, quiet_from: quiet ? from : null, quiet_to: quiet ? to : null }),
      });
      setDirty(false);
      toast.push({ tone: "ok", title: "حُفظت الإعدادات" });
    } catch {
      toast.push({ tone: "critical", title: "تعذّر الحفظ" });
    } finally {
      setBusy(null);
    }
  }

  async function enablePush() {
    if (!settings.data?.vapid_public_key) return;
    setBusy("push");
    try {
      const sub = await subscribePush(settings.data.vapid_public_key);
      await api("/v1/me/push-subscriptions", { method: "POST", body: JSON.stringify({ endpoint: sub.endpoint, keys: sub.keys }) });
      setSubscribed(true);
      if (!critical.includes("push")) setCritical((c) => [...c, "push"]);
      toast.push({ tone: "ok", title: "فُعّلت الإشعارات على هذا الجهاز" });
      settings.reload();
    } catch (e) {
      toast.push({ tone: "critical", title: "لم تُفعَّل الإشعارات", body: e instanceof Error && e.message === "permission" ? "المتصفح رفض الإذن. فعّله من إعدادات الموقع." : "جرّب من متصفح آخر أو بعد تثبيت التطبيق." });
    } finally {
      setBusy(null);
    }
  }

  async function disablePush() {
    setBusy("push");
    try {
      const endpoint = await unsubscribePush();
      if (endpoint) await api("/v1/me/push-subscriptions", { method: "DELETE", body: JSON.stringify({ endpoint }) });
      setSubscribed(false);
      settings.reload();
    } finally {
      setBusy(null);
    }
  }

  async function test() {
    setBusy("test");
    try {
      await api("/v1/me/notifications/test", { method: "POST" });
      toast.push({ tone: "ok", title: "أُرسل إشعار تجريبي", body: "يصل خلال ثوانٍ إلى أجهزتك المشتركة." });
    } catch {
      toast.push({ tone: "critical", title: "لم يصل الإشعار", body: "فعّل الإشعارات على هذا الجهاز أولاً." });
    } finally {
      setBusy(null);
    }
  }

  if (settings.error) return <ErrorBox error={settings.error} retry={settings.reload} />;
  const s = settings.data;

  return (
    <div className="max-w-3xl">
      <PageHeader title="الإعدادات" subtitle="كيف ومتى تصلك التنبيهات، وشكل التطبيق على جهازك." />

      {!s ? (
        <div className="space-y-3">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-32" />
          ))}
        </div>
      ) : (
        <div className="space-y-4">
          <motion.section initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} className="card p-5 space-y-4">
            <div className="flex items-start justify-between gap-3">
              <div>
                <h2 className="font-semibold flex items-center gap-2"><Vibrate className="size-4 text-brand-600" /> إشعارات هذا الجهاز</h2>
                <p className="muted text-sm mt-1">
                  {subscribed === null ? "جارٍ الفحص…" : subscribed ? "هذا الجهاز يستقبل الإشعارات." : pushSupported() ? "فعّلها لتصلك التنبيهات الحرجة حتى والتطبيق مغلق." : "متصفحك لا يدعم الإشعارات. على iPhone ثبّت التطبيق على الشاشة الرئيسية أولاً."}
                </p>
                <p className="muted text-xs mt-1">أجهزة مشتركة: {s.push_subscriptions}</p>
              </div>
              <div className="flex gap-2 shrink-0">
                {subscribed ? (
                  <>
                    <Button variant="secondary" size="sm" loading={busy === "test"} onClick={test}>إشعار تجريبي</Button>
                    <Button variant="ghost" size="sm" loading={busy === "push"} onClick={disablePush}>إيقاف</Button>
                  </>
                ) : (
                  <Button size="sm" icon={BellRing} loading={busy === "push"} disabled={!pushSupported() || !s.vapid_public_key} onClick={enablePush}>تفعيل</Button>
                )}
              </div>
            </div>
            {(canInstall || !standalone) && (
              <div className="rounded-lg surface-2 p-3 flex items-center justify-between gap-3">
                <div className="text-sm">
                  <div className="font-medium flex items-center gap-2"><Download className="size-4" /> ثبّت التطبيق على جهازك</div>
                  <div className="muted text-xs mt-0.5">{canInstall ? "أيقونة على الشاشة الرئيسية وإشعارات أسرع." : "من قائمة المتصفح: «إضافة إلى الشاشة الرئيسية»."}</div>
                </div>
                {canInstall && <Button size="sm" variant="secondary" onClick={install}>تثبيت</Button>}
              </div>
            )}
          </motion.section>

          <motion.section initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.05 }} className="card p-5 space-y-4">
            <div>
              <h2 className="font-semibold">قنوات التنبيه</h2>
              <p className="muted text-sm mt-1">
                الحرج (حريق، كسر زجاج، كاميرا متوقفة) يصل فوراً على كل القنوات المختارة. التحذيرات تنتظر انتهاء ساعات الهدوء. الأحداث العادية تبقى داخل التطبيق.
              </p>
            </div>
            <ul className="divide-y divide-[var(--border)] text-sm">
              {CHANNELS.map((c) => {
                const enabled = c.id === "push" || (c.id === "sms" ? s.sms_enabled : s.whatsapp_enabled);
                return (
                  <li key={c.id} className="flex items-center justify-between gap-4 py-3">
                    <div className="flex items-center gap-3 min-w-0">
                      <span className="size-9 shrink-0 rounded-lg surface-2 flex items-center justify-center"><c.icon className="size-4" /></span>
                      <div className="min-w-0">
                        <div className="font-medium flex items-center gap-2 flex-wrap">
                          {c.label}
                          {!enabled && <Badge status="pending" />}
                        </div>
                        <div className="muted text-xs">{c.hint}{c.id !== "push" && s.phone ? ` ${s.phone}` : ""}</div>
                      </div>
                    </div>
                    <div className="flex gap-3 shrink-0">
                      <label className="flex flex-col items-center gap-1 w-12 cursor-pointer">
                        <input type="checkbox" className="size-5 accent-brand-600" checked={critical.includes(c.id)} onChange={() => toggle(critical, setCritical, c.id)} />
                        <span className="muted text-[11px]">حرج</span>
                      </label>
                      <label className="flex flex-col items-center gap-1 w-12 cursor-pointer">
                        <input type="checkbox" className="size-5 accent-brand-600" checked={warning.includes(c.id)} onChange={() => toggle(warning, setWarning, c.id)} />
                        <span className="muted text-[11px]">تحذير</span>
                      </label>
                    </div>
                  </li>
                );
              })}
            </ul>
            <div className="border-t border-[var(--border)] pt-4">
              <label className="flex items-center gap-3 cursor-pointer">
                <input type="checkbox" className="size-5 accent-brand-600" checked={quiet} onChange={(e) => { setQuiet(e.target.checked); setDirty(true); }} />
                <span className="flex items-center gap-2 font-medium"><MoonStar className="size-4" /> ساعات هدوء</span>
                <span className="muted text-xs">التحذيرات تُؤجَّل، والحرج يصل دائماً.</span>
              </label>
              {quiet && (
                <div className="flex items-center gap-3 mt-3 text-sm">
                  <span className="muted">من</span>
                  <input type="time" value={from} onChange={(e) => { setFrom(e.target.value); setDirty(true); }} className="input !py-1" dir="ltr" />
                  <span className="muted">إلى</span>
                  <input type="time" value={to} onChange={(e) => { setTo(e.target.value); setDirty(true); }} className="input !py-1" dir="ltr" />
                </div>
              )}
            </div>
            <div className="flex justify-end">
              <Button loading={busy === "save"} disabled={!dirty} onClick={save}>حفظ</Button>
            </div>
          </motion.section>

          <motion.section initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.1 }} className="card p-5">
            <h2 className="font-semibold mb-3">المظهر</h2>
            <div className="grid grid-cols-3 gap-2">
              {([
                ["light", "فاتح", Sun],
                ["dark", "داكن", Moon],
                ["system", "حسب النظام", Monitor],
              ] as const).map(([id, label, Icon]) => (
                <button key={id} onClick={() => setTheme(id)} className={`rounded-lg border p-3 flex flex-col items-center gap-2 text-sm transition-colors ${theme === id ? "border-brand-500 bg-brand-50 text-brand-700 dark:bg-brand-900/30 dark:text-brand-300" : "border-[var(--border)] hover:border-[var(--border-strong)]"}`}>
                  <Icon className="size-5" /> {label}
                </button>
              ))}
            </div>
            <p className="muted text-xs mt-3">الداكن مريح لغرفة العمليات، والفاتح للمالك في النهار.</p>
          </motion.section>
        </div>
      )}
    </div>
  );
}
