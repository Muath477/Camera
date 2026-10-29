"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { getToken, setToken, type Me } from "@/lib/api";
import { useApi } from "@/lib/hooks";

const NAV = [
  { href: "/", label: "الرئيسية" },
  { href: "/events", label: "الأحداث" },
  { href: "/compliance", label: "الامتثال" },
  { href: "/devices", label: "الأجهزة" },
];

export default function Shell({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [ready, setReady] = useState(false);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!getToken()) router.replace("/login");
    else setReady(true);
  }, [router]);

  const { data: me } = useApi<Me>(ready ? "/v1/me" : null);

  if (!ready) return null;

  const active = (href: string) => (href === "/" ? pathname === "/" : pathname.startsWith(href));

  return (
    <div className="min-h-screen md:flex">
      <aside
        className={`${open ? "block" : "hidden"} md:block md:w-60 shrink-0 bg-slate-900 text-slate-100 md:min-h-screen`}
      >
        <div className="px-5 py-5 text-xl font-bold tracking-tight">Camera</div>
        <nav className="px-3 space-y-1">
          {NAV.map((n) => (
            <Link
              key={n.href}
              href={n.href}
              onClick={() => setOpen(false)}
              className={`block rounded-lg px-3 py-2 text-sm ${
                active(n.href) ? "bg-brand-600 text-white" : "text-slate-300 hover:bg-slate-800"
              }`}
            >
              {n.label}
            </Link>
          ))}
        </nav>
        {me && (
          <div className="mt-8 px-5 text-xs text-slate-400 space-y-1">
            <div className="text-slate-200 text-sm">{me.tenant_name}</div>
            <div>{me.name}</div>
            <button
              className="underline hover:text-white"
              onClick={() => {
                setToken(null);
                router.replace("/login");
              }}
            >
              تسجيل الخروج
            </button>
          </div>
        )}
      </aside>
      <div className="flex-1 min-w-0">
        <header className="md:hidden flex items-center justify-between px-4 py-3 bg-slate-900 text-white">
          <span className="font-bold">Camera</span>
          <button onClick={() => setOpen(!open)} aria-label="القائمة" className="px-2 py-1 border border-slate-600 rounded">
            ☰
          </button>
        </header>
        <main className="p-4 md:p-8 max-w-7xl">{children}</main>
      </div>
    </div>
  );
}
