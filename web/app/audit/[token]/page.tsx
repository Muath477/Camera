"use client";

import { use, useEffect, useState } from "react";
import { Badge, Loading } from "@/components/ui";
import { API_URL, type ComplianceReport } from "@/lib/api";

// Read-only report for an outside auditor. No account is needed; the link
// expires after seven days.
export default function AuditorReport({ params }: { params: Promise<{ token: string }> }) {
  const { token } = use(params);
  const [report, setReport] = useState<ComplianceReport | null>(null);
  const [error, setError] = useState(false);

  useEffect(() => {
    fetch(`${API_URL}/public/compliance/${token}`)
      .then((r) => (r.ok ? r.json() : Promise.reject()))
      .then(setReport)
      .catch(() => setError(true));
  }, [token]);

  if (error) return <div className="p-8 text-center">الرابط غير صالح أو انتهت مدته.</div>;
  if (!report) return <Loading />;

  return (
    <div className="max-w-2xl mx-auto p-6 space-y-4">
      <h1 className="text-2xl font-bold">تقرير جاهزية الامتثال</h1>
      <div className="card p-4">
        <div className="font-semibold">{report.site_name}</div>
        <div className="muted text-sm">
          {report.category?.name_ar} · صدر في {new Date(report.generated_at).toLocaleString("ar-SA-u-nu-latn", { timeZone: "Asia/Riyadh" })}
        </div>
        <div className="text-4xl font-bold mt-2">{report.score}%</div>
      </div>
      <ul className="card divide-y divide-[var(--border)]">
        {report.checks.map((c) => (
          <li key={c.type} className="p-4">
            <div className="flex justify-between items-center">
              <span className="font-medium">{c.title_ar}</span>
              <Badge status={c.status} />
            </div>
            <div className="muted text-sm mt-1">{c.detail_ar}</div>
          </li>
        ))}
      </ul>
    </div>
  );
}
