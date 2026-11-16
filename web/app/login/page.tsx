"use client";

import { AnimatePresence, motion } from "motion/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { ArrowLeft, Bell, Building2, KeyRound, MessageCircle, ShieldCheck, Smartphone, Video } from "lucide-react";
import Mascot, { type Mood } from "@/components/Mascot";
import { Button } from "@/components/ui";
import { ApiError, api, setToken, type Account } from "@/lib/api";

type Step = "phone" | "code" | "choose" | "register";

const PHOTO_LINES = [
  "قل «تشيز»!",
  "لقطة حلوة، بس الإضاءة تحتاج شغل.",
  "حفظتها في سجل التدقيق… أمزح.",
  "كاميرا تصوّر كاميرا؟ هذا تصوير ذاتي.",
  "عيني عليك، بس بلطف.",
  "ابتسم، أنت في البث المباشر… لا، لست فيه.",
];

// buddyState picks Lensy's face, where it looks and what it says from the
// state of the sign-in. Errors win, then success, then the step.
function buddyState(s: { step: Step; focus: "phone" | "code" | null; digits: number; code: number; busy: boolean; error: string | null; success: boolean; photo?: string }): {
  mood: Mood;
  look: { x: number; y: number } | null;
  line: string;
} {
  if (s.success) return { mood: "wink", look: null, line: "ابتسم… دخلت! نجهّز لك فروعك." };
  if (s.error) return { mood: "dizzy", look: null, line: "أوبس! دوّختني. شوف الرسالة تحت وجرّب مرة ثانية." };
  if (s.busy) return { mood: "happy", look: { x: 0, y: -1 }, line: s.step === "phone" ? "أرسل الحمام الزاجل… أقصد الرسالة." : "لحظة، أتأكد منك…" };
  if (s.photo) return { mood: "wink", look: null, line: s.photo };
  switch (s.step) {
    case "phone":
      if (s.digits >= 10) return { mood: "happy", look: { x: 0, y: 1 }, line: "رقم مرتّب! اضغط «أرسل الرمز» وأنا أنتظر." };
      // The eye follows the number along the field as it is typed (LTR field).
      if (s.focus === "phone" || s.digits > 0) return { mood: "happy", look: { x: -1 + (s.digits / 10) * 2, y: 1 }, line: s.digits ? `كمّل… ${s.digits} من 10 أرقام.` : "أنا أتابع، اكتب رقمك." };
      return { mood: "happy", look: null, line: "هلا! أنا لنزي. أراقب فروعك طول اليوم، بس رقمك ما أعرفه." };
    case "code":
      if (s.code === 6) return { mood: "wink", look: null, line: "ستة من ستة. ما شفت شي، والله." };
      if (s.focus === "code") return { mood: "shy", look: null, line: "ما أشوف! اكتب الرمز وأنا مغمّض." };
      return { mood: "happy", look: null, line: "وصلك الرمز؟ أنا أنتظر معك." };
    case "choose":
      return { mood: "happy", look: { x: 0, y: 1 }, line: "ما شاء الله، أكثر من منشأة! اختر وحدة." };
    case "register":
      return { mood: "wink", look: null, line: "ضيف جديد! حيّاك، عرّفني على منشأتك." };
  }
}

// Confetti: a handful of stickers that burst from the card on success.
function Confetti() {
  const colors = ["bg-sun-400", "bg-pop-400", "bg-aqua-400", "bg-brand-400", "bg-lilac-400"];
  return (
    <div className="pointer-events-none absolute inset-x-0 top-10 h-0 motion-reduce:hidden" aria-hidden>
      {Array.from({ length: 18 }, (_, i) => {
        const angle = (i / 18) * Math.PI * 2;
        return (
          <motion.span
            key={i}
            initial={{ x: 0, y: 0, opacity: 1, rotate: 0, scale: 0.6 }}
            animate={{ x: Math.cos(angle) * (90 + (i % 3) * 30), y: Math.sin(angle) * 80 + 60, opacity: 0, rotate: i * 40, scale: 1 }}
            transition={{ duration: 0.9, ease: "easeOut" }}
            className={`absolute start-14 size-3 border-2 border-[var(--line)] ${colors[i % colors.length]} ${i % 2 ? "rounded-full" : "rounded-sm"}`}
          />
        );
      })}
    </div>
  );
}

