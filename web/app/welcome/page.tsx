"use client";

import { motion, useReducedMotion } from "motion/react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { ArrowLeft, Bell, Check, HardDrive, Map, Minus, Search, ShieldCheck, Video, X } from "lucide-react";
import { Button, Reveal } from "@/components/ui";

const FEATURES = [
  { icon: ShieldCheck, title: "امتثال محسوب تلقائياً", body: "تقرير جاهزية لكل موقع حسب فئته: مدة الحفظ، تشغيل الكاميرات، تغطية المناطق، لوحات التنبيه، سجل الوصول. ورابط للمفتش بلا حساب." },
  { icon: HardDrive, title: "الفيديو يبقى في موقعك", body: "جهاز في كل فرع يسجّل على قرصه المخصص ويكشف محلياً. السحابة تستقبل الأحداث والصور المصغرة فقط، وكل شيء داخل المملكة." },
  { icon: Video, title: "أي كاميرا عندك", body: "اكتشاف تلقائي وتعريفات مختبرة لـ Hikvision وDahua وReolink وAxis وHanwha وUniview وTapo وغيرها. بلا تغيير كاميراتك." },
  { icon: Bell, title: "تنبيه خلال ثوانٍ", body: "شخص بعد الإغلاق، كسر زجاج، إنذار حريق. قواعد بالمنطقة والوقت، وإشعار على الجوال وواتساب، ومقطع قصير مع كل تنبيه." },
  { icon: Map, title: "خريطة فروعك", body: "كل الفروع على خريطة المملكة، وكل كاميرا بمخروط رؤيتها على موقعها. الأخضر يعمل، والأحمر يحتاج انتباهك." },
  { icon: Search, title: "ابحث بالوصف", body: "«رجل يحمل صندوقاً عند البوابة» عبر كل الفروع في ثوانٍ، بدل ساعات من التقليب في التسجيلات." },
];

type Cell = boolean | "partial" | string;
const COMPARE: { name: string; rows: Cell[]; note?: string }[] = [
  { name: "Camera", rows: [true, true, true, true, true, true, true, "من 15 ريال"] },
  { name: "Verkada", rows: ["partial", false, false, true, true, true, true, "≈ 78 ريال + كاميراتهم"], note: "تتطلب كاميرات الشركة نفسها" },
  { name: "Eagle Eye", rows: [true, false, "partial", true, true, true, "partial", "غير معلن"] },
  { name: "Rhombus", rows: ["partial", false, false, true, true, true, true, "غير معلن"], note: "كاميراتهم فقط" },
  { name: "Hik-Connect", rows: [true, "partial", false, true, "partial", true, false, "مجاني"], note: "كاميرات Hikvision فقط" },
  { name: "UniFi Protect", rows: ["partial", false, false, true, "partial", true, false, "مع العتاد"], note: "كاميرات Ubiquiti فقط" },
  { name: "Milestone", rows: [true, false, false, true, "partial", true, false, "ترخيص"], note: "يحتاج تكامل ومقدّم خدمة" },
];
const COMPARE_COLS = ["أي كاميرا", "واجهة عربية", "تقرير امتثال سعودي", "تنبيهات", "خريطة وفروع", "بث حي", "بحث بالوصف", "السعر لكل كاميرا شهرياً"];

const PLANS = [
  { name: "أساسي", price: 15, who: "محل أو مطعم صغير", items: ["تسجيل محلي ومدة حفظ مراقبة", "تطبيق عربي وتنبيهات الأعطال", "تقرير التفتيش وسجل التدقيق", "قفل الحوادث"] },
  { name: "ذكي", price: 25, who: "مطاعم، محطات وقود، مستودعات", items: ["كل ما في الأساسي", "كشف الأشخاص والمركبات والصوت", "إشعارات فورية ومقاطع في السحابة 30 يوماً", "المناطق والجداول"], highlight: true },
  { name: "سلاسل", price: 35, who: "سلاسل، فنادق، مجمعات", items: ["كل ما في الذكي", "خريطة الفروع وأدوار متعددة", "بحث بالوصف وتقارير مقارنة", "SSO وAPI واتفاقية مستوى خدمة"] },
];

function Cell({ v }: { v: Cell }) {
  if (v === true) return <Check className="size-5 text-brand-600 mx-auto" />;
  if (v === false) return <X className="size-4 text-ink-300 mx-auto" />;
  if (v === "partial") return <Minus className="size-4 text-warn-500 mx-auto" />;
  return <span className="text-xs whitespace-nowrap">{v}</span>;
}

