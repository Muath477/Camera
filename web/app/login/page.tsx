"use client";

import { AnimatePresence, motion } from "motion/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { ArrowLeft, Building2, KeyRound, MessageCircle, ShieldCheck, Smartphone, Video } from "lucide-react";
import { Button } from "@/components/ui";
import { ApiError, api, setToken, type Account } from "@/lib/api";

type Step = "phone" | "code" | "choose" | "register";

// Sign-in by phone: a code by SMS or WhatsApp, then straight in. A number
// without an account can open one for its facility on the same screen.
export default function LoginPage() {
  const router = useRouter();
  const [step, setStep] = useState<Step>("phone");
  const [phone, setPhone] = useState("");
  const [channel, setChannel] = useState<"sms" | "whatsapp">("sms");
  const [code, setCode] = useState("");
  const [devCode, setDevCode] = useState<string | null>(null);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [name, setName] = useState("");
  const [tenantName, setTenantName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [devOpen, setDevOpen] = useState(false);
  const [devToken, setDevToken] = useState("");
  const [cooldown, setCooldown] = useState(0);
  const codeRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (cooldown <= 0) return;
    const t = setTimeout(() => setCooldown((c) => c - 1), 1000);
    return () => clearTimeout(t);
  }, [cooldown]);

  useEffect(() => {
    if (step === "code") codeRef.current?.focus();
  }, [step]);

  const fail = (e: unknown, fallback: string) => setError(e instanceof ApiError && e.status < 500 ? e.message : fallback);

  async function requestCode(e?: React.FormEvent) {
    e?.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const res = await api<{ phone: string; dev_code?: string }>("/v1/auth/otp/request", { method: "POST", body: JSON.stringify({ phone, channel }) });
      setPhone(res.phone);
      setDevCode(res.dev_code ?? null);
      setCode("");
      setCooldown(45);
      setStep("code");
    } catch (err) {
      fail(err, "تعذّر إرسال الرمز، حاول مرة أخرى.");
    } finally {
      setBusy(false);
    }
  }

  function enter(token: string) {
    setToken(token);
    router.replace("/");
  }

  async function verify(e?: React.FormEvent) {
    e?.preventDefault();
    if (code.length !== 6) return;
    setBusy(true);
    setError(null);
    try {
      const res = await api<{ token: string; accounts: Account[] }>("/v1/auth/otp/verify", { method: "POST", body: JSON.stringify({ phone, code }) });
      if (res.accounts.length > 1) {
        setAccounts(res.accounts);
        setStep("choose");
      } else {
        enter(res.token);
      }
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        setStep("register");
      } else {
        fail(err, "تعذّر التحقق من الرمز.");
      }
    } finally {
      setBusy(false);
    }
  }

  async function register(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      // The code was consumed by the verify call; ask for a fresh one and
      // register with it in one step when the person types it.
      const res = await api<{ token: string }>("/v1/auth/register", { method: "POST", body: JSON.stringify({ phone, code, name, tenant_name: tenantName }) });
      enter(res.token);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setError("انتهى الرمز، أرسلنا لك رمزاً جديداً. أدخله ثم أكمل التسجيل.");
        await requestCode();
        setStep("register");
      } else {
        fail(err, "تعذّر إنشاء الحساب.");
      }
    } finally {
      setBusy(false);
    }
  }

  async function devLogin(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setToken(devToken.trim());
    try {
      await api("/v1/me");
      router.replace("/");
    } catch {
      setToken(null);
      setError("رمز التطوير غير صالح أو منتهي");
      setBusy(false);
    }
  }

  const field = "input w-full mt-1";
  const slide = { initial: { opacity: 0, x: -12 }, animate: { opacity: 1, x: 0 }, exit: { opacity: 0, x: 12 }, transition: { duration: 0.22 } };

  return (
    <div className="min-h-screen grid lg:grid-cols-2">
      <div className="hidden lg:flex flex-col justify-between p-10 bg-[var(--sidebar)] text-white relative overflow-hidden">
        <div className="absolute -top-32 -left-32 size-96 rounded-full bg-brand-500/20 blur-3xl" />
        <div className="absolute -bottom-40 -right-20 size-[28rem] rounded-full bg-brand-400/10 blur-3xl" />
        <Link href="/welcome" className="flex items-center gap-2.5 relative">
          <span className="size-10 rounded-xl bg-brand-500 flex items-center justify-center">
            <Video className="size-5" />
          </span>
          <span className="text-xl font-bold">Camera</span>
        </Link>
        <motion.div initial={{ opacity: 0, y: 16 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.2 }} className="relative space-y-6 max-w-md">
          <h2 className="text-3xl font-bold leading-snug">كل فروعك في شاشة واحدة، والامتثال محسوب تلقائياً.</h2>
          <ul className="space-y-3 text-slate-300 text-sm">
            <li className="flex gap-3"><ShieldCheck className="size-5 text-brand-400 shrink-0" /> تقرير جاهزية لكل موقع حسب فئته في نظام كاميرات المراقبة.</li>
            <li className="flex gap-3"><Video className="size-5 text-brand-400 shrink-0" /> الفيديو يبقى في موقعك؛ السحابة ترى الأحداث فقط.</li>
            <li className="flex gap-3"><Smartphone className="size-5 text-brand-400 shrink-0" /> تنبيه على جوالك خلال ثوانٍ من الحدث.</li>
          </ul>
        </motion.div>
        <p className="text-xs text-slate-500 relative">بياناتك ومفاتيحك داخل المملكة.</p>
      </div>

      <div className="flex items-center justify-center p-6">
        <div className="w-full max-w-md">
          <div className="lg:hidden flex items-center gap-2 font-bold text-lg mb-6">
            <span className="size-9 rounded-xl bg-brand-500 flex items-center justify-center"><Video className="size-5 text-white" /></span> Camera
          </div>

          <AnimatePresence mode="wait">
            {step === "phone" && (
              <motion.form key="phone" {...slide} onSubmit={requestCode} className="space-y-5">
                <div>
                  <h1 className="text-2xl font-bold">تسجيل الدخول</h1>
                  <p className="muted text-sm mt-1">أدخل رقم جوالك وسنرسل لك رمز دخول. لا كلمات مرور.</p>
                </div>
                <label className="block">
                  <span className="muted text-xs flex items-center gap-1"><Smartphone className="size-3" /> رقم الجوال</span>
                  <input
                    dir="ltr"
                    inputMode="tel"
                    autoComplete="tel"
                    value={phone}
                    onChange={(e) => setPhone(e.target.value)}
                    className={`${field} text-lg tracking-wider`}
                    placeholder="05xxxxxxxx"
                    required
                    autoFocus
                  />
                </label>
                <div className="flex gap-2">
                  {(["sms", "whatsapp"] as const).map((c) => (
                    <button
                      key={c}
                      type="button"
                      onClick={() => setChannel(c)}
                      className={`flex-1 rounded-lg border px-3 py-2 text-sm flex items-center justify-center gap-2 transition-colors ${channel === c ? "border-brand-500 bg-brand-50 text-brand-700 dark:bg-brand-900/30 dark:text-brand-300" : "border-[var(--border)] hover:border-[var(--border-strong)]"}`}
                    >
                      {c === "sms" ? <Smartphone className="size-4" /> : <MessageCircle className="size-4" />}
                      {c === "sms" ? "رسالة نصية" : "واتساب"}
                    </button>
                  ))}
                </div>
                {error && <p className="text-sm text-bad-500">{error}</p>}
                <Button type="submit" size="lg" loading={busy} className="w-full">أرسل الرمز</Button>
                <p className="muted text-xs text-center">
                  أول مرة؟ أدخل رقمك وسنفتح لك حساب منشأتك خلال دقيقة.
                </p>
              </motion.form>
            )}

            {step === "code" && (
              <motion.form key="code" {...slide} onSubmit={verify} className="space-y-5">
                <div>
                  <h1 className="text-2xl font-bold">أدخل الرمز</h1>
                  <p className="muted text-sm mt-1">
                    أرسلناه إلى <span dir="ltr" className="font-mono">{phone}</span>{" "}
                    <button type="button" onClick={() => setStep("phone")} className="text-brand-600 hover:underline">تغيير</button>
                  </p>
                </div>
                <input
                  ref={codeRef}
                  dir="ltr"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  pattern="[0-9]{6}"
                  maxLength={6}
                  value={code}
                  onChange={(e) => {
                    const v = e.target.value.replace(/\D/g, "").slice(0, 6);
                    setCode(v);
                  }}
                  className="input w-full text-center text-3xl tracking-[0.5em] font-mono py-3"
                  placeholder="••••••"
                  required
                />
                {devCode && (
                  <p className="text-xs rounded-lg bg-amber-50 text-amber-800 dark:bg-amber-900/30 dark:text-amber-200 px-3 py-2">
                    بيئة تطوير: لا يوجد مرسل رسائل، الرمز هو <button type="button" className="font-mono font-bold underline" onClick={() => setCode(devCode)}>{devCode}</button>
                  </p>
                )}
                {error && <p className="text-sm text-bad-500">{error}</p>}
                <Button type="submit" size="lg" loading={busy} disabled={code.length !== 6} className="w-full">دخول</Button>
                <p className="muted text-xs text-center">
                  ما وصلك؟{" "}
                  <button type="button" disabled={cooldown > 0} onClick={() => requestCode()} className="text-brand-600 hover:underline disabled:opacity-50 disabled:no-underline">
                    {cooldown > 0 ? `أعد الإرسال بعد ${cooldown} ثانية` : "أعد الإرسال"}
                  </button>
                </p>
              </motion.form>
            )}

            {step === "choose" && (
              <motion.div key="choose" {...slide} className="space-y-4">
                <div>
                  <h1 className="text-2xl font-bold">اختر الحساب</h1>
                  <p className="muted text-sm mt-1">رقمك مرتبط بأكثر من منشأة.</p>
                </div>
                <ul className="space-y-2">
                  {accounts.map((a) => (
                    <li key={a.tenant_id}>
                      <button onClick={() => a.token && enter(a.token)} className="card card-hover w-full p-4 flex items-center gap-3 text-right">
                        <span className="size-10 rounded-lg bg-brand-100 text-brand-700 dark:bg-brand-900/40 dark:text-brand-300 flex items-center justify-center"><Building2 className="size-5" /></span>
                        <span className="font-semibold">{a.tenant_name}</span>
                        <ArrowLeft className="size-4 muted mr-auto" />
                      </button>
                    </li>
                  ))}
                </ul>
              </motion.div>
            )}

            {step === "register" && (
              <motion.form key="register" {...slide} onSubmit={register} className="space-y-5">
                <div>
                  <h1 className="text-2xl font-bold">أهلاً بك، نفتح حساب منشأتك</h1>
                  <p className="muted text-sm mt-1">
                    الرقم <span dir="ltr" className="font-mono">{phone}</span> ليس له حساب بعد. أكمل البيانات وستكون المالك.
                  </p>
                </div>
                <label className="block">
                  <span className="muted text-xs">اسمك</span>
                  <input value={name} onChange={(e) => setName(e.target.value)} className={field} required autoFocus />
                </label>
                <label className="block">
                  <span className="muted text-xs">اسم المنشأة</span>
                  <input value={tenantName} onChange={(e) => setTenantName(e.target.value)} className={field} placeholder="مطاعم النخلة" required />
                </label>
                <label className="block">
                  <span className="muted text-xs">رمز التحقق</span>
                  <input dir="ltr" inputMode="numeric" maxLength={6} value={code} onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))} className={`${field} font-mono tracking-[0.4em] text-center`} required />
                  {devCode && <span className="muted text-[11px]">بيئة تطوير: الرمز {devCode}</span>}
                </label>
                {error && <p className="text-sm text-bad-500">{error}</p>}
                <Button type="submit" size="lg" loading={busy} className="w-full">إنشاء الحساب</Button>
                <button type="button" onClick={() => setStep("phone")} className="muted text-xs text-center w-full hover:underline">رجوع</button>
              </motion.form>
            )}
          </AnimatePresence>

          <div className="mt-8 border-t border-[var(--border)] pt-4">
            <button type="button" onClick={() => setDevOpen((v) => !v)} className="muted text-xs flex items-center gap-1 hover:text-current">
              <KeyRound className="size-3" /> دخول برمز تطوير
            </button>
            <AnimatePresence>
              {devOpen && (
                <motion.form initial={{ opacity: 0, height: 0 }} animate={{ opacity: 1, height: "auto" }} exit={{ opacity: 0, height: 0 }} onSubmit={devLogin} className="overflow-hidden">
                  <textarea dir="ltr" value={devToken} onChange={(e) => setDevToken(e.target.value)} rows={3} className="input w-full mt-2 text-xs font-mono" placeholder="eyJhbGciOi..." required />
                  <Button type="submit" variant="secondary" size="sm" loading={busy} className="mt-2">دخول</Button>
                </motion.form>
              )}
            </AnimatePresence>
          </div>
          <p className="muted text-xs text-center mt-6">
            <Link href="/welcome" className="text-brand-600 inline-flex items-center gap-1">تعرّف على المنصة <ArrowLeft className="size-3" /></Link>
          </p>
        </div>
      </div>
    </div>
  );
}