// Sign-in by phone: a code by SMS or WhatsApp, then straight in. A number
// without an account can open one for its facility on the same screen.
export default function LoginPage() {
  const router = useRouter();
  const [step, setStep] = useState<Step>("phone");
  const [phone, setPhone] = useState("");
  const [channel, setChannel] = useState<"sms" | "whatsapp">("sms");
  const [code, setCode] = useState("");
  const [devCode, setDevCode] = useState<string | null>(null);
  // Proof from the server that this phone was just verified; lets a new
  // number open an account without typing a second code.
  const [ticket, setTicket] = useState<string | null>(null);
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

  // Lensy's reactions: which field has focus, a shake on every error, a
  // flash when the mascot is clicked, and a celebration on the way in.
  const [focus, setFocus] = useState<"phone" | "code" | null>(null);
  const [shakeKey, setShakeKey] = useState(0);
  const [success, setSuccess] = useState(false);
  const [photo, setPhoto] = useState<{ key: number; line: string } | null>(null);

  useEffect(() => {
    if (error) setShakeKey((k) => k + 1);
  }, [error]);

  useEffect(() => {
    if (!photo?.line) return;
    const t = setTimeout(() => setPhoto((p) => (p ? { ...p, line: "" } : p)), 2600);
    return () => clearTimeout(t);
  }, [photo?.key, photo?.line]);

  function snap() {
    const key = (photo?.key ?? 0) + 1;
    setPhoto({ key, line: key >= 5 ? "خمس صور! خلاص، يكفي غرور." : PHOTO_LINES[Math.floor(Math.random() * PHOTO_LINES.length)] });
  }

  const digits = phone.replace(/\D/g, "").length;
  const buddy = buddyState({ step, focus, digits, code: code.length, busy, error, success, photo: photo?.line });

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
    setSuccess(true);
    // A beat for the celebration; none when motion is reduced.
    const wait = window.matchMedia("(prefers-reduced-motion: reduce)").matches ? 0 : 900;
    setTimeout(() => router.replace("/"), wait);
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
        setTicket(typeof err.body.ticket === "string" ? err.body.ticket : null);
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
      // The verify call spent the code and handed back a ticket for this
      // phone. Without one (or once it expires) the person types a new code.
      const proof = ticket ? { ticket } : { code };
      const res = await api<{ token: string }>("/v1/auth/register", { method: "POST", body: JSON.stringify({ phone, ...proof, name, tenant_name: tenantName }) });
      enter(res.token);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setTicket(null);
        setError("انتهت صلاحية التحقق، أرسلنا لك رمزاً جديداً. أدخله ثم أكمل التسجيل.");
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

  const field = "input w-full mt-1.5";
  const slide = { initial: { opacity: 0, x: -16, rotate: -1 }, animate: { opacity: 1, x: 0, rotate: 0 }, exit: { opacity: 0, x: 16 }, transition: { type: "spring" as const, stiffness: 420, damping: 32 } };
  const errorLine = error && (
    <p role="alert" className="text-sm font-medium text-bad-600 dark:text-pop-400 flex items-center gap-2">
      <Mascot size={26} mood="alert" /> {error}
    </p>
  );
  const stepNo = step === "phone" ? 1 : step === "code" ? 2 : 3;

  return (
    <div className="min-h-screen grid lg:grid-cols-[1.05fr_1fr]">
      {/* The welcome side: one big friendly sticker scene. */}
      <div className="hidden lg:flex flex-col justify-between p-10 m-3 rounded-[var(--radius-xl)] bg-[var(--sidebar)] text-white border-2 border-[var(--line)] shadow-[var(--pop)] relative overflow-hidden">
        <Link href="/welcome" className="flex items-center gap-2 relative self-start rounded-xl">
          <Mascot size={44} />
          <span className="font-display text-2xl font-extrabold">Camera</span>
        </Link>

        <div className="relative">
          <div className="relative mx-auto w-fit">
            <button type="button" onClick={snap} aria-label="التقط صورة مع لنزي" className="group block rounded-full cursor-pointer transition-transform hover:-rotate-3 active:scale-95">
              <span key={shakeKey} className={shakeKey ? "shake block" : "block"}>
                <Mascot size={190} mood={buddy.mood} look={buddy.look} follow />
              </span>
            </button>
            <span className="bob absolute -top-4 -start-24 rounded-2xl bg-sun-400 text-ink-900 border-2 border-[var(--line)] px-3 py-1.5 text-sm font-bold shadow-[var(--pop-sm)]" style={{ ["--r" as string]: "-6deg" }}>
              الامتثال 91%
            </span>
            <span className="bob absolute top-16 -end-28 rounded-2xl bg-aqua-400 text-ink-900 border-2 border-[var(--line)] px-3 py-1.5 text-sm font-bold shadow-[var(--pop-sm)] [animation-delay:-1.6s]" style={{ ["--r" as string]: "5deg" }}>
              12 كاميرا تعمل
            </span>
            <span className="bob absolute -bottom-3 -start-16 rounded-2xl bg-pop-400 text-ink-900 border-2 border-[var(--line)] px-3 py-1.5 text-sm font-bold shadow-[var(--pop-sm)] flex items-center gap-1.5 [animation-delay:-3s]" style={{ ["--r" as string]: "-3deg" }}>
              <Bell className="size-4" /> تنبيه خلال ثانيتين
            </span>
          </div>
          <motion.h2 initial={{ opacity: 0, y: 16 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.15 }} className="mt-14 text-4xl font-extrabold leading-tight max-w-lg">
            كل فروعك في شاشة واحدة، و<span className="text-sun-300">الامتثال محسوب</span> تلقائياً.
          </motion.h2>
          <ul className="mt-6 space-y-3 text-[var(--sidebar-text)] max-w-md">
            <li className="flex gap-3"><ShieldCheck className="size-5 text-brand-300 shrink-0 mt-1" /> تقرير جاهزية لكل موقع حسب فئته في نظام كاميرات المراقبة.</li>
            <li className="flex gap-3"><Video className="size-5 text-aqua-400 shrink-0 mt-1" /> الفيديو يبقى في موقعك، والسحابة ترى الأحداث فقط.</li>
            <li className="flex gap-3"><Smartphone className="size-5 text-pop-400 shrink-0 mt-1" /> تنبيه على جوالك خلال ثوانٍ من الحدث.</li>
          </ul>
        </div>
        <p className="text-xs text-[var(--sidebar-text)] opacity-80 relative">بياناتك ومفاتيحك داخل المملكة.</p>
      </div>

      <div className="flex items-center justify-center p-5 sm:p-8">
        <div className="w-full max-w-md">
          <div className="lg:hidden flex items-center gap-2 mb-8">
            <Mascot size={44} /> <span className="font-display text-2xl font-extrabold">Camera</span>
          </div>

          <div className="card p-6 sm:p-8 relative">
            {/* Lensy sits on the card and comments on what you do. */}
            <div className="flex items-end gap-3 mb-6 min-h-[76px]">
              <button type="button" onClick={snap} aria-label="التقط صورة مع لنزي" className="shrink-0 rounded-2xl cursor-pointer transition-transform hover:-rotate-6 active:scale-90">
                <span key={shakeKey} className={shakeKey ? "shake block" : "block"}>
                  <Mascot size={68} mood={buddy.mood} look={buddy.look} />
                </span>
              </button>
              <AnimatePresence mode="wait">
                <motion.p
                  key={buddy.line}
                  initial={{ opacity: 0, y: 6, scale: 0.92 }}
                  animate={{ opacity: 1, y: 0, scale: 1 }}
                  exit={{ opacity: 0, y: -4 }}
                  transition={{ type: "spring", stiffness: 520, damping: 26 }}
                  aria-live="polite"
                  className="relative mb-3 rounded-2xl rounded-es-sm border-2 border-[var(--line)] bg-sun-100 dark:bg-[var(--surface-2)] px-3 py-2 text-sm font-semibold leading-snug shadow-[var(--pop-sm)]"
                >
                  {buddy.line}
                </motion.p>
              </AnimatePresence>
            </div>

            {/* Three dots that show where the person is in the sign-in. */}
            <ol className="flex items-center gap-2 mb-6" aria-label={`الخطوة ${stepNo} من 3`}>
              {[1, 2, 3].map((n) => (
                <li key={n} className={`h-2.5 rounded-full border-2 border-[var(--line)] transition-all duration-300 ${n === stepNo ? "w-10 bg-sun-400" : n < stepNo ? "w-2.5 bg-brand-400" : "w-2.5 bg-[var(--surface-2)]"}`} />
              ))}
            </ol>
            {success && <Confetti />}
            {photo?.line && <div key={photo.key} className="camera-flash" aria-hidden />}

            <AnimatePresence mode="wait">
              {step === "phone" && (
                <motion.form key="phone" {...slide} onSubmit={requestCode} className="space-y-5">
                  <div>
                    <h1 className="text-3xl font-extrabold">أهلاً! سجّل دخولك</h1>
                    <p className="muted mt-1">أدخل رقم جوالك ونرسل لك رمزاً من 6 أرقام. بلا كلمات مرور.</p>
                  </div>
                  <label className="block">
                    <span className="text-sm font-semibold flex items-center gap-1.5"><Smartphone className="size-4" /> رقم الجوال</span>
                    <input
                      dir="ltr"
                      inputMode="tel"
                      autoComplete="tel"
                      value={phone}
                      onChange={(e) => {
                        setPhone(e.target.value);
                        setError(null);
                      }}
                      onFocus={() => setFocus("phone")}
                      onBlur={() => setFocus(null)}
                      className={`${field} text-xl tracking-wider font-mono`}
                      placeholder="05xxxxxxxx"
                      required
                      autoFocus
                    />
                    {/* A film strip that fills up as the number is typed. */}
                    <span className="mt-2 flex gap-1" aria-hidden>
                      {Array.from({ length: 10 }, (_, i) => (
                        <span key={i} className={`h-2 flex-1 rounded-sm border-[1.5px] border-[var(--card-line)] transition-colors duration-200 ${i < digits ? (digits >= 10 ? "bg-brand-400" : "bg-sun-400") : "bg-[var(--surface-2)]"}`} />
                      ))}
                    </span>
                  </label>
                  <fieldset>
                    <legend className="text-sm font-semibold mb-1.5">أرسل الرمز عبر</legend>
                    <div className="grid grid-cols-2 gap-2">
                      {(["sms", "whatsapp"] as const).map((c) => (
                        <button
                          key={c}
                          type="button"
                          aria-pressed={channel === c}
                          onClick={() => setChannel(c)}
                          className={`group min-h-11 rounded-2xl border-2 px-3 py-2 text-sm font-semibold flex items-center justify-center gap-2 transition-all ${channel === c ? "border-[var(--line)] bg-brand-100 text-brand-800 shadow-[var(--pop-sm)] dark:bg-brand-900/50 dark:text-brand-200" : "border-[var(--border-strong)] hover:border-[var(--line)]"}`}
                        >
                          {c === "sms" ? <Smartphone className="size-4 wiggle" /> : <MessageCircle className="size-4 wiggle" />}
                          {c === "sms" ? "رسالة نصية" : "واتساب"}
                        </button>
                      ))}
                    </div>
                  </fieldset>
                  {errorLine}
                  <Button type="submit" size="lg" loading={busy} className="w-full">أرسل الرمز</Button>
                  <p className="muted text-sm text-center">أول مرة هنا؟ أدخل رقمك ونفتح حساب منشأتك خلال دقيقة.</p>
                </motion.form>
              )}

              {step === "code" && (
                <motion.form key="code" {...slide} onSubmit={verify} className="space-y-5">
                  <div>
                    <h1 className="text-3xl font-extrabold">أدخل الرمز</h1>
                    <p className="muted mt-1">
                      أرسلناه إلى <bdi dir="ltr" className="font-mono text-[var(--text)]">{phone}</bdi>{" "}
                      <button type="button" onClick={() => setStep("phone")} className="text-brand-700 dark:text-brand-300 font-semibold underline-offset-4 hover:underline">غيّر الرقم</button>
                    </p>
                  </div>
                  <label className="block">
                    <span className="sr-only">رمز الدخول</span>
                    <input
                      ref={codeRef}
                      dir="ltr"
                      inputMode="numeric"
                      autoComplete="one-time-code"
                      pattern="[0-9]{6}"
                      maxLength={6}
                      value={code}
                      onChange={(e) => {
                        setCode(e.target.value.replace(/\D/g, "").slice(0, 6));
                        setError(null);
                      }}
                      onFocus={() => setFocus("code")}
                      onBlur={() => setFocus(null)}
                      className="input w-full text-center text-4xl tracking-[0.45em] font-mono py-3"
                      placeholder="••••••"
                      required
                    />
                  </label>
                  {devCode && (
                    <p className="text-sm rounded-2xl bg-sun-100 text-sun-700 border-2 border-dashed border-sun-400 px-3 py-2 dark:bg-sun-400/10 dark:text-sun-300">
                      بيئة تطوير بلا مرسل رسائل. الرمز:{" "}
                      <button type="button" className="font-mono font-bold underline" onClick={() => {
                          setCode(devCode);
                          setError(null);
                        }}>{devCode}</button>
                    </p>
                  )}
                  {errorLine}
                  <Button type="submit" size="lg" loading={busy} disabled={code.length !== 6} className="w-full">ادخل</Button>
                  <p className="muted text-sm text-center">
                    ما وصلك؟{" "}
                    <button type="button" disabled={cooldown > 0} onClick={() => requestCode()} className="text-brand-700 dark:text-brand-300 font-semibold hover:underline disabled:opacity-60 disabled:no-underline tabular-nums">
                      {cooldown > 0 ? `أعد الإرسال بعد ${cooldown} ثانية` : "أعد الإرسال"}
                    </button>
                  </p>
                </motion.form>
              )}

              {step === "choose" && (
                <motion.div key="choose" {...slide} className="space-y-4">
                  <div>
                    <h1 className="text-3xl font-extrabold">أي منشأة اليوم؟</h1>
                    <p className="muted mt-1">رقمك مرتبط بأكثر من منشأة، اختر واحدة.</p>
                  </div>
                  <ul className="space-y-3">
                    {accounts.map((a, i) => (
                      <li key={a.tenant_id}>
                        <button onClick={() => a.token && enter(a.token)} className="card card-hover group w-full p-4 flex items-center gap-3 text-start">
                          <span className={`size-11 rounded-xl border-2 border-[var(--line)] text-ink-900 flex items-center justify-center -rotate-6 ${["bg-brand-400", "bg-sun-400", "bg-aqua-400", "bg-lilac-400"][i % 4]}`}>
                            <Building2 className="size-5 wiggle" />
                          </span>
                          <span className="font-bold">{a.tenant_name}</span>
                          <ArrowLeft className="size-5 ms-auto transition-transform group-hover:-translate-x-1 rtl:group-hover:-translate-x-1" />
                        </button>
                      </li>
                    ))}
                  </ul>
                </motion.div>
              )}

              {step === "register" && (
                <motion.form key="register" {...slide} onSubmit={register} className="space-y-5">
                  <div>
                    <h1 className="text-3xl font-extrabold">أهلاً بك! نفتح حساب منشأتك</h1>
                    <p className="muted mt-1">
                      الرقم <bdi dir="ltr" className="font-mono text-[var(--text)]">{phone}</bdi> ليس له حساب بعد. أكمل البيانات وستكون المالك.
                    </p>
                  </div>
                  <label className="block">
                    <span className="text-sm font-semibold">اسمك</span>
                    <input value={name} onChange={(e) => setName(e.target.value)} className={field} autoComplete="name" required autoFocus />
                  </label>
                  <label className="block">
                    <span className="text-sm font-semibold">اسم المنشأة</span>
                    <input value={tenantName} onChange={(e) => setTenantName(e.target.value)} className={field} autoComplete="organization" placeholder="مثل: مطاعم النخلة" required />
                  </label>
                  {!ticket && (
                    <label className="block">
                      <span className="text-sm font-semibold">رمز التحقق الجديد</span>
                      <input dir="ltr" inputMode="numeric" autoComplete="one-time-code" maxLength={6} value={code} onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))} className={`${field} font-mono tracking-[0.4em] text-center text-xl`} required />
                      {devCode && <span className="muted text-xs">بيئة تطوير: الرمز {devCode}</span>}
                    </label>
                  )}
                  {errorLine}
                  <Button type="submit" size="lg" loading={busy} className="w-full">أنشئ الحساب</Button>
                  <button type="button" onClick={() => setStep("phone")} className="muted text-sm text-center w-full min-h-11 hover:underline">رجوع</button>
                </motion.form>
              )}
            </AnimatePresence>
          </div>

          <div className="mt-6">
            <button type="button" aria-expanded={devOpen} onClick={() => setDevOpen((v) => !v)} className="muted text-sm min-h-11 flex items-center gap-1.5 hover:text-[var(--text)]">
              <KeyRound className="size-4" /> دخول برمز تطوير
            </button>
            <AnimatePresence>
              {devOpen && (
                <motion.form initial={{ opacity: 0, height: 0 }} animate={{ opacity: 1, height: "auto" }} exit={{ opacity: 0, height: 0 }} onSubmit={devLogin} className="overflow-hidden">
                  <label className="sr-only" htmlFor="dev-token">رمز التطوير</label>
                  <textarea id="dev-token" dir="ltr" value={devToken} onChange={(e) => setDevToken(e.target.value)} rows={3} className="input w-full mt-1 text-xs font-mono" placeholder="eyJhbGciOi..." required />
                  <Button type="submit" variant="secondary" size="sm" loading={busy} className="mt-2">ادخل بالرمز</Button>
                </motion.form>
              )}
            </AnimatePresence>
          </div>
          <p className="text-sm text-center mt-6">
            <Link href="/welcome" className="group inline-flex items-center gap-1 font-semibold text-brand-700 dark:text-brand-300 hover:underline">
              تعرّف على المنصة <ArrowLeft className="size-4 transition-transform group-hover:-translate-x-1" />
            </Link>
          </p>
        </div>
      </div>
    </div>
  );
}

