"use client";

import { createContext, useContext, useEffect, useState, type ReactNode } from "react";

type InstallPrompt = Event & { prompt: () => Promise<void>; userChoice: Promise<{ outcome: "accepted" | "dismissed" }> };

type Ctx = {
  // Set while the browser offers to install the app.
  canInstall: boolean;
  install: () => Promise<boolean>;
  // True when running as an installed app.
  standalone: boolean;
};

const PWAContext = createContext<Ctx>({ canInstall: false, install: async () => false, standalone: false });

export function usePWA() {
  return useContext(PWAContext);
}

// Registers the service worker and keeps the install prompt until the
// person asks for it from the settings page or the top bar.
export function PWAProvider({ children }: { children: ReactNode }) {
  const [prompt, setPrompt] = useState<InstallPrompt | null>(null);
  const [standalone, setStandalone] = useState(false);

  useEffect(() => {
    if ("serviceWorker" in navigator) navigator.serviceWorker.register("/sw.js").catch(() => {});
    setStandalone(window.matchMedia("(display-mode: standalone)").matches || (navigator as unknown as { standalone?: boolean }).standalone === true);
    const onPrompt = (e: Event) => {
      e.preventDefault();
      setPrompt(e as InstallPrompt);
    };
    const onInstalled = () => setPrompt(null);
    window.addEventListener("beforeinstallprompt", onPrompt);
    window.addEventListener("appinstalled", onInstalled);
    return () => {
      window.removeEventListener("beforeinstallprompt", onPrompt);
      window.removeEventListener("appinstalled", onInstalled);
    };
  }, []);

  const install = async () => {
    if (!prompt) return false;
    await prompt.prompt();
    const { outcome } = await prompt.userChoice;
    if (outcome === "accepted") setPrompt(null);
    return outcome === "accepted";
  };

  return <PWAContext.Provider value={{ canInstall: !!prompt, install, standalone }}>{children}</PWAContext.Provider>;
}
