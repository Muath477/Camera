"use client";

import { AnimatePresence, motion } from "motion/react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import {
  Activity,
  Bell,
  Cpu,
  Download,
  LayoutDashboard,
  LogOut,
  Monitor,
  Moon,
  Search,
  Settings,
  ShieldCheck,
  Sun,
  Video,
  X,
} from "lucide-react";
import { getToken, setToken, type Me } from "@/lib/api";
import { useApi } from "@/lib/hooks";
import { useTheme } from "@/lib/theme";
import CommandPalette from "./CommandPalette";
import { LiveProvider, useLive } from "./Live";
import { PWAProvider, usePWA } from "./PWA";
import { ToastProvider } from "./Toast";

const NAV = [
  { href: "/", label: "الرئيسية", icon: LayoutDashboard },
  { href: "/cameras", label: "الكاميرات", icon: Video },
  { href: "/events", label: "الأحداث", icon: Bell },
  { href: "/compliance", label: "الامتثال", icon: ShieldCheck },
  { href: "/devices", label: "الأجهزة", icon: Cpu },
];

function ThemeButton() {
  const { theme, cycle } = useTheme();
  const Icon = theme === "dark" ? Moon : theme === "light" ? Sun : Monitor;
  const title = theme === "dark" ? "الوضع الداكن" : theme === "light" ? "الوضع الفاتح" : "حسب النظام";
  return (
    <button onClick={cycle} title={title} aria-label={title} className="size-9 rounded-lg hover:bg-white/10 flex items-center justify-center text-[var(--sidebar-text)]">
      <Icon className="size-4" />
    </button>
  );
}

function LiveDot() {
  const { connected } = useLive();
  return (
    <span className="flex items-center gap-2 text-xs text-[var(--sidebar-text)]">
      <span className={`pulse-dot ${connected ? "" : "bad"}`} />
      {connected ? "متصل مباشرة" : "إعادة الاتصال…"}
    </span>
  );
}

function Sidebar({ me, onLogout }: { me: Me | null; onLogout: () => void }) {
  const pathname = usePathname();
  const active = (href: string) => (href === "/" ? pathname === "/" : pathname.startsWith(href));
  return (
    <aside className="hidden md:flex md:w-64 shrink-0 flex-col bg-[var(--sidebar)] text-slate-100 md:min-h-screen sticky top-0 h-screen">
      <div className="px-5 pt-5 pb-4 flex items-center justify-between">
        <Link href="/" className="flex items-center gap-2.5">
          <span className="size-9 rounded-xl bg-brand-500 flex items-center justify-center shadow-[0_0_0_4px_rgba(15,159,110,0.18)]">
            <Video className="size-5 text-white" />
          </span>
          <span className="text-lg font-bold tracking-tight">Camera</span>
        </Link>
        <ThemeButton />
      </div>
      <nav className="px-3 space-y-0.5 mt-2">
        {NAV.map(({ href, label, icon: Icon }) => {
          const on = active(href);
          return (
            <Link
              key={href}
              href={href}
              className={`relative flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm transition-colors ${on ? "text-white" : "text-[var(--sidebar-text)] hover:text-white hover:bg-white/5"}`}
            >
              {on && (
                <motion.span
                  layoutId="nav-active"
                  className="absolute inset-0 rounded-lg bg-brand-600"
                  transition={{ type: "spring", stiffness: 500, damping: 40 }}
                />
              )}
              <Icon className="size-[18px] relative" />
              <span className="relative font-medium">{label}</span>
            </Link>
          );
        })}
      </nav>
      <div className="mt-auto p-4 space-y-3 border-t border-white/10">
        <LiveDot />
        {me && (
          <div className="flex items-center gap-3">
            <span className="size-9 rounded-full bg-white/10 flex items-center justify-center text-sm font-semibold">{me.name.slice(0, 1)}</span>
            <div className="min-w-0 flex-1">
              <div className="text-sm font-medium truncate">{me.name}</div>
              <div className="text-xs text-[var(--sidebar-text)] truncate">{me.tenant_name}</div>
            </div>
            <Link href="/settings" className="text-[var(--sidebar-text)] hover:text-white" aria-label="الإعدادات" title="الإعدادات">
              <Settings className="size-4" />
            </Link>
            <button onClick={onLogout} className="text-[var(--sidebar-text)] hover:text-white" aria-label="تسجيل الخروج" title="تسجيل الخروج">
              <LogOut className="size-4" />
            </button>
          </div>
        )}
      </div>
    </aside>
  );
}

function MobileNav() {
  const pathname = usePathname();
  const active = (href: string) => (href === "/" ? pathname === "/" : pathname.startsWith(href));
  return (
    <nav className="md:hidden fixed bottom-0 inset-x-0 z-40 bg-[var(--bg-elevated)]/95 backdrop-blur border-t border-[var(--border)] grid grid-cols-5 pb-[env(safe-area-inset-bottom)]">
      {NAV.map(({ href, label, icon: Icon }) => {
        const on = active(href);
        return (
          <Link key={href} href={href} className={`flex flex-col items-center gap-1 py-2 text-[11px] ${on ? "text-brand-600" : "muted"}`}>
            <span className="relative">
              {on && <motion.span layoutId="mnav" className="absolute -inset-x-3 -inset-y-1 rounded-full bg-brand-100 dark:bg-brand-900/50" />}
              <Icon className="size-5 relative" />
            </span>
            {label}
          </Link>
        );
      })}
    </nav>
  );
}

