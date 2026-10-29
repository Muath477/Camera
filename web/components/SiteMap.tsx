"use client";

import maplibregl from "maplibre-gl";
import { useEffect, useRef } from "react";
import type { Camera, Site } from "@/lib/api";

const STYLE = process.env.NEXT_PUBLIC_MAP_STYLE ?? "https://tiles.openfreemap.org/styles/liberty";

// Used when the tile server cannot be reached, so sites and cameras still show.
const FALLBACK_STYLE: maplibregl.StyleSpecification = {
  version: 8,
  sources: {},
  layers: [{ id: "bg", type: "background", paint: { "background-color": "#dfe5ea" } }],
};

// viewCone returns the polygon a camera sees: its position, heading and
// field of view, out to a fixed range in metres.
function viewCone(c: Camera, rangeM = 25): [number, number][] {
  const fov = c.fov_deg ?? 90;
  const heading = c.heading_deg ?? 0;
  const lat = c.lat!;
  const lng = c.lng!;
  const mPerDegLat = 111_320;
  const mPerDegLng = 111_320 * Math.cos((lat * Math.PI) / 180);
  const pts: [number, number][] = [[lng, lat]];
  const steps = 12;
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
  onSelectSite?: (id: string) => void;
  className?: string;
};

export default function SiteMap({ sites, cameras = [], onSelectSite, className }: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);

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
    return () => map.remove();
  }, []);

  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;

    const siteFeatures = sites
      .filter((s) => s.lat != null && s.lng != null)
      .map((s) => ({
        type: "Feature" as const,
        properties: {
          id: s.id,
          name: s.name,
          health: s.cameras === 0 ? "empty" : s.cameras_online === s.cameras ? "ok" : "degraded",
        },
        geometry: { type: "Point" as const, coordinates: [s.lng!, s.lat!] },
      }));
    const placed = cameras.filter((c) => c.lat != null && c.lng != null);
    const coneFeatures = placed.map((c) => ({
      type: "Feature" as const,
      properties: { id: c.id, name: c.display_name, online: c.status === "online" },
      geometry: { type: "Polygon" as const, coordinates: [viewCone(c)] },
    }));
    const camFeatures = placed.map((c) => ({
      type: "Feature" as const,
      properties: { id: c.id, name: c.display_name, online: c.status === "online" },
      geometry: { type: "Point" as const, coordinates: [c.lng!, c.lat!] },
    }));

    const apply = () => {
      const set = (id: string, features: unknown[]) => {
        const data = { type: "FeatureCollection", features } as GeoJSON.FeatureCollection;
        const src = map.getSource(id) as maplibregl.GeoJSONSource | undefined;
        if (src) src.setData(data);
        else map.addSource(id, { type: "geojson", data });
      };
      set("sites", siteFeatures);
      set("cones", coneFeatures);
      set("cams", camFeatures);

      if (!map.getLayer("cones")) {
        map.addLayer({
          id: "cones",
          type: "fill",
          source: "cones",
          paint: {
            "fill-color": ["case", ["get", "online"], "#0f9f6e", "#dc2626"],
            "fill-opacity": 0.25,
          },
        });
        map.addLayer({
          id: "cams",
          type: "circle",
          source: "cams",
          paint: {
            "circle-radius": 5,
            "circle-color": ["case", ["get", "online"], "#0b7f58", "#dc2626"],
            "circle-stroke-color": "#fff",
            "circle-stroke-width": 1.5,
          },
        });
        map.addLayer({
          id: "sites",
          type: "circle",
          source: "sites",
          maxzoom: 15,
          paint: {
            "circle-radius": 9,
            "circle-color": ["match", ["get", "health"], "ok", "#0f9f6e", "degraded", "#f59e0b", "#64748b"],
            "circle-stroke-color": "#fff",
            "circle-stroke-width": 2,
          },
        });
        map.on("click", "sites", (e) => {
          const id = e.features?.[0]?.properties?.id;
          if (id && onSelectSite) onSelectSite(String(id));
        });
        map.on("mouseenter", "sites", () => (map.getCanvas().style.cursor = "pointer"));
        map.on("mouseleave", "sites", () => (map.getCanvas().style.cursor = ""));
        const popup = new maplibregl.Popup({ closeButton: false, closeOnClick: false });
        map.on("mousemove", "cams", (e) => {
          const f = e.features?.[0];
          if (!f) return;
          popup.setLngLat(e.lngLat).setText(String(f.properties?.name)).addTo(map);
        });
        map.on("mouseleave", "cams", () => popup.remove());
      }

      const points = [...siteFeatures, ...camFeatures].map((f) => f.geometry.coordinates as [number, number]);
      if (points.length > 0) {
        const bounds = points.reduce((b, p) => b.extend(p), new maplibregl.LngLatBounds(points[0], points[0]));
        map.fitBounds(bounds, { padding: 60, maxZoom: 18, duration: 0 });
      }
    };

    if (map.isStyleLoaded()) apply();
    else map.once("style.load", apply);
  }, [sites, cameras, onSelectSite]);

  return <div ref={ref} className={className ?? "h-[420px] w-full rounded-xl overflow-hidden"} />;
}
