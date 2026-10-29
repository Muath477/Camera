"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { api, setToken } from "@/lib/api";

export default function LoginPage() {
  const router = useRouter();
  const [token, setValue] = useState("");
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setToken(token.trim());
    try {
      await api("/v1/me");
      router.replace("/");
    } catch {
      setToken(null);
      setError("الرمز غير صالح أو منتهي");
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      <form onSubmit={submit} className="card w-full max-w-md p-6 space-y-4">
        <h1 className="text-2xl font-bold">تسجيل الدخول</h1>
        <p className="muted text-sm">
          في بيئة التطوير الصق الرمز الذي يطبعه أمر <code dir="ltr">api seed</code>. في الإنتاج يتم الدخول عبر مزود الهوية برقم
          الجوال والتحقق الثنائي.
        </p>
        <textarea
          dir="ltr"
          value={token}
          onChange={(e) => setValue(e.target.value)}
          rows={4}
          className="w-full rounded-lg border border-[var(--border)] bg-transparent p-3 text-xs font-mono"
          placeholder="eyJhbGciOi..."
        />
        {error && <p className="text-sm text-red-600">{error}</p>}
        <button className="w-full rounded-lg bg-brand-600 hover:bg-brand-700 text-white py-2 font-medium">دخول</button>
      </form>
    </div>
  );
}
