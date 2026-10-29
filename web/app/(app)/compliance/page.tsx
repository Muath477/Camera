"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Badge, Loading } from "@/components/ui";
import { api, type ComplianceReport, type Site } from "@/lib/api";
import { useApi } from "@/lib/hooks";

export default function CompliancePage() {
  const sites = useApi<{ items: Site[] | null }>("/v1/sites");
  const [reports, setReports] = useState<ComplianceReport[]>([]);
  const [shared, setShared] = useState<Record<string, string>>({});

  useEffect(() => {
    const list = sites.data?.items ?? [];
    Promise.all(list.map((s) => api<ComplianceReport>(`/v1/sites/${s.id}/compliance`).catch(() => null))).then((r) =>
      setReports(r.filter((x): x is ComplianceReport => x !== null)),
    );
  }, [sites.data]);

  async function share(siteId: string) {
    const res = await api<{ token: string }>(`/v1/sites/${siteId}/compliance/share`, { method: "POST" });
    setShared((m) => ({ ...m, [siteId]: `${location.origin}/audit/${res.token}` }));
  }

  if (!sites.data) return <Loading />;

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-bold">الامتثال لنظام كاميرات المراقبة</h1>
        <p className="muted text-sm">
          فحص مستمر لمدة الحفظ وتشغيل الكاميرات وتغطية المناطق ولوحات التنبيه وسجل الوصول، لكل موقع حسب فئته.
        </p>
      </div>
      <div className="grid md:grid-cols-2 gap-4">
        {reports.map((r) => (
          <div key={r.site_id} className="card p-4 space-y-3">
            <div className="flex items-start justify-between gap-2">
              <div>
                <Link href={`/sites/${r.site_id}`} className="font-semibold hover:text-brand-600">{r.site_name}</Link>
                <div className="muted text-xs">{r.category?.name_ar}</div>
              </div>
              <div className={`text-3xl font-bold ${r.score >= 80 ? "text-emerald-600" : r.score >= 50 ? "text-amber-600" : "text-red-600"}`}>
                {r.score}%
              </div>
            </div>
            <ul className="space-y-1.5">
              {r.checks.map((c) => (
                <li key={c.type} className="flex items-center justify-between text-sm gap-2">
                  <span>{c.title_ar}</span>
                  <Badge status={c.status} />
                </li>
              ))}
            </ul>
            <div className="flex gap-3 text-sm">
              <button onClick={() => window.print()} className="text-brand-600 underline">طباعة التقرير</button>
              <button onClick={() => share(r.site_id)} className="text-brand-600 underline">رابط للمدقق (7 أيام)</button>
            </div>
            {shared[r.site_id] && (
              <input readOnly dir="ltr" value={shared[r.site_id]} className="w-full text-xs rounded border border-[var(--border)] bg-transparent p-2" onFocus={(e) => e.target.select()} />
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