function HeroVisual() {
  const reduce = useReducedMotion();
  const [tick, setTick] = useState(0);
  useEffect(() => {
    const t = setInterval(() => setTick((n) => n + 1), 2600);
    return () => clearInterval(t);
  }, []);
  const events = [
    { t: "شخص · المدخل الرئيسي", s: "info" },
    { t: "كسر زجاج · الكاشير", s: "critical" },
    { t: "سيارة · المواقف", s: "info" },
  ];
  return (
    <div className="relative">
      <motion.div
        initial={{ opacity: 0, y: 24, rotateX: 8 }}
        animate={{ opacity: 1, y: 0, rotateX: 0 }}
        transition={{ duration: 0.8, ease: [0.16, 1, 0.3, 1] }}
        className="card p-3 shadow-[var(--shadow-lg)] grid grid-cols-3 gap-2"
        style={{ perspective: 1000 }}
      >
        <div className="col-span-2 rounded-lg bg-gradient-to-br from-slate-800 to-slate-900 aspect-[4/3] relative overflow-hidden">
          <svg viewBox="0 0 200 150" className="absolute inset-0 w-full h-full opacity-70">
            <defs>
              <linearGradient id="g" x1="0" x2="1">
                <stop offset="0" stopColor="#0f9f6e" stopOpacity="0.5" />
                <stop offset="1" stopColor="#0f9f6e" stopOpacity="0" />
              </linearGradient>
            </defs>
            {[40, 110, 160].map((x, i) => (
              <g key={i}>
                <circle cx={x} cy={60 + i * 20} r="3" fill="#34d399" />
                <path d={`M${x},${60 + i * 20} L${x - 28},${30 + i * 20} A36,36 0 0 1 ${x + 22},${28 + i * 20} Z`} fill="url(#g)" />
              </g>
            ))}
            <motion.circle
              cx="110"
              cy="80"
              r="6"
              fill="none"
              stroke="#e11d48"
              strokeWidth="1.5"
              animate={reduce ? {} : { r: [4, 16], opacity: [0.9, 0] }}
              transition={{ repeat: Infinity, duration: 1.6 }}
            />
          </svg>
          <span className="absolute top-2 right-2 text-[10px] text-white/80 bg-black/40 rounded px-1.5 py-0.5 flex items-center gap-1"><span className="pulse-dot" /> فرع العليا</span>
        </div>
        <div className="space-y-2">
          {events.map((e, i) => {
            const active = tick % 3 === i;
            return (
              <motion.div key={e.t} animate={{ scale: active ? 1.04 : 1, opacity: active ? 1 : 0.6 }} className={`rounded-lg p-2 text-[11px] border ${e.s === "critical" ? "border-rose-300 bg-rose-50 dark:bg-rose-900/30 dark:border-rose-800" : "border-[var(--border)] surface-2"}`}>
                {e.t}
              </motion.div>
            );
          })}
          <div className="rounded-lg surface-2 p-2 text-center">
            <div className="text-xl font-bold text-brand-600">91%</div>
            <div className="muted text-[10px]">جاهزية الامتثال</div>
          </div>
        </div>
      </motion.div>
      <motion.div
        initial={{ opacity: 0, x: 20 }}
        animate={{ opacity: 1, x: 0 }}
        transition={{ delay: 0.6 }}
        className="absolute -bottom-5 -left-3 card px-3 py-2 text-xs flex items-center gap-2 shadow-[var(--shadow-md)]"
      >
        <span className="size-7 rounded-full bg-rose-100 text-rose-600 flex items-center justify-center"><Bell className="size-3.5" /></span>
        تنبيه على الجوال خلال 2 ثانية
      </motion.div>
    </div>
  );
}