function TopBar({ me, onSearch, onLogout }: { me: Me | null; onSearch: () => void; onLogout: () => void }) {
  const { criticalCount } = useLive();
  const { canInstall, install } = usePWA();
  return (
    <header className="sticky top-0 z-30 bg-[var(--bg)]/80 backdrop-blur border-b border-[var(--border)] md:border-0 md:bg-transparent">
      <div className="flex items-center gap-2 px-4 py-2.5 md:px-8 md:py-4 max-w-7xl">
        <Link href="/" className="md:hidden flex items-center gap-2 font-bold">
          <span className="size-8 rounded-lg bg-brand-500 flex items-center justify-center">
            <Video className="size-4 text-white" />
          </span>
          Camera
        </Link>
        <button
          onClick={onSearch}
          className="mr-auto md:mr-0 flex items-center gap-2 h-9 px-3 rounded-lg border border-[var(--border)] bg-[var(--surface)] text-sm muted hover:border-[var(--border-strong)] w-auto md:w-72"
        >
          <Search className="size-4" />
          <span className="hidden sm:inline">ابحث عن موقع أو كاميرا…</span>
          <kbd className="hidden md:inline mr-auto text-[10px] border border-[var(--border)] rounded px-1" dir="ltr">
            Ctrl K
          </kbd>
        </button>
        <Link href="/events" className="relative size-9 rounded-lg hover:bg-[var(--surface-2)] flex items-center justify-center" aria-label="الأحداث">
          <Activity className="size-4" />
          <AnimatePresence>
            {criticalCount > 0 && (
              <motion.span
                key={criticalCount}
                initial={{ scale: 0.4 }}
                animate={{ scale: 1 }}
                className="absolute -top-0.5 -left-0.5 min-w-[18px] h-[18px] px-1 rounded-full bg-bad-500 text-white text-[10px] font-bold flex items-center justify-center"
              >
                {criticalCount}
              </motion.span>
            )}
          </AnimatePresence>
        </Link>
        {canInstall && (
          <button onClick={install} className="hidden sm:flex items-center gap-1.5 h-9 px-3 rounded-lg border border-brand-300 text-brand-700 dark:text-brand-300 dark:border-brand-700 text-sm hover:bg-brand-50 dark:hover:bg-brand-900/30" title="ثبّت التطبيق على جهازك">
            <Download className="size-4" /> تثبيت التطبيق
          </button>
        )}
        <div className="md:hidden">
          <ThemeButtonLight />
        </div>
        <Link href="/settings" className="md:hidden size-9 rounded-lg hover:bg-[var(--surface-2)] flex items-center justify-center" aria-label="الإعدادات">
          <Settings className="size-4" />
        </Link>
        {me && (
          <button onClick={onLogout} className="md:hidden size-9 rounded-lg hover:bg-[var(--surface-2)] flex items-center justify-center" aria-label="تسجيل الخروج">
            <LogOut className="size-4" />
          </button>
        )}
      </div>
    </header>
  );
}

function ThemeButtonLight() {
  const { theme, cycle } = useTheme();
  const Icon = theme === "dark" ? Moon : theme === "light" ? Sun : Monitor;
  return (
    <button onClick={cycle} className="size-9 rounded-lg hover:bg-[var(--surface-2)] flex items-center justify-center" aria-label="المظهر">
      <Icon className="size-4" />
    </button>
  );
}

export default function Shell({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const [ready, setReady] = useState(false);
  const [palette, setPalette] = useState(false);

  useEffect(() => {
    if (!getToken()) router.replace("/login");
    else setReady(true);
  }, [router]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setPalette((v) => !v);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const { data: me } = useApi<Me>(ready ? "/v1/me" : null);
  const logout = () => {
    setToken(null);
    router.replace("/login");
  };

  if (!ready) return null;

  return (
    <ToastProvider>
      <PWAProvider>
      <LiveProvider>
        <div className="min-h-screen md:flex">
          <Sidebar me={me} onLogout={logout} />
          <div className="flex-1 min-w-0 flex flex-col">
            <TopBar me={me} onSearch={() => setPalette(true)} onLogout={logout} />
            <main className="px-4 pt-3 pb-24 md:pt-0 md:pb-10 md:px-8 max-w-7xl w-full flex-1">{children}</main>
          </div>
          <MobileNav />
        </div>
        <AnimatePresence>{palette && <CommandPalette onClose={() => setPalette(false)} />}</AnimatePresence>
      </LiveProvider>
      </PWAProvider>
    </ToastProvider>
  );
}

export { X };
