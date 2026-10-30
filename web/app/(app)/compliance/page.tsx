"use client";

import { AnimatePresence, motion } from "motion/react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { Check, ChevronDown, Copy, FileDown, FileText, Link2, ShieldCheck } from "lucide-react";
import { useToast } from "@/components/Toast";
import { Badge, Button, Counter, EmptyState, PageHeader, Ring, Skeleton } from "@/components/ui";
import { api, download, type ComplianceReport, type Site } from "@/lib/api";
import { useApi } from "@/lib/hooks";

export default function CompliancePage() {
  const sites = useApi<{ items: Site[] | null }>("/v1/sites");
  const [reports, setReports] = useState<ComplianceReport[] | null>(null);
  const [open, setOpen] = useState<string | null>(null);
  const [shared, setShared] = useState<Record<string, string>>({});
  const toast = useToast();

  useEffect(() => {
    const list = sites.data?.items ?? [];
    if (!sites.data) return;
    Promise.all(list.map((s) => api<ComplianceReport>(`/v1/sites/${s.id}/compliance`).catch(() => null))).then((r) =>
      setReports(r.filter((x): x is ComplianceReport => x !== null)),
    );
  }, [sites.data]);

  useEffect(() => {
    if (location.hash) setOpen(location.hash.slice(1));
  }, []);

  const [downloading, setDownloading] = useState<string | null>(null);
  async function pdf(r: ComplianceReport) {
    setDownloading(r.site_id);
    try {
      await download(`/v1/sites/${r.site_id}/compliance.pdf`, `تقرير-الامتثال-${r.site_name}.pdf`);
    } catch {
      toast.push({ tone: "critical", title: "تعذّر إنشاء الملف", body: "حاول مرة أخرى بعد قليل." });
    } finally {
      setDownloading(null);
    }
  }

  async function share(siteId: string) {
    const res = await api<{ token: string }>(`/v1/sites/${siteId}/compliance/share`, { method: "POST" });
    const url = `${location.origin}/audit/${res.token}`;
    setShared((m) => ({ ...m, [siteId]: url }));
    try {
      await navigator.clipboard.writeText(url);
      toast.push({ tone: "ok", title: "نُسخ رابط المدقق", body: "صالح 7 أيام، قراءة فقط، بلا حساب." });
    } catch {
      toast.push({ tone: "ok", title: "أُنشئ رابط المدقق", body: "صالح 7 أيام." });
    }
  }

  const ready = reports?.filter((r) => r.score >= 80).length ?? 0;
  const avg = reports && reports.length ? Math.round(reports.reduce((a, r) => a + r.score, 0) / reports.length) : 0;
  const failing = reports?.reduce((a, r) => a + r.checks.filter((c) => c.status === "fail").length, 0) ?? 0;

  return (
    <div>
      <PageHeader
        title="الامتثال لنظام كاميرات المراقبة"
        subtitle="فحص مستمر لكل موقع حسب فئته: مدة الحفظ، تشغيل الكاميرات، تغطية المناطق، لوحات التنبيه، سجل الوصول."
      />

      {!reports ? (
        <div className="grid grid-cols-3 gap-3 mb-6">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-24" />
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-3 gap-3 mb-6">
          <motion.div initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} className="card p-4 flex items-center gap-4">
            <Ring value={avg} size={64} stroke={6} />
            <div>
              <div className="muted text-xs">متوسط الجاهزية</div>
              <div className="font-semibold">{reports.length} موقع</div>
            </div>
          </motion.div>
          <motion.div initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.05 }} className="card p-4">
            <div className="muted text-xs">جاهز للتفتيش</div>
            <div className="text-3xl font-bold text-brand-600"><Counter value={ready} /><span className="muted text-base font-medium"> / {reports.length}</span></div>
          </motion.div>
          <motion.div initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.1 }} className="card p-4">
            <div className="muted text-xs">بنود غير مستوفاة</div>
            <div className={`text-3xl font-bold ${failing ? "text-bad-500" : "text-brand-600"}`}><Counter value={failing} /></div>
          </motion.div>
        </div>
      )}

      {reports && reports.length === 0 && <EmptyState icon={ShieldCheck} title="لا توجد مواقع" hint="أضف موقعاً لتبدأ فحوصات الامتثال." />}

      <div className="space-y-3">
        {(reports ?? []).map((r, i) => {
          const isOpen = open === r.site_id;
          const fails = r.checks.filter((c) => c.status === "fail").length;
          const warns = r.checks.filter((c) => c.status === "warn").length;
          return (
            <motion.div key={r.site_id} id={r.site_id} initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.05 * i }} className="card overflow-hidden">
              <button onClick={() => setOpen(isOpen ? null : r.site_id)} className="w-full flex items-center gap-4 p-4 text-right">
                <Ring value={r.score} size={56} stroke={6} />
                <div className="min-w-0 flex-1">
                  <div className="font-semibold">{r.site_name}</div>
                  <div className="muted text-xs">{r.category?.name_ar} · {r.category?.retention_days} يوماً</div>
                  <div className="flex gap-1.5 mt-1.5 flex-wrap">
                    {fails > 0 && <Badge status="fail">{fails} غير مستوفى</Badge>}
                    {warns > 0 && <Badge status="warn">{warns} تنبيه</Badge>}
                    {fails === 0 && warns === 0 && <Badge status="pass">جاهز للتفتيش</Badge>}
                  </div>
                </div>
                <ChevronDown className={`size-5 muted transition-transform ${isOpen ? "rotate-180" : ""}`} />
              </button>
              <AnimatePresence initial={false}>
                {isOpen && (
                  <motion.div initial={{ height: 0, opacity: 0 }} animate={{ height: "auto", opacity: 1 }} exit={{ height: 0, opacity: 0 }} transition={{ duration: 0.3, ease: [0.16, 1, 0.3, 1] }} className="overflow-hidden">
                    <div className="border-t border-[var(--border)] p-4 space-y-4">
                      <ul className="divide-y divide-[var(--border)]">
                        {r.checks.map((c) => (
                          <li key={c.type} className="py-2.5 flex items-start gap-3">
                            <span className={`mt-0.5 size-5 rounded-full flex items-center justify-center shrink-0 ${c.status === "pass" ? "bg-brand-100 text-brand-700" : c.status === "warn" ? "bg-amber-100 text-amber-700" : "bg-rose-100 text-rose-700"}`}>
                              {c.status === "pass" ? <Check className="size-3" /> : <span className="text-[10px] font-bold">!</span>}
                            </span>
                            <div className="flex-1 min-w-0">
                              <div className="flex items-center justify-between gap-2">
                                <span className="font-medium text-sm">{c.title_ar}</span>
                                <Badge status={c.status} />
                              </div>
                              <div className="muted text-xs mt-0.5">{c.detail_ar}</div>
                            </div>
                          </li>
                        ))}
                      </ul>
                      <div className="flex flex-wrap gap-2 items-center">
                        <Link href={`/sites/${r.site_id}`}>
                          <Button variant="secondary" size="sm">صفحة الموقع</Button>
                        </Link>
                        <Button variant="secondary" size="sm" icon={FileDown} loading={downloading === r.site_id} onClick={() => pdf(r)}>
                          تنزيل PDF
                        </Button>
                        <Button variant="secondary" size="sm" icon={FileText} onClick={() => window.open(`/audit/print/${r.site_id}`, "_blank")}>
                          طباعة
                        </Button>
                        <Button size="sm" icon={Link2} onClick={() => share(r.site_id)}>
                          رابط للمدقق (7 أيام)
                        </Button>
                        {shared[r.site_id] && (
                          <button
                            onClick={() => navigator.clipboard.writeText(shared[r.site_id]).catch(() => {})}
                            className="flex items-center gap-1 text-xs muted hover:text-current max-w-full"
                            dir="ltr"
                          >
                            <Copy className="size-3" /> <span className="truncate">{shared[r.site_id]}</span>
                          </button>
                        )}
                      </div>
                    </div>
                  </motion.div>
                )}
              </AnimatePresence>
            </motion.div>
          );
        })}
      </div>
    </div>
  );
}
