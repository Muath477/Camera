"use client";

import { motion } from "motion/react";
import Link from "next/link";
import { Cpu, HardDrive, Plus, Thermometer } from "lucide-react";
import { Badge, Button, EmptyState, ErrorBox, PageHeader, Skeleton } from "@/components/ui";
import type { Device, Site } from "@/lib/api";
import { fmtRelative, fmtTB } from "@/lib/format";
import { useApi } from "@/lib/hooks";

export default function DevicesPage() {
  const devices = useApi<{ items: Device[] | null }>("/v1/devices");
  const sites = useApi<{ items: Site[] | null }>("/v1/sites");
  const siteName = Object.fromEntries((sites.data?.items ?? []).map((s) => [s.id, s.name]));
  const list = devices.data?.items ?? [];

  return (
    <div>
      <PageHeader
        title="أجهزة التسجيل"
        subtitle="جهاز واحد في كل موقع: يسجّل على قرصه المخصص ويرسل الأحداث فقط."
        actions={
          <Link href="/devices/new">
            <Button icon={Plus}>إضافة جهاز</Button>
          </Link>
        }
      />
      {devices.error ? (
        <ErrorBox error={devices.error} retry={devices.reload} />
      ) : !devices.data ? (
        <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-3">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-40" />
          ))}
        </div>
      ) : list.length === 0 ? (
        <div className="card">
          <EmptyState
            icon={Cpu}
            title="لا توجد أجهزة بعد"
            hint="امسح رمز QR على الجهاز أو أدخل رقمه التسلسلي ورمز التفعيل."
            action={
              <Link href="/devices/new">
                <Button icon={Plus}>إضافة أول جهاز</Button>
              </Link>
            }
          />
        </div>
      ) : (
        <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-3">
          {list.map((d, i) => {
            const used = d.disk_total ? (d.disk_used ?? 0) / d.disk_total : 0;
            const health = d.health ?? null;
            return (
              <motion.div key={d.id} initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.05 * i }} className="card card-hover p-4 space-y-3">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="font-semibold font-mono text-sm" dir="ltr">{d.serial}</div>
                    <div className="muted text-xs truncate">{d.site_id ? siteName[d.site_id] : "غير مرتبط بموقع"}</div>
                  </div>
                  <Badge status={d.status} dot={d.status !== "online"} />
                </div>
                <div>
                  <div className="flex justify-between text-xs mb-1">
                    <span className="muted flex items-center gap-1"><HardDrive className="size-3.5" /> قرص التسجيل</span>
                    <span>{fmtTB(d.disk_used)} / {fmtTB(d.disk_total)}</span>
                  </div>
                  <div className="h-2 rounded-full surface-2 overflow-hidden">
                    <motion.div className={`h-full rounded-full ${used > 0.9 ? "bg-bad-500" : "bg-brand-500"}`} initial={{ width: 0 }} animate={{ width: `${used * 100}%` }} transition={{ duration: 0.8, delay: 0.1 * i }} />
                  </div>
                </div>
                <div className="grid grid-cols-3 gap-2 text-center">
                  <div className="rounded-lg surface-2 p-2">
                    <div className="text-lg font-bold leading-none">{d.retention_days != null ? Math.floor(d.retention_days) : "—"}</div>
                    <div className="muted text-[10px] mt-1">يوم حفظ</div>
                  </div>
                  <div className="rounded-lg surface-2 p-2">
                    <div className="text-lg font-bold leading-none flex items-center justify-center gap-0.5">
                      {health?.cpu_temp_c ? Math.round(health.cpu_temp_c) : "—"}
                      <Thermometer className="size-3.5 muted" />
                    </div>
                    <div className="muted text-[10px] mt-1">حرارة °C</div>
                  </div>
                  <div className="rounded-lg surface-2 p-2">
                    <div className="text-lg font-bold leading-none">{health?.queue_depth ?? 0}</div>
                    <div className="muted text-[10px] mt-1">بالانتظار</div>
                  </div>
                </div>
                <div className="flex justify-between muted text-xs">
                  <span>{d.hw_model} · {d.fw_version || "—"}</span>
                  <span>{d.last_seen_at ? fmtRelative(d.last_seen_at) : "لم يتصل بعد"}</span>
                </div>
              </motion.div>
            );
          })}
        </div>
      )}
    </div>
  );
}
