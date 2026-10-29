"use client";

import { useState } from "react";
import { Badge, ErrorBox, Loading } from "@/components/ui";
import { api, formatTime, type Device, type Site } from "@/lib/api";
import { useApi } from "@/lib/hooks";

const tb = (n: number | null) => (n == null ? "—" : `${(n / 1e12).toFixed(1)} TB`);

export default function DevicesPage() {
  const devices = useApi<{ items: Device[] | null }>("/v1/devices");
  const sites = useApi<{ items: Site[] | null }>("/v1/sites");
  const [form, setForm] = useState({ serial: "", claim_code: "", site_id: "" });
  const [msg, setMsg] = useState<string | null>(null);
  const siteName = Object.fromEntries((sites.data?.items ?? []).map((s) => [s.id, s.name]));

  async function claim(e: React.FormEvent) {
    e.preventDefault();
    try {
      await api("/v1/devices/claim", { method: "POST", body: JSON.stringify(form) });
      setMsg("تم ربط الجهاز. وصّله بالكهرباء والإنترنت وسيظهر متصلاً خلال دقيقة.");
      devices.reload();
    } catch (err) {
      setMsg(err instanceof Error ? err.message : String(err));
    }
  }

  const input = "rounded-lg border border-[var(--border)] bg-[var(--surface)] px-3 py-2 text-sm";

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">أجهزة التسجيل</h1>
      <form onSubmit={claim} className="card p-4 flex flex-wrap gap-2 items-end">
        <div className="w-full font-semibold">إضافة جهاز جديد</div>
        <input className={input} dir="ltr" placeholder="الرقم التسلسلي" value={form.serial} onChange={(e) => setForm({ ...form, serial: e.target.value })} />
        <input className={input} dir="ltr" placeholder="رمز التفعيل" value={form.claim_code} onChange={(e) => setForm({ ...form, claim_code: e.target.value })} />
        <select className={input} value={form.site_id} onChange={(e) => setForm({ ...form, site_id: e.target.value })}>
          <option value="">اختر الموقع</option>
          {(sites.data?.items ?? []).map((s) => (
            <option key={s.id} value={s.id}>{s.name}</option>
          ))}
        </select>
        <button className="rounded-lg bg-brand-600 hover:bg-brand-700 text-white px-4 py-2 text-sm">ربط</button>
        {msg && <div className="w-full text-sm muted">{msg}</div>}
      </form>
      {devices.error ? (
        <ErrorBox error={devices.error} />
      ) : !devices.data ? (
        <Loading />
      ) : (
        <div className="card overflow-x-auto">
          <table className="w-full text-sm whitespace-nowrap">
            <thead className="muted text-right">
              <tr className="border-b border-[var(--border)]">
                <th className="p-3 font-medium">الجهاز</th>
                <th className="p-3 font-medium">الموقع</th>
                <th className="p-3 font-medium">الحالة</th>
                <th className="p-3 font-medium">القرص</th>
                <th className="p-3 font-medium">تكفي لمدة</th>
                <th className="p-3 font-medium">آخر اتصال</th>
              </tr>
            </thead>
            <tbody>
              {(devices.data.items ?? []).map((d) => (
                <tr key={d.id} className="border-b border-[var(--border)] last:border-0">
                  <td className="p-3">
                    <div className="font-medium" dir="ltr">{d.serial}</div>
                    <div className="muted text-xs">{d.hw_model} · {d.fw_version}</div>
                  </td>
                  <td className="p-3">{d.site_id ? siteName[d.site_id] : "—"}</td>
                  <td className="p-3"><Badge status={d.status} /></td>
                  <td className="p-3">{tb(d.disk_used)} / {tb(d.disk_total)}</td>
                  <td className="p-3">{d.retention_days != null ? `${Math.floor(d.retention_days)} يوماً` : "—"}</td>
                  <td className="p-3 muted">{d.last_seen_at ? formatTime(d.last_seen_at) : "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
