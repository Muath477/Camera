"use client";

import { motion } from "motion/react";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useRef, useState } from "react";
import { Building2, Cpu, Search, Video, type LucideIcon } from "lucide-react";
import type { Camera, Device, Site } from "@/lib/api";
import { useApi } from "@/lib/hooks";

type Item = { id: string; title: string; hint?: string; href: string; icon: LucideIcon };

export default function CommandPalette({ onClose }: { onClose: () => void }) {
  const router = useRouter();
  const [q, setQ] = useState("");
  const [cursor, setCursor] = useState(0);
  const input = useRef<HTMLInputElement>(null);
  const sites = useApi<{ items: Site[] | null }>("/v1/sites");
  const cameras = useApi<{ items: Camera[] | null }>("/v1/cameras");
  const devices = useApi<{ items: Device[] | null }>("/v1/devices");

  useEffect(() => input.current?.focus(), []);

  const items = useMemo<Item[]>(() => {
    const siteName = Object.fromEntries((sites.data?.items ?? []).map((s) => [s.id, s.name]));
    const all: Item[] = [
      ...(sites.data?.items ?? []).map((s) => ({ id: "s" + s.id, title: s.name, hint: s.address, href: `/sites/${s.id}`, icon: Building2 })),
      ...(cameras.data?.items ?? []).map((c) => ({ id: "c" + c.id, title: c.display_name, hint: siteName[c.site_id], href: `/cameras/${c.id}`, icon: Video })),
      ...(devices.data?.items ?? []).map((d) => ({ id: "d" + d.id, title: d.serial, hint: d.site_id ? siteName[d.site_id] : "غير مرتبط", href: "/devices", icon: Cpu })),
    ];
    const needle = q.trim().toLowerCase();
    if (!needle) return all.slice(0, 8);
    return all.filter((i) => (i.title + " " + (i.hint ?? "")).toLowerCase().includes(needle)).slice(0, 10);
  }, [q, sites.data, cameras.data, devices.data]);

  useEffect(() => setCursor(0), [q]);

  const go = (item: Item) => {
    router.push(item.href);
    onClose();
  };

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      className="fixed inset-0 z-[70] bg-black/50 backdrop-blur-sm flex items-start justify-center p-4 pt-[12vh]"
      onClick={onClose}
    >
      <motion.div
        initial={{ opacity: 0, y: -12, scale: 0.98 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        exit={{ opacity: 0, y: -12, scale: 0.98 }}
        transition={{ type: "spring", stiffness: 500, damping: 38 }}
        className="card w-full max-w-lg overflow-hidden shadow-[var(--shadow-lg)]"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-label="بحث سريع"
      >
        <div className="flex items-center gap-2 px-3 border-b border-[var(--border)]">
          <Search className="size-4 muted" />
          <input
            ref={input}
            value={q}
            onChange={(e) => setQ(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "ArrowDown") setCursor((c) => Math.min(c + 1, items.length - 1));
              else if (e.key === "ArrowUp") setCursor((c) => Math.max(c - 1, 0));
              else if (e.key === "Enter" && items[cursor]) go(items[cursor]);
              else if (e.key === "Escape") onClose();
            }}
            placeholder="اكتب اسم موقع أو كاميرا أو رقم جهاز…"
            className="w-full bg-transparent py-3 text-sm outline-none"
          />
        </div>
        <ul className="max-h-80 overflow-y-auto py-1">
          {items.length === 0 && <li className="muted text-sm text-center py-8">لا توجد نتائج</li>}
          {items.map((it, i) => (
            <li key={it.id}>
              <button
                onMouseEnter={() => setCursor(i)}
                onClick={() => go(it)}
                className={`w-full flex items-center gap-3 px-3 py-2.5 text-right text-sm ${i === cursor ? "bg-brand-50 dark:bg-brand-900/30" : ""}`}
              >
                <it.icon className="size-4 muted shrink-0" />
                <span className="truncate">{it.title}</span>
                {it.hint && <span className="muted text-xs mr-auto truncate max-w-[40%]">{it.hint}</span>}
              </button>
            </li>
          ))}
        </ul>
      </motion.div>
    </motion.div>
  );
}
