"use client";

import maplibregl from "maplibre-gl";
import { useEffect, useRef } from "react";
import { label as labelAR, thumbUrl, type Camera, type CameraEvent, type Site } from "@/lib/api";
import { fmtRelative } from "@/lib/format";

const STYLE = process.env.NEXT_PUBLIC_MAP_STYLE ?? "https://tiles.openfreemap.org/styles/liberty";

// Used when the tile server cannot be reached, so sites and cameras still show.
const FALLBACK_STYLE: maplibregl.StyleSpecification = {
  version: 8,
  sources: {},
  layers: [{ id: "bg", type: "background", paint: { "background-color": "#dfe5ea" } }],
};

// viewCone returns the polygon a camera sees: its position, heading and
// field of view, out to a fixed range in metres.
export function viewCone(lng: number, lat: number, heading = 0, fov = 90, rangeM = 25): [number, number][] {
  const mPerDegLat = 111_320;
  const mPerDegLng = 111_320 * Math.cos((lat * Math.PI) / 180);
  const pts: [number, number][] = [[lng, lat]];
  const steps = 14;
  for (let i = 0; i <= steps; i++) {
    const a = ((heading - fov / 2 + (fov * i) / steps) * Math.PI) / 180;
    pts.push([lng + (Math.sin(a) * rangeM) / mPerDegLng, lat + (Math.cos(a) * rangeM) / mPerDegLat]);
  }
  pts.push([lng, lat]);
  return pts;
}

type Props = {
  sites: Site[];
  cameras?: Camera[];
  // Site ids with a critical event in the last hour; their markers pulse.
  critical?: Set<string>;
  // The newest critical event per camera, shown as a bubble at the camera.
  alerts?: Record<string, CameraEvent>;
  onSelectSite?: (id: string) => void;
  onSelectCamera?: (id: string) => void;
  // When set, the camera with this id is draggable and the callback gets the
  // new position.
  editable?: { cameraId: string; onMove: (lng: number, lat: number) => void };
  className?: string;
  fit?: boolean;
};

