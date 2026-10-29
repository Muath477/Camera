"use client";

import { motion } from "motion/react";
import { useEffect, useRef, useState } from "react";
import { Camera as CameraIcon, ChevronDown, ChevronLeft, ChevronRight, ChevronUp, Minus, Plus, Square, X } from "lucide-react";
import { api, type Camera } from "@/lib/api";
import { useToast } from "./Toast";
import { Button, Spinner } from "./ui";

// LiveView negotiates WebRTC with the camera's edge device. The offer and
// answer pass through the API; the video itself flows device to browser.
export default function LiveView({ camera, onClose }: { camera: Camera; onClose: () => void }) {
  const video = useRef<HTMLVideoElement>(null);
  const [state, setState] = useState<"connecting" | "playing" | "failed">("connecting");
  const [error, setError] = useState("");
  const [snap, setSnap] = useState<{ sha256: string; url: string } | null>(null);
  const [snapping, setSnapping] = useState(false);
  const toast = useToast();

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  useEffect(() => {
    const pc = new RTCPeerConnection({ iceServers: [{ urls: "stun:stun.l.google.com:19302" }] });
    pc.addTransceiver("video", { direction: "recvonly" });
    pc.addTransceiver("audio", { direction: "recvonly" });
    pc.ontrack = (e) => {
      if (video.current && e.streams[0]) {
        video.current.srcObject = e.streams[0];
        setState("playing");
      }
    };
    let cancelled = false;
    (async () => {
      try {
        await pc.setLocalDescription(await pc.createOffer());
        await new Promise<void>((resolve) => {
          if (pc.iceGatheringState === "complete") return resolve();
          const t = setTimeout(resolve, 2000);
          pc.onicegatheringstatechange = () => {
            if (pc.iceGatheringState === "complete") {
              clearTimeout(t);
              resolve();
            }
          };
        });
        const res = await api<{ sdp: string }>(`/v1/cameras/${camera.id}/webrtc`, {
          method: "POST",
          body: JSON.stringify({ sdp: pc.localDescription?.sdp }),
        });
        if (!cancelled) await pc.setRemoteDescription({ type: "answer", sdp: res.sdp });
      } catch (e) {
        if (!cancelled) {
          setState("failed");
          setError(e instanceof Error ? e.message : String(e));
        }
      }
    })();
    return () => {
      cancelled = true;
      pc.close();
    };
  }, [camera.id]);

  async function snapshot() {
    setSnapping(true);
    try {
      const s = await api<{ sha256: string; url: string }>(`/v1/cameras/${camera.id}/snapshots`, { method: "POST" });
      setSnap(s);
      toast.push({ tone: "ok", title: "لقطة موقّعة محفوظة", body: `SHA-256 ${s.sha256.slice(0, 12)}…` });
    } catch (e) {
      toast.push({ tone: "critical", title: "تعذّر أخذ اللقطة", body: e instanceof Error ? e.message : String(e) });
    } finally {
      setSnapping(false);
    }
  }

  const ptz = (action: string) => api(`/v1/cameras/${camera.id}/ptz`, { method: "POST", body: JSON.stringify({ action }) }).catch(() => {});
  const hold = (action: string) => ({
    onPointerDown: () => ptz(action),
    onPointerUp: () => ptz("STOP"),
    onPointerLeave: () => ptz("STOP"),
  });

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      className="fixed inset-0 z-50 bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 md:p-6"
      onClick={onClose}
      role="dialog"
      aria-label={`بث حي: ${camera.display_name}`}
    >
      <motion.div
        initial={{ opacity: 0, scale: 0.96, y: 10 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        exit={{ opacity: 0, scale: 0.96, y: 10 }}
        transition={{ type: "spring", stiffness: 380, damping: 34 }}
        className="card w-full max-w-5xl overflow-hidden shadow-[var(--shadow-lg)]"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-4 py-3 border-b border-[var(--border)]">
          <h3 className="font-semibold flex items-center gap-2">
            <span className={`pulse-dot ${state === "playing" ? "" : "bad"}`} />
            {camera.display_name}
            <span className="muted text-xs font-normal">{state === "playing" ? "بث مباشر" : state === "connecting" ? "جارٍ الاتصال" : "متوقف"}</span>
          </h3>
          <button onClick={onClose} className="size-8 rounded-lg hover:bg-[var(--surface-2)] flex items-center justify-center" aria-label="إغلاق">
            <X className="size-4" />
          </button>
        </div>
        <div className="relative aspect-video bg-black">
          <video ref={video} autoPlay playsInline muted className="w-full h-full object-contain" />
          {state !== "playing" && (
            <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 text-slate-300 text-sm text-center p-6">
              {state === "connecting" ? (
                <>
                  <Spinner className="size-8" />
                  جارٍ فتح اتصال مباشر مع جهاز الموقع…
                </>
              ) : (
                <>
                  <span className="font-medium text-slate-100">تعذّر بدء البث</span>
                  <span className="text-xs max-w-md" dir="auto">{error}</span>
                  <span className="text-xs max-w-md muted">في بيئة التطوير لا يوجد فيديو؛ على الجهاز الحقيقي يفتح البث خلال ثانية.</span>
                </>
              )}
            </div>
          )}
          {camera.ptz && (
            <div className="absolute bottom-3 left-3 grid grid-cols-3 gap-1 p-1 rounded-xl bg-black/50 backdrop-blur" dir="ltr">
              <span />
              <PtzBtn {...hold("MOVE_UP")}><ChevronUp className="size-4" /></PtzBtn>
              <span />
              <PtzBtn {...hold("MOVE_LEFT")}><ChevronLeft className="size-4" /></PtzBtn>
              <PtzBtn onPointerDown={() => ptz("STOP")}><Square className="size-3" /></PtzBtn>
              <PtzBtn {...hold("MOVE_RIGHT")}><ChevronRight className="size-4" /></PtzBtn>
              <PtzBtn {...hold("ZOOM_OUT")}><Minus className="size-4" /></PtzBtn>
              <PtzBtn {...hold("MOVE_DOWN")}><ChevronDown className="size-4" /></PtzBtn>
              <PtzBtn {...hold("ZOOM_IN")}><Plus className="size-4" /></PtzBtn>
            </div>
          )}
        </div>
        <div className="flex flex-wrap gap-2 items-center px-4 py-3">
          <Button icon={CameraIcon} loading={snapping} onClick={snapshot}>لقطة موقّعة</Button>
          {snap && (
            <a href={snap.url} target="_blank" rel="noreferrer" className="text-xs text-brand-600 hover:underline" dir="ltr">
              SHA-256 {snap.sha256.slice(0, 16)}…
            </a>
          )}
          <span className="muted text-xs mr-auto">كل جلسة بث تُسجَّل في سجل التدقيق باسمك.</span>
        </div>
      </motion.div>
    </motion.div>
  );
}

function PtzBtn(props: React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return <button {...props} className="size-9 rounded-lg bg-white/15 hover:bg-white/30 text-white flex items-center justify-center active:scale-95 transition" />;
}
