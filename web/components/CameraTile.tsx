"use client";

import { motion } from "motion/react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { Maximize2, Play, VideoOff } from "lucide-react";
import { getToken, thumbUrl, type Camera } from "@/lib/api";
import { Badge } from "./ui";

// Thumb fetches the camera's latest preview with the bearer token and
// refreshes it on an interval. The server asks the device to start sending
// previews on the first request.
export function Thumb({ cameraId, every = 15_000, className = "" }: { cameraId: string; every?: number; className?: string }) {
  const [src, setSrc] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    let url: string | null = null;
    let stop = false;
    const load = () =>
      fetch(thumbUrl(cameraId), { headers: { Authorization: `Bearer ${getToken()}` } })
        .then((r) => (r.status === 200 ? r.blob() : null))
        .then((b) => {
          if (stop || !b) return;
          if (url) URL.revokeObjectURL(url);
          url = URL.createObjectURL(b);
          setSrc(url);
        })
        .catch(() => setFailed(true));
    load();
    const t = setInterval(load, every);
    return () => {
      stop = true;
      clearInterval(t);
      if (url) URL.revokeObjectURL(url);
    };
  }, [cameraId, every]);
  if (src) {
    // eslint-disable-next-line @next/next/no-img-element
    return <motion.img initial={{ opacity: 0 }} animate={{ opacity: 1 }} src={src} alt="" className={`w-full h-full object-cover ${className}`} />;
  }
  return (
    <div className={`w-full h-full flex flex-col items-center justify-center gap-1 text-slate-500 text-xs ${className}`}>
      <VideoOff className="size-5 opacity-60" />
      {failed ? "تعذّر جلب الصورة" : "بانتظار أول صورة"}
    </div>
  );
}

export function CameraTile({ camera, subtitle, onLive }: { camera: Camera; subtitle?: string; onLive: () => void }) {
  const online = camera.status === "online";
  return (
    <div className="card overflow-hidden group">
      <button onClick={onLive} className="relative block w-full aspect-video bg-slate-900 text-right" aria-label={`بث حي: ${camera.display_name}`}>
        <Thumb cameraId={camera.id} className={online ? "" : "grayscale opacity-60"} />
        <div className="absolute inset-0 bg-gradient-to-t from-black/60 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity" />
        <span className="absolute inset-0 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-all scale-90 group-hover:scale-100">
          <span className="size-12 rounded-full bg-white/90 text-brand-700 flex items-center justify-center shadow-lg">
            <Play className="size-5 mr-[-2px]" fill="currentColor" />
          </span>
        </span>
        <span className="absolute top-2 right-2">
          <Badge status={camera.status} dot={!online} />
        </span>
        {camera.ptz && <span className="absolute top-2 left-2 text-[10px] rounded bg-black/50 text-white px-1.5 py-0.5">PTZ</span>}
      </button>
      <div className="p-3 flex items-center justify-between gap-2">
        <div className="min-w-0">
          <div className="font-medium truncate">{camera.display_name}</div>
          {subtitle && <div className="muted text-xs truncate">{subtitle}</div>}
        </div>
        <Link href={`/cameras/${camera.id}`} className="shrink-0 size-8 rounded-lg hover:bg-[var(--surface-2)] flex items-center justify-center muted hover:text-current" aria-label="التفاصيل" title="التفاصيل والتسجيلات">
          <Maximize2 className="size-4" />
        </Link>
      </div>
    </div>
  );
}
