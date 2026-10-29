"use client";

import { useCallback, useEffect, useState } from "react";
import { api, eventStreamUrl, type CameraEvent } from "./api";

// useApi loads a path and reloads when it changes.
export function useApi<T>(path: string | null) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    if (!path) return;
    setLoading(true);
    try {
      setData(await api<T>(path));
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [path]);

  useEffect(() => {
    load();
  }, [load]);

  return { data, error, loading, reload: load };
}

// useEventStream calls onEvent for every new event pushed by the server.
export function useEventStream(onEvent: (e: CameraEvent) => void) {
  useEffect(() => {
    const es = new EventSource(eventStreamUrl());
    const handler = (m: MessageEvent) => {
      try {
        onEvent(JSON.parse(m.data));
      } catch {
        /* ignore malformed */
      }
    };
    es.addEventListener("event", handler);
    return () => {
      es.removeEventListener("event", handler);
      es.close();
    };
  }, [onEvent]);
}
