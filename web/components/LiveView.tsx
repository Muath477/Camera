"use client";

import { useEffect, useRef, useState } from "react";
import { api, type Camera } from "@/lib/api";

// LiveView negotiates WebRTC with the camera's edge device. The offer and
// answer pass through the API; the video itself flows device to browser.
export default function LiveView({ camera, onClose }: { camera: Camera; onClose: () => void }) {
  const video = useRef<HTMLVideoElement>(null);
  const [state, setState] = useState<"connecting" | "playing" | "failed">("connecting");
  const [error, setError] = useState("");
  const [snap, setSnap] = useState<{ sha256: string; url: string } | null>(null);

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
    try {
      setSnap(await api(`/v1/cameras/${camera.id}/snapshots`, { method: "POST" }));
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function ptz(action: string) {
    await api(`/v1/cameras/${camera.id}/ptz`, { method: "POST", body: JSON.stringify({ action }) }).catch(() => {});
  }

  return (
    <div className="fixed inset-0 z-50 bg-black/70 flex items-center justify-center p-4" onClick={onClose}>
      <div className="card w-full max-w-4xl p-4 space-y-3" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between">
          <h3 className="font-semibold">{camera.display_name} — بث مباشر</h3>
          <button onClick={onClose} className="muted hover:text-current" aria-label="إغلاق">✕</button>
        </div>
        <div className="relative aspect-video bg-black rounded-lg overflow-hidden">
          <video ref={video} autoPlay playsInline muted className="w-full h-full object-contain" />
          {state !== "playing" && (
            <div className="absolute inset-0 flex items-center justify-center text-slate-300 text-sm text-center p-6">
              {state === "connecting" ? "جارٍ الاتصال بجهاز الموقع…" : `تعذّر بدء البث: ${error}`}
            </div>
          )}
        </div>
        <div className="flex flex-wrap gap-2 items-center">
          <button onClick={snapshot} className="rounded-lg bg-brand-600 hover:bg-brand-700 text-white px-4 py-2 text-sm">
            لقطة موقّعة
          </button>
          {camera.ptz &&
            [
              ["MOVE_LEFT", "◀"],
              ["MOVE_UP", "▲"],
              ["MOVE_DOWN", "▼"],
              ["MOVE_RIGHT", "▶"],
              ["ZOOM_IN", "+"],
              ["ZOOM_OUT", "−"],
            ].map(([a, l]) => (
              <button
                key={a}
                onMouseDown={() => ptz(a)}
                onMouseUp={() => ptz("STOP")}
                className="rounded-lg border border-[var(--border)] px-3 py-2 text-sm"
              >
                {l}
              </button>
            ))}
          {snap && (
            <span className="muted text-xs" dir="ltr">
              SHA-256 {snap.sha256.slice(0, 16)}…
            </span>
          )}
        </div>
      </div>
    </div>
  );
}
