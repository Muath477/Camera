"use client";

import { AnimatePresence, motion } from "motion/react";
import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { Grid2X2, Grid3X3, Video } from "lucide-react";
import { CameraTile } from "@/components/CameraTile";
import LiveView from "@/components/LiveView";
import { EmptyState, ErrorBox, PageHeader, Skeleton } from "@/components/ui";
import type { Camera, Site } from "@/lib/api";
import { useApi } from "@/lib/hooks";

export default function CamerasPage() {
  const sites = useApi<{ items: Site[] | null }>("/v1/sites");
  const cameras = useApi<{ items: Camera[] | null }>("/v1/cameras");
  const [site, setSite] = useState("");
  const [cols, setCols] = useState<2 | 3>(3);
  const [live, setLive] = useState<Camera | null>(null);
  const [onlyOffline, setOnlyOffline] = useState(false);

  // Links from the dashboard open the grid already filtered.
  useEffect(() => {
    setOnlyOffline(new URLSearchParams(location.search).get("status") === "offline");
  }, []);

  const siteName = useMemo(() => Object.fromEntries((sites.data?.items ?? []).map((s) => [s.id, s.name])), [sites.data]);
  const inSite = (cameras.data?.items ?? []).filter((c) => !site || c.site_id === site);
  const offline = inSite.filter((c) => c.status !== "online").length;
  const list = onlyOffline ? inSite.filter((c) => c.status !== "online") : inSite;

  if (cameras.error) return <ErrorBox error={cameras.error} retry={cameras.reload} />;

  return (
    <div>
      <PageHeader
        title="الكاميرات"
        subtitle={cameras.data ? `${inSite.length} كاميرا${offline ? `، ${offline} منقطعة` : "، كلها تعمل"}` : undefined}
        actions={
          <>
            <button
              type="button"
              onClick={() => setOnlyOffline((v) => !v)}
              className={`h-10 px-3 rounded-lg border text-sm transition-colors ${onlyOffline ? "bg-amber-100 border-amber-300 text-amber-800 dark:bg-amber-900/40 dark:border-amber-700 dark:text-amber-200" : "border-[var(--border)] hover:border-[var(--border-strong)]"}`}
            >
              المنقطعة فقط{offline ? ` (${offline})` : ""}
            </button>
            <select value={site} onChange={(e) => setSite(e.target.value)} className="input">
              <option value="">كل المواقع</option>
              {(sites.data?.items ?? []).map((s) => (
                <option key={s.id} value={s.id}>{s.name}</option>
              ))}
            </select>
            <div className="hidden sm:flex rounded-lg border border-[var(--border)] overflow-hidden">
              <button onClick={() => setCols(2)} className={`size-10 flex items-center justify-center ${cols === 2 ? "bg-brand-600 text-white" : ""}`} aria-label="شبكة 2">
                <Grid2X2 className="size-4" />
              </button>
              <button onClick={() => setCols(3)} className={`size-10 flex items-center justify-center ${cols === 3 ? "bg-brand-600 text-white" : ""}`} aria-label="شبكة 3">
                <Grid3X3 className="size-4" />
              </button>
            </div>
          </>
        }
      />
      {cameras.loading && !cameras.data ? (
        <div className={`grid gap-3 ${cols === 2 ? "sm:grid-cols-2" : "sm:grid-cols-2 lg:grid-cols-3"}`}>
          {[0, 1, 2, 3, 4, 5].map((i) => (
            <Skeleton key={i} className="aspect-video" />
          ))}
        </div>
      ) : list.length === 0 ? (
        onlyOffline ? (
          <EmptyState icon={Video} title="كل الكاميرات تعمل" hint="لا توجد كاميرا منقطعة في هذا النطاق." />
        ) : (
          <EmptyState icon={Video} title="لا توجد كاميرات" hint="أضف جهازاً ثم اكتشف الكاميرات على شبكته." />
        )
      ) : (
        <motion.div layout className={`grid gap-3 ${cols === 2 ? "sm:grid-cols-2" : "sm:grid-cols-2 lg:grid-cols-3"}`}>
          <AnimatePresence>
            {list.map((c, i) => (
              <motion.div key={c.id} layout initial={{ opacity: 0, scale: 0.96 }} animate={{ opacity: 1, scale: 1 }} exit={{ opacity: 0, scale: 0.96 }} transition={{ delay: Math.min(i * 0.03, 0.3) }}>
                <CameraTile camera={c} subtitle={siteName[c.site_id]} onLive={() => setLive(c)} />
              </motion.div>
            ))}
          </AnimatePresence>
        </motion.div>
      )}
      <p className="muted text-xs mt-4">
        الصور المصغرة تتجدد كل 15 ثانية بلا فتح بث. اضغط على كاميرا للبث الحي، أو افتح <Link href="/sites" className="underline">صفحة الموقع</Link> للتفاصيل.
      </p>
      <AnimatePresence>{live && <LiveView camera={live} onClose={() => setLive(null)} />}</AnimatePresence>
    </div>
  );
}
