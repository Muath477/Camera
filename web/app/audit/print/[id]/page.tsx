"use client";

import { use, useEffect } from "react";
import { FileDown, Printer } from "lucide-react";
import { Badge, Button, Loading } from "@/components/ui";
import { download, type ComplianceReport } from "@/lib/api";
import { fmtDateTime } from "@/lib/format";
import { useApi } from "@/lib/hooks";

// Printable readiness report. The owner hands this to the inspector.
export default function PrintReport({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const report = useApi<ComplianceReport>(`/v1/sites/${id}/compliance`);

  useEffect(() => {
    if (report.data) setTimeout(() => window.print(), 400);
  }, [report.data]);

  if (!report.data) return <Loading />;
  const r = report.data;

  return (
    <div className="max-w-2xl mx-auto p-8 space-y-6 bg-white text-ink-900 min-h-screen">
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold">تقرير جاهزية الامتثال</h1>
          <div className="text-sm text-ink-500">نظام استخدام كاميرات المراقبة الأمنية</div>
        </div>
        <div className="flex gap-2 no-print">
          <Button variant="secondary" size="sm" icon={FileDown} onClick={() => download(`/v1/sites/${id}/compliance.pdf`, `تقرير-الامتثال-${r.site_name}.pdf`)}>PDF</Button>
          <Button variant="secondary" size="sm" icon={Printer} onClick={() => window.print()}>طباعة</Button>
        </div>
      </div>
      <table className="w-full text-sm">
        <tbody>
          <tr><td className="py-1 text-ink-500 w-40">المنشأة</td><td className="font-medium">{r.site_name}</td></tr>
          <tr><td className="py-1 text-ink-500">الفئة</td><td>{r.category?.name_ar}</td></tr>
          <tr><td className="py-1 text-ink-500">مدة الحفظ المطلوبة</td><td>{r.category?.retention_days} يوماً</td></tr>
          <tr><td className="py-1 text-ink-500">تاريخ التقرير</td><td>{fmtDateTime(r.generated_at)}</td></tr>
          <tr><td className="py-1 text-ink-500">نسبة الجاهزية</td><td className="font-bold text-lg">{r.score}%</td></tr>
        </tbody>
      </table>
      <table className="w-full text-sm border-t border-ink-200">
        <thead>
          <tr className="text-right text-ink-500">
            <th className="py-2 font-medium">البند</th>
            <th className="py-2 font-medium">الحالة</th>
            <th className="py-2 font-medium">التفاصيل</th>
          </tr>
        </thead>
        <tbody>
          {r.checks.map((c) => (
            <tr key={c.type} className="border-t border-ink-100 align-top">
              <td className="py-2 font-medium">{c.title_ar}</td>
              <td className="py-2"><Badge status={c.status} /></td>
              <td className="py-2 text-ink-600">{c.detail_ar}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <p className="text-xs text-ink-500">
        هذا التقرير مولّد من المنصة بناءً على حالة الأجهزة والكاميرات وقت إصداره، وسجل الوصول والأعطال محفوظ ويمكن تقديمه عند الطلب.
      </p>
    </div>
  );
}
