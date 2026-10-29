"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { eventStreamUrl, label, type CameraEvent } from "@/lib/api";
import { useToast } from "./Toast";

type LiveCtx = {
  events: CameraEvent[];
  connected: boolean;
  // Number of critical events seen since the page loaded.
  criticalCount: number;
};

const Ctx = createContext<LiveCtx>({ events: [], connected: false, criticalCount: 0 });

export const useLive = () => useContext(Ctx);

// LiveProvider holds one server-sent events connection for the whole app.
// Pages read recent events from it; critical ones also raise a toast.
export function LiveProvider({ children }: { children: ReactNode }) {
  const [events, setEvents] = useState<CameraEvent[]>([]);
  const [connected, setConnected] = useState(false);
  const [criticalCount, setCritical] = useState(0);
  const toast = useToast();
  const toastRef = useRef(toast);
  toastRef.current = toast;

  const onEvent = useCallback((ev: CameraEvent) => {
    setEvents((prev) => {
      const exists = prev.some((p) => p.device_id === ev.device_id && p.edge_id === ev.edge_id);
      const next = exists ? prev.map((p) => (p.device_id === ev.device_id && p.edge_id === ev.edge_id ? ev : p)) : [ev, ...prev];
      return next.slice(0, 50);
    });
    if (ev.severity === "critical" && !ev.end_time) {
      setCritical((n) => n + 1);
      toastRef.current.push({
        tone: "critical",
        title: label(ev.label),
        body: ev.camera_name ?? undefined,
        href: `/sites/${ev.site_id}`,
        ttl: 9000,
      });
    }
  }, []);

  useEffect(() => {
    let es: EventSource | null = null;
    let retry: ReturnType<typeof setTimeout> | null = null;
    const connect = () => {
      es = new EventSource(eventStreamUrl());
      es.onopen = () => setConnected(true);
      es.onerror = () => setConnected(false);
      es.addEventListener("event", (m: MessageEvent) => {
        try {
          onEvent(JSON.parse(m.data));
        } catch {
          /* ignore */
        }
      });
    };
    connect();
    return () => {
      es?.close();
      if (retry) clearTimeout(retry);
    };
  }, [onEvent]);

  const value = useMemo(() => ({ events, connected, criticalCount }), [events, connected, criticalCount]);
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}
