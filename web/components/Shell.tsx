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
import Mascot from "./Mascot";
import { LiveProvider, useLive } from "./Live";
import { PWAProvider, usePWA } from "./PWA";
import { ToastProvider } from "./Toast";

// Each section keeps its own sticker colour, in the sidebar and on its pages.
const NAV = [
  { href: "/", label: "الرئيسية", icon: LayoutDashboard, chip: "bg-brand-400" },
  { href: "/cameras", label: "الكاميرات", icon: Video, chip: "bg-aqua-400" },
  { href: "/events", label: "الأحداث", icon: Bell, chip: "bg-pop-400" },
  { href: "/compliance", label: "الامتثال", icon: ShieldCheck, chip: "bg-sun-400" },
  { href: "/devices", label: "الأجهزة", icon: Cpu, chip: "bg-lilac-400" },
];

function ThemeButton({ dark }: { dark?: boolean }) {
  const { theme, cycle } = useTheme();
  const Icon = theme === "dark" ? Moon : theme === "light" ? Sun : Monitor;
  const title = theme === "dark" ? "المظهر: داكن" : theme === "light" ? "المظهر: فاتح" : "المظهر: حسب الجهاز";
  return (
    <button
      onClick={cycle}
      title={title}
      aria-label={`${title}، اضغط للتبديل`}
      className={`group size-11 rounded-full flex items-center justify-center transition-colors ${dark ? "text-[var(--sidebar-text)] hover:bg-white/10 hover:text-sun-300" : "hover:bg-[var(--surface-2)]"}`}
    >
      <Icon className="size-[18px] wiggle" />
    </button>
  );
}

function Brand({ onDark }: { onDark?: boolean }) {
  return (
    <Link href="/" className="group flex items-center gap-2 rounded-xl" aria-label="Camera، الرئيسية">
      <Mascot size={onDark ? 44 : 38} className="transition-transform duration-300 group-hover:-rotate-6" />
      <span className={`font-display font-extrabold tracking-tight ${onDark ? "text-2xl text-white" : "text-xl"}`}>Camera</span>
    </Link>
  );
}

function LiveDot() {
  const { connected } = useLive();
  return (
    <span className="flex items-center gap-2 text-xs text-[var(--sidebar-text)]" role="status">
      <span className={`pulse-dot ${connected ? "" : "bad"}`} />
      {connected ? "متصل، الأحداث تصل لحظياً" : "انقطع الاتصال، نعيد المحاولة…"}
    </span>
  );
}

function Sidebar({ me, onLogout }: { me: Me | null; onLogout: () => void }) {
  const pathname = usePathname();
  const active = (href: string) => (href === "/" ? pathname === "/" : pathname.startsWith(href));
  return (
    <aside className="hidden md:flex md:w-[17rem] shrink-0 p-3 sticky top-0 h-screen">
      <div className="flex flex-col flex-1 rounded-[var(--radius-xl)] bg-[var(--sidebar)] text-white border-2 border-[var(--line)] shadow-[var(--pop)] overflow-hidden">
        <div className="px-4 pt-4 pb-3 flex items-center justify-between">
          <Brand onDark />
          <ThemeButton dark />
        </div>
        <nav aria-label="الأقسام" className="px-3 space-y-1 mt-3">
          {NAV.map(({ href, label, icon: Icon, chip }) => {
            const on = active(href);
            return (
              <Link
                key={href}
                href={href}
                aria-current={on ? "page" : undefined}
                className={`group relative flex items-center gap-3 rounded-2xl px-2.5 py-2 text-[15px] transition-colors ${on ? "text-ink-900" : "text-[var(--sidebar-text)] hover:text-white hover:bg-white/5"}`}
              >
                {on && (
                  <motion.span
                    layoutId="nav-active"
                    className="absolute inset-0 rounded-2xl bg-[var(--surface)] border-2 border-[var(--line)]"
                    transition={{ type: "spring", stiffness: 520, damping: 34 }}
                  />
                )}
                <span className={`relative size-9 rounded-xl flex items-center justify-center text-ink-900 border-2 ${on ? `${chip} border-[var(--line)] -rotate-6` : "border-transparent bg-white/10 text-[var(--sidebar-text)] group-hover:text-white"} transition-transform`}>
                  <Icon className={`size-[18px] wiggle ${on ? "text-ink-900" : ""}`} />
                </span>
                <span className={`relative ${on ? "font-bold text-[var(--text)]" : "font-medium"}`}>{label}</span>
              </Link>
            );
          })}
        </nav>
        <div className="mt-auto m-3 p-3 space-y-3 rounded-2xl bg-[var(--sidebar-2)]">
          <LiveDot />
          {me && (
            <div className="flex items-center gap-2">
              <span className="size-10 shrink-0 rounded-full bg-sun-400 text-ink-900 border-2 border-[var(--line)] flex items-center justify-center font-display font-bold">{me.name.slice(0, 1)}</span>
              <div className="min-w-0 flex-1">
                <div className="text-sm font-semibold truncate">{me.name}</div>
                <div className="text-xs text-[var(--sidebar-text)] truncate">{me.tenant_name}</div>
              </div>
              <Link href="/settings" className="group size-9 rounded-full flex items-center justify-center text-[var(--sidebar-text)] hover:text-white hover:bg-white/10" aria-label="الإعدادات" title="الإعدادات">
                <Settings className="size-4 wiggle" />
              </Link>
              <button onClick={onLogout} className="group size-9 rounded-full flex items-center justify-center text-[var(--sidebar-text)] hover:text-white hover:bg-white/10" aria-label="تسجيل الخروج" title="تسجيل الخروج">
                <LogOut className="size-4 wiggle" />
              </button>
            </div>
          )}
        </div>
      </div>
    </aside>
  );
}