export default function Welcome() {
  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-30 backdrop-blur bg-[var(--bg)]/80 border-b border-[var(--border)]">
        <div className="max-w-6xl mx-auto px-5 h-16 flex items-center justify-between">
          <Link href="/welcome" className="flex items-center gap-2 font-bold text-lg">
            <span className="size-9 rounded-xl bg-brand-500 flex items-center justify-center"><Video className="size-5 text-white" /></span> Camera
          </Link>
          <nav className="hidden md:flex items-center gap-6 text-sm">
            <a href="#features" className="hover:text-brand-600">المزايا</a>
            <a href="#how" className="hover:text-brand-600">كيف يعمل</a>
            <a href="#compare" className="hover:text-brand-600">المقارنة</a>
            <a href="#pricing" className="hover:text-brand-600">الأسعار</a>
          </nav>
          <Link href="/login"><Button size="sm">دخول</Button></Link>
        </div>
      </header>

      <section className="max-w-6xl mx-auto px-5 pt-16 pb-20 grid lg:grid-cols-2 gap-12 items-center">
        <div className="space-y-6">
          <motion.span initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} className="inline-flex items-center gap-2 rounded-full border border-brand-200 bg-brand-50 dark:bg-brand-900/30 dark:border-brand-800 text-brand-700 dark:text-brand-300 px-3 py-1 text-xs font-medium">
            <ShieldCheck className="size-3.5" /> متوافق مع نظام كاميرات المراقبة الأمنية
          </motion.span>
          <motion.h1 initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.05 }} className="text-4xl md:text-5xl font-bold leading-[1.2] tracking-tight">
            كاميراتك كلها في شاشة واحدة،
            <br />
            <span className="text-brand-600">والامتثال محسوب</span> قبل ما يجيك المفتش.
          </motion.h1>
          <motion.p initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.1 }} className="muted text-lg max-w-xl">
            جهاز صغير في كل فرع يشتغل مع أي كاميرا عندك، يسجّل محلياً ويرسل لك الأحداث فقط. منصة عربية، بيانات داخل المملكة، وتقرير جاهزية لكل موقع.
          </motion.p>
          <motion.div initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.15 }} className="flex flex-wrap gap-3">
            <Link href="/login"><Button size="lg">جرّب المنصة</Button></Link>
            <a href="#how"><Button size="lg" variant="secondary">كيف يعمل <ArrowLeft className="size-4" /></Button></a>
          </motion.div>
          <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} transition={{ delay: 0.3 }} className="flex gap-6 text-sm muted">
            <span><b className="text-[var(--text)]">10 دقائق</b> للتفعيل</span>
            <span><b className="text-[var(--text)]">أقل من ثانية</b> للبث الحي</span>
            <span><b className="text-[var(--text)]">100%</b> داخل المملكة</span>
          </motion.div>
        </div>
        <HeroVisual />
      </section>

      <section id="features" className="bg-[var(--surface)] border-y border-[var(--border)]">
        <div className="max-w-6xl mx-auto px-5 py-20">
          <Reveal className="text-center max-w-2xl mx-auto mb-12">
            <h2 className="text-3xl font-bold">مبني لصاحب المنشأة، لا لغرفة التحكم فقط</h2>
            <p className="muted mt-2">كل ميزة تجيب على سؤال واحد: هل منشأتي آمنة وملتزمة الآن؟</p>
          </Reveal>
          <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-5">
            {FEATURES.map((f, i) => (
              <Reveal key={f.title} delay={i * 0.06}>
                <div className="card card-hover p-5 h-full">
                  <span className="size-11 rounded-xl bg-brand-100 text-brand-700 dark:bg-brand-900/50 dark:text-brand-300 flex items-center justify-center mb-4"><f.icon className="size-5" /></span>
                  <h3 className="font-semibold text-lg">{f.title}</h3>
                  <p className="muted text-sm mt-1.5 leading-relaxed">{f.body}</p>
                </div>
              </Reveal>
            ))}
          </div>
        </div>
      </section>

      <section id="how" className="max-w-6xl mx-auto px-5 py-20">
        <Reveal className="text-center max-w-2xl mx-auto mb-12">
          <h2 className="text-3xl font-bold">كيف يعمل</h2>
          <p className="muted mt-2">ثلاث خطوات، ولا تحتاج فنياً في كل زيارة.</p>
        </Reveal>
        <div className="grid md:grid-cols-3 gap-6 relative">
          <div className="hidden md:block absolute top-8 inset-x-[16%] h-0.5 bg-gradient-to-l from-brand-500 via-brand-300 to-brand-500 opacity-40" />
          {[
            { n: 1, t: "وصّل الجهاز", b: "جهاز صغير بمنفذ للكاميرات ومنفذ للإنترنت. امسح الرمز من التطبيق وهو يتصل خلال دقيقة." },
            { n: 2, t: "اكتشف كاميراتك", b: "يمسح الجهاز الشبكة ويعرض الكاميرات بعلامتها وطرازها. أدخل كلمة المرور مرة واحدة وتبقى على الجهاز." },
            { n: 3, t: "راقب وامتثل", b: "خريطة، بث حي، تنبيهات، وتقرير جاهزية يتحدّث كل ساعة. اطبعه للمفتش أو أرسل له رابطاً." },
          ].map((s, i) => (
            <Reveal key={s.n} delay={i * 0.1}>
              <div className="text-center relative">
                <motion.span whileHover={{ scale: 1.08 }} className="mx-auto size-16 rounded-2xl bg-[var(--surface)] border-2 border-brand-500 text-brand-700 text-2xl font-bold flex items-center justify-center relative z-10">
                  {s.n}
                </motion.span>
                <h3 className="font-semibold text-lg mt-4">{s.t}</h3>
                <p className="muted text-sm mt-1.5 max-w-xs mx-auto">{s.b}</p>
              </div>
            </Reveal>
          ))}
        </div>
      </section>

      <section id="compare" className="bg-[var(--surface)] border-y border-[var(--border)]">
        <div className="max-w-6xl mx-auto px-5 py-20">
          <Reveal className="text-center max-w-2xl mx-auto mb-10">
            <h2 className="text-3xl font-bold">مقارنة مع المنصات الأخرى</h2>
            <p className="muted mt-2">أغلبها يطلب منك تغيير كاميراتك أو لا يعرف اللائحة السعودية. الأسعار المعلنة تقريبية.</p>
          </Reveal>
          <Reveal>
            <div className="card overflow-x-auto">
              <table className="w-full text-sm min-w-[760px]">
                <thead>
                  <tr className="muted text-xs">
                    <th className="p-3 text-right font-medium">المنصة</th>
                    {COMPARE_COLS.map((c) => (
                      <th key={c} className="p-3 font-medium text-center">{c}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {COMPARE.map((r, i) => (
                    <tr key={r.name} className={`border-t border-[var(--border)] ${i === 0 ? "bg-brand-50/60 dark:bg-brand-900/20" : ""}`}>
                      <td className="p-3">
                        <div className={`font-semibold ${i === 0 ? "text-brand-700 dark:text-brand-300" : ""}`} dir="ltr">{r.name}</div>
                        {r.note && <div className="muted text-[11px]">{r.note}</div>}
                      </td>
                      {r.rows.map((v, j) => (
                        <td key={j} className="p-3 text-center"><Cell v={v} /></td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <p className="muted text-xs mt-3 flex gap-4"><span className="flex items-center gap-1"><Check className="size-3.5 text-brand-600" /> متوفر</span><span className="flex items-center gap-1"><Minus className="size-3.5 text-warn-500" /> جزئي أو بشروط</span><span className="flex items-center gap-1"><X className="size-3.5 text-ink-300" /> غير متوفر</span></p>
          </Reveal>
        </div>
      </section>

      <section id="pricing" className="max-w-6xl mx-auto px-5 py-20">
        <Reveal className="text-center max-w-2xl mx-auto mb-12">
          <h2 className="text-3xl font-bold">الأسعار</h2>
          <p className="muted mt-2">لكل كاميرا شهرياً بفوترة سنوية. الجهاز مجاني مع التزام 24 شهراً.</p>
        </Reveal>
        <div className="grid md:grid-cols-3 gap-5 items-stretch">
          {PLANS.map((p, i) => (
            <Reveal key={p.name} delay={i * 0.08}>
              <div className={`card p-6 h-full flex flex-col relative ${p.highlight ? "border-brand-500 shadow-[0_0_0_4px_rgba(15,159,110,0.15)]" : ""}`}>
                {p.highlight && <span className="absolute -top-3 right-5 text-xs bg-brand-600 text-white rounded-full px-3 py-1">الأكثر طلباً</span>}
                <div className="font-semibold text-lg">{p.name}</div>
                <div className="muted text-xs">{p.who}</div>
                <div className="mt-4 flex items-baseline gap-1">
                  <span className="text-4xl font-bold">{p.price}</span>
                  <span className="muted text-sm">ريال / كاميرا / شهر</span>
                </div>
                <ul className="mt-5 space-y-2 text-sm flex-1">
                  {p.items.map((it) => (
                    <li key={it} className="flex gap-2"><Check className="size-4 text-brand-600 shrink-0 mt-0.5" /> {it}</li>
                  ))}
                </ul>
                <Link href="/login" className="mt-6"><Button variant={p.highlight ? "primary" : "secondary"} className="w-full">ابدأ</Button></Link>
              </div>
            </Reveal>
          ))}
        </div>
        <p className="muted text-xs text-center mt-6">غرامة واحدة لعدم الحفظ (5,000 ريال) تعادل اشتراك سنتين لموقع بثماني كاميرات على باقة «ذكي».</p>
      </section>

      <footer className="border-t border-[var(--border)]">
        <div className="max-w-6xl mx-auto px-5 py-8 flex flex-col md:flex-row items-center justify-between gap-3 text-sm muted">
          <span className="flex items-center gap-2 font-semibold text-[var(--text)]"><Video className="size-4 text-brand-600" /> Camera</span>
          <span>الحوسبة والتخزين والمفاتيح داخل المملكة العربية السعودية.</span>
          <Link href="/login" className="hover:text-brand-600">دخول</Link>
        </div>
      </footer>
    </div>
  );
}
