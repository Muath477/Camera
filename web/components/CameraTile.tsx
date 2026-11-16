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
    <div className="card card-hover overflow-hidden group">
      {/* A little viewfinder: the frame corners say "this is a camera". */}
      <button onClick={onLive} className="relative block w-full aspect-video bg-ink-900 text-start border-b-2 border-[var(--card-line)]" aria-label={`بث حي: ${camera.display_name}`}>
        <Thumb cameraId={camera.id} className={online ? "" : "grayscale opacity-60"} />
        <span aria-hidden className="pointer-events-none absolute inset-3 opacity-70 group-hover:opacity-100 transition-opacity">
          <span className="absolute top-0 start-0 size-4 border-t-2 border-s-2 border-white/80 rounded-ss-md" />
          <span className="absolute top-0 end-0 size-4 border-t-2 border-e-2 border-white/80 rounded-se-md" />
          <span className="absolute bottom-0 start-0 size-4 border-b-2 border-s-2 border-white/80 rounded-es-md" />
          <span className="absolute bottom-0 end-0 size-4 border-b-2 border-e-2 border-white/80 rounded-ee-md" />
        </span>
        <span className="absolute inset-0 flex items-center justify-center opacity-0 group-hover:opacity-100 group-focus-visible:opacity-100 transition-all scale-75 group-hover:scale-100">
          <span className="size-14 rounded-full bg-sun-400 text-ink-900 border-2 border-[var(--line)] flex items-center justify-center shadow-[var(--pop-sm)]">
            <Play className="size-6 ms-0.5" fill="currentColor" />
          </span>
        </span>
        <span className="absolute top-2 start-2">
          <Badge status={camera.status} dot={!online} />
        </span>
        {camera.ptz && <span className="absolute top-2 end-2 text-[10px] font-bold rounded-full bg-lilac-400 text-ink-900 border-2 border-[var(--line)] px-2 py-0.5">PTZ</span>}
      </button>
      <div className="p-3 flex items-center justify-between gap-2">
        <div className="min-w-0">
          <div className="font-display font-bold truncate">{camera.display_name}</div>
          {subtitle && <div className="muted text-xs truncate">{subtitle}</div>}
        </div>
        <Link href={`/cameras/${camera.id}`} className="shrink-0 size-11 rounded-full hover:bg-[var(--surface-2)] flex items-center justify-center muted hover:text-current" aria-label={`تفاصيل وتسجيلات ${camera.display_name}`} title="التفاصيل والتسجيلات">
          <Maximize2 className="size-4" />
        </Link>
      </div>
    </div>
  );
}
