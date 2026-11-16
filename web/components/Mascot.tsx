"use client";

import { useEffect, useRef } from "react";

type Mood = "happy" | "alert" | "sleepy";

// Lensy: a little camera with one big eye. It blinks, looks toward the
// pointer, and shows the account's mood: happy when all is well, alert when
// something needs attention, sleepy when there is nothing to show yet.
export default function Mascot({ size = 64, mood = "happy", follow = false, className = "" }: { size?: number; mood?: Mood; follow?: boolean; className?: string }) {
  const pupil = useRef<SVGGElement>(null);
  const root = useRef<SVGSVGElement>(null);

  useEffect(() => {
    if (!follow || window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    const onMove = (e: PointerEvent) => {
      const svg = root.current;
      const p = pupil.current;
      if (!svg || !p) return;
      const r = svg.getBoundingClientRect();
      const dx = e.clientX - (r.left + r.width / 2);
      const dy = e.clientY - (r.top + r.height * 0.56);
      const d = Math.hypot(dx, dy) || 1;
      const k = Math.min(1, d / 300) * 5;
      p.style.transform = `translate(${(dx / d) * k}px, ${(dy / d) * k}px)`;
    };
    window.addEventListener("pointermove", onMove);
    return () => window.removeEventListener("pointermove", onMove);
  }, [follow]);

  const body = mood === "alert" ? "var(--color-pop-400)" : mood === "sleepy" ? "var(--color-lilac-400)" : "var(--color-brand-400)";

  return (
    <svg ref={root} viewBox="0 0 64 64" width={size} height={size} className={className} aria-hidden>
      {/* flash bump and record light */}
      <rect x="10" y="9" width="16" height="10" rx="3" fill="var(--color-sun-400)" stroke="var(--color-ink-900)" strokeWidth="2.5" />
      <circle cx="48" cy="17" r="3.2" fill={mood === "alert" ? "var(--color-bad-500)" : "var(--color-pop-400)"} stroke="var(--color-ink-900)" strokeWidth="2" />
      {/* body */}
      <rect x="4" y="15" width="56" height="42" rx="12" fill={body} stroke="var(--color-ink-900)" strokeWidth="2.5" />
      {/* lens ring */}
      <circle cx="32" cy="36" r="15" fill="#fffdf8" stroke="var(--color-ink-900)" strokeWidth="2.5" />
      <g ref={pupil} className="mascot-pupil" style={{ transition: "transform 240ms cubic-bezier(0.34,1.56,0.64,1)" }}>
        {mood === "sleepy" ? (
          <path d="M24 37 q8 6 16 0" fill="none" stroke="var(--color-ink-900)" strokeWidth="3" strokeLinecap="round" />
        ) : (
          <>
            <circle cx="32" cy="36" r={mood === "alert" ? 8.5 : 7.5} fill="var(--color-ink-900)" />
            <circle cx="29" cy="33" r="2.6" fill="#fff" />
          </>
        )}
      </g>
      {mood !== "sleepy" && <ellipse className="mascot-lid" cx="32" cy="36" rx="13.6" ry="13.6" fill={body} />}
      {/* cheeks */}
      <circle cx="12" cy="45" r="3" fill="var(--color-pop-100)" opacity="0.9" />
      <circle cx="52" cy="45" r="3" fill="var(--color-pop-100)" opacity="0.9" />
    </svg>
  );
}