function MobileNav() {
  const pathname = usePathname();
  const active = (href: string) => (href === "/" ? pathname === "/" : pathname.startsWith(href));
  return (
    <nav aria-label="الأقسام" className="md:hidden fixed bottom-3 inset-x-3 z-40 rounded-[1.75rem] bg-[var(--sidebar)] border-2 border-[var(--line)] shadow-[var(--pop)] grid grid-cols-5 p-1.5 mb-[env(safe-area-inset-bottom)]">
      {NAV.map(({ href, label, icon: Icon, chip }) => {
        const on = active(href);
        return (
          <Link key={href} href={href} aria-current={on ? "page" : undefined} className={`relative flex flex-col items-center gap-0.5 py-1.5 min-h-[52px] justify-center rounded-2xl text-[11px] ${on ? "text-ink-900 font-bold" : "text-[var(--sidebar-text)]"}`}>
            {on && <motion.span layoutId="mnav" className={`absolute inset-0 rounded-2xl ${chip} border-2 border-[var(--line)]`} transition={{ type: "spring", stiffness: 520, damping: 34 }} />}
            <Icon className="size-5 relative" />
            <span className="relative">{label}</span>
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
    <header className="sticky top-0 z-30 bg-[var(--bg)]/90 backdrop-blur md:bg-transparent md:backdrop-blur-none">
      <div className="flex items-center gap-1.5 px-4 py-2.5 md:px-8 md:py-5 max-w-7xl">
        <span className="md:hidden">
          <Brand />
        </span>
        <button
          onClick={onSearch}
          aria-label="ابحث عن موقع أو كاميرا"
          className="group ms-auto md:ms-0 flex items-center gap-2 h-11 px-4 rounded-full border-2 border-[var(--card-line)] bg-[var(--surface)] text-sm muted hover:text-[var(--text)] shadow-[var(--pop-sm)] transition-transform hover:-translate-y-0.5 w-auto md:w-80"
        >
          <Search className="size-4 wiggle" />
          <span className="hidden sm:inline">ابحث عن موقع أو كاميرا…</span>
          <kbd className="hidden md:inline ms-auto text-[10px] font-mono border-[1.5px] border-[var(--border-strong)] rounded-md px-1.5 py-0.5" dir="ltr">
            Ctrl K
          </kbd>
        </button>
        <Link href="/events" className="group relative size-11 rounded-full hover:bg-[var(--surface-2)] flex items-center justify-center" aria-label={criticalCount > 0 ? `الأحداث، ${criticalCount} تنبيه حرج` : "الأحداث"}>
          <Activity className="size-[18px] wiggle" />
          <AnimatePresence>
            {criticalCount > 0 && (
              <motion.span
                key={criticalCount}
                initial={{ scale: 0.3, rotate: -30 }}
                animate={{ scale: 1, rotate: 0 }}
                transition={{ type: "spring", stiffness: 600, damping: 14 }}
                className="absolute top-0 end-0 min-w-[20px] h-[20px] px-1 rounded-full bg-bad-500 text-white border-2 border-[var(--line)] text-[10px] font-bold flex items-center justify-center"
              >
                {criticalCount}
              </motion.span>
            )}
          </AnimatePresence>
        </Link>
        {canInstall && (
          <button onClick={install} className="hidden sm:flex btn-pop items-center gap-1.5 h-11 px-4 rounded-full bg-sun-300 text-ink-900 text-sm font-semibold" title="ثبّت التطبيق على جهازك">
            <Download className="size-4" /> ثبّت التطبيق
          </button>
        )}
        <div className="md:hidden">
          <ThemeButton />
        </div>
        <Link href="/settings" className="md:hidden size-11 rounded-full hover:bg-[var(--surface-2)] flex items-center justify-center" aria-label="الإعدادات">
          <Settings className="size-[18px]" />
        </Link>
        {me && (
          <button onClick={onLogout} className="md:hidden size-11 rounded-full hover:bg-[var(--surface-2)] flex items-center justify-center" aria-label="تسجيل الخروج">
            <LogOut className="size-[18px]" />
          </button>
        )}
      </div>
    </header>
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
            <main className="px-4 pt-3 pb-28 md:pt-0 md:pb-10 md:px-8 max-w-7xl w-full flex-1">{children}</main>
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