export default function SiteMap({ sites, cameras = [], critical, alerts = {}, onSelectSite, onSelectCamera, editable, className, fit = true }: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const markers = useRef<Map<string, maplibregl.Marker>>(new Map());
  const didFit = useRef(false);
  const cbs = useRef({ onSelectSite, onSelectCamera, editable });
  cbs.current = { onSelectSite, onSelectCamera, editable };

  useEffect(() => {
    if (!ref.current) return;
    const map = new maplibregl.Map({
      container: ref.current,
      style: STYLE,
      center: [45.0, 24.0],
      zoom: 4.3,
      attributionControl: { compact: true },
    });
    map.addControl(new maplibregl.NavigationControl({ showCompass: false }), "top-left");
    let fellBack = false;
    map.on("error", (e) => {
      if (!fellBack && !map.isStyleLoaded() && String(e.error?.message ?? "").includes(STYLE)) {
        fellBack = true;
        map.setStyle(FALLBACK_STYLE);
      }
    });
    mapRef.current = map;
    return () => {
      markers.current.forEach((m) => m.remove());
      markers.current.clear();
      map.remove();
    };
  }, []);

  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;

    const placed = cameras.filter((c) => c.lat != null && c.lng != null);
    // state drives the cone colour: editing (blue), alert (red), online
    // (green), offline (grey).
    const stateOf = (c: Camera) => (editable?.cameraId === c.id ? "editing" : alerts[c.id] ? "alert" : c.status === "online" ? "online" : "offline");
    const coneFeatures = placed.map((c) => ({
      type: "Feature" as const,
      properties: { id: c.id, state: stateOf(c) },
      geometry: { type: "Polygon" as const, coordinates: [viewCone(c.lng!, c.lat!, c.heading_deg ?? 0, c.fov_deg ?? 90)] },
    }));

    const apply = () => {
      const data = { type: "FeatureCollection", features: coneFeatures } as GeoJSON.FeatureCollection;
      const src = map.getSource("cones") as maplibregl.GeoJSONSource | undefined;
      if (src) src.setData(data);
      else map.addSource("cones", { type: "geojson", data });
      if (!map.getLayer("cones")) {
        map.addLayer({
          id: "cones",
          type: "fill",
          source: "cones",
          paint: {
            "fill-color": ["match", ["get", "state"], "editing", "#3b82f6", "alert", "#e11d48", "online", "#0f9f6e", "#94a3b8"],
            "fill-opacity": ["match", ["get", "state"], "editing", 0.4, "alert", 0.34, "offline", 0.14, 0.22],
          },
        });
        map.addLayer({
          id: "cones-line",
          type: "line",
          source: "cones",
          paint: {
            "line-color": ["match", ["get", "state"], "editing", "#2563eb", "alert", "#be123c", "online", "#0b7f58", "#64748b"],
            "line-width": ["match", ["get", "state"], "alert", 1.6, 1],
            "line-opacity": 0.7,
            "line-dasharray": ["match", ["get", "state"], "offline", ["literal", [2, 2]], ["literal", [1, 0]]],
          },
        });
      }

      // DOM markers for sites and cameras: they can pulse and be dragged.
      const wanted = new Set<string>();
      const upsert = (key: string, lngLat: [number, number], make: () => HTMLElement, opts?: maplibregl.MarkerOptions) => {
        wanted.add(key);
        let m = markers.current.get(key);
        if (!m) {
          m = new maplibregl.Marker({ element: make(), ...opts }).setLngLat(lngLat).addTo(map);
          markers.current.set(key, m);
        } else {
          m.setLngLat(lngLat);
          const el = m.getElement();
          const fresh = make();
          el.className = fresh.className;
          el.title = fresh.title;
          el.style.cssText = fresh.style.cssText;
          if (fresh.innerHTML !== el.innerHTML) el.innerHTML = fresh.innerHTML;
          m.setDraggable(!!opts?.draggable);
        }
        return m;
      };

      for (const s of sites) {
        if (s.lat == null || s.lng == null) continue;
        const health = s.cameras === 0 ? "empty" : s.cameras_online === s.cameras ? "ok" : "degraded";
        const m = upsert("s" + s.id, [s.lng, s.lat], () => {
          const el = document.createElement("div");
          el.className = `map-marker ${health} ${critical?.has(s.id) ? "critical" : ""}`;
          el.style.position = "relative";
          el.title = s.name;
          el.onclick = () => cbs.current.onSelectSite?.(s.id);
          return el;
        });
        if (!m.getPopup()) m.setPopup(new maplibregl.Popup({ offset: 14, closeButton: false }).setText(s.name));
      }
      for (const c of placed) {
        const editing = editable?.cameraId === c.id;
        const alert = alerts[c.id];
        const colour = editing ? "#3b82f6" : alert ? "#e11d48" : c.status === "online" ? "#0b7f58" : "#64748b";
        const m = upsert(
          "c" + c.id,
          [c.lng!, c.lat!],
          () => {
            const el = document.createElement("div");
            el.className = `map-marker ${alert ? "critical" : ""}`;
            el.style.cssText = `width:12px;height:12px;border-width:2px;background:${colour};${editing ? "cursor:grab;width:16px;height:16px" : ""}`;
            el.title = c.display_name;
            el.onclick = () => !editing && cbs.current.onSelectCamera?.(c.id);
            return el;
          },
          { draggable: editing },
        );
        // A bubble above the camera with the alert and its thumbnail; it
        // appears once and stays until the event is acknowledged.
        if (alert && !editing) {
          upsert(
            "a" + c.id,
            [c.lng!, c.lat!],
            () => {
              const el = document.createElement("button");
              el.type = "button";
              el.className = "map-bubble";
              el.innerHTML = `<img alt="" src="${thumbUrl(c.id)}" onerror="this.style.display='none'" /><span class="map-bubble-text"><b>${labelAR(alert.label)}</b><small>${c.display_name} · ${fmtRelative(alert.time)}</small></span>`;
              el.title = labelAR(alert.label);
              el.onclick = (e) => {
                e.stopPropagation();
                cbs.current.onSelectCamera?.(c.id);
              };
              return el;
            },
            { anchor: "bottom", offset: [0, -12] },
          );
        }
        if (editing) {
          m.off("dragend", () => {});
          m.on("dragend", () => {
            const p = m.getLngLat();
            cbs.current.editable?.onMove(p.lng, p.lat);
          });
        }
      }
      for (const [key, m] of markers.current) {
        if (!wanted.has(key)) {
          m.remove();
          markers.current.delete(key);
        }
      }

      const points = [
        ...sites.filter((s) => s.lat != null).map((s) => [s.lng!, s.lat!] as [number, number]),
        ...placed.map((c) => [c.lng!, c.lat!] as [number, number]),
      ];
      if (fit && !didFit.current && points.length > 0) {
        const bounds = points.reduce((b, p) => b.extend(p), new maplibregl.LngLatBounds(points[0], points[0]));
        map.fitBounds(bounds, { padding: 60, maxZoom: 18, duration: 600 });
        didFit.current = true;
      }
    };

    if (map.isStyleLoaded()) apply();
    else map.once("style.load", apply);
  }, [sites, cameras, critical, alerts, editable, fit]);

  return <div ref={ref} className={className ?? "h-[420px] w-full rounded-xl overflow-hidden"} />;
}
