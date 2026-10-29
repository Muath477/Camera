"use client";

import { motion } from "motion/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { ArrowLeft, KeyRound, ShieldCheck, Smartphone, Video } from "lucide-react";
import { Button } from "@/components/ui";
import { api, setToken } from "@/lib/api";

export default function LoginPage() {
  const router = useRouter();
  const [token, setValue] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setToken(token.trim());
    try {
      await api("/v1/me");
      router.replace("/");
    } catch {
      setToken(null);
      setError("الرمز غير صالح أو منتهي");
      setBusy(false);
    }
  }

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
        <motion.form initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} onSubmit={submit} className="w-full max-w-md space-y-5">
          <div className="lg:hidden flex items-center gap-2 font-bold text-lg mb-6">
            <span className="size-9 rounded-xl bg-brand-500 flex items-center justify-center"><Video className="size-5 text-white" /></span> Camera
          </div>
          <div>
            <h1 className="text-2xl font-bold">تسجيل الدخول</h1>
            <p className="muted text-sm mt-1">في الإنتاج يتم الدخول برقم الجوال ورمز تحقق. في بيئة التطوير الصق رمز الدخول.</p>
          </div>
          <label className="block">
            <span className="muted text-xs flex items-center gap-1"><KeyRound className="size-3" /> رمز التطوير</span>
            <textarea dir="ltr" value={token} onChange={(e) => setValue(e.target.value)} rows={4} className="input w-full mt-1 text-xs font-mono" placeholder="eyJhbGciOi..." required />
          </label>
          {error && (
            <motion.p initial={{ opacity: 0, x: -6 }} animate={{ opacity: 1, x: 0 }} className="text-sm text-bad-500">
              {error}
            </motion.p>
          )}
          <Button type="submit" size="lg" loading={busy} className="w-full">دخول</Button>
          <p className="muted text-xs text-center">
            ما عندك حساب؟ <Link href="/welcome" className="text-brand-600 inline-flex items-center gap-1">تعرّف على المنصة <ArrowLeft className="size-3" /></Link>
          </p>
        </motion.form>
      </div>
    </div>
  );
}
