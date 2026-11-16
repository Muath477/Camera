"use client";

import { useEffect, useRef } from "react";

export type Mood = "happy" | "alert" | "sleepy" | "shy" | "dizzy" | "wink";

// Lensy: a little camera with one big eye. It blinks, looks toward the
// pointer (or wherever `look` points), and shows a mood: happy when all is
// well, alert when something needs attention, sleepy when there is nothing
// yet, shy (eye covered) while a secret is typed, dizzy after a mistake and
// wink on success.
export default function Mascot({
  size = 64,
  mood = "happy",
  follow = false,
  look,
  className = "",
}: {
  size?: number;
  mood?: Mood;
  follow?: boolean;
  // Where the eye looks, each axis from -1 to 1; overrides `follow`.
  look?: { x: number; y: number } | null;
  className?: string;
}) {
  const pupil = useRef<SVGGElement>(null);
  const root = useRef<SVGSVGElement>(null);

  useEffect(() => {
    if (look || !follow || window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
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
  }, [follow, look]);

  useEffect(() => {
    if (look && pupil.current) pupil.current.style.transform = `translate(${look.x * 5}px, ${look.y * 5}px)`;
  }, [look]);

  const body = mood === "alert" || mood === "dizzy" ? "var(--color-pop-400)" : mood === "sleepy" ? "var(--color-lilac-400)" : "var(--color-brand-400)";
  const ink = "var(--color-ink-900)";
  const openEye = mood === "happy" || mood === "alert";

  return (
    <svg ref={root} viewBox="0 0 64 64" width={size} height={size} className={className} aria-hidden>
      {/* flash bump and record light */}
      <rect x="10" y="9" width="16" height="10" rx="3" fill="var(--color-sun-400)" stroke={ink} strokeWidth="2.5" />
      <circle cx="48" cy="17" r="3.2" fill={mood === "alert" ? "var(--color-bad-500)" : "var(--color-pop-400)"} stroke={ink} strokeWidth="2" />
      {/* body */}
      <rect x="4" y="15" width="56" height="42" rx="12" fill={body} stroke={ink} strokeWidth="2.5" />
      {/* lens ring */}
      <circle cx="32" cy="36" r="15" fill="#fffdf8" stroke={ink} strokeWidth="2.5" />
      <g ref={pupil} className="mascot-pupil" style={{ transition: "transform 240ms cubic-bezier(0.34,1.56,0.64,1)" }}>
        {openEye && (
          <>
            <circle cx="32" cy="36" r={mood === "alert" ? 8.5 : 7.5} fill={ink} />
            <circle cx="29" cy="33" r="2.6" fill="#fff" />
          </>
        )}
        {mood === "sleepy" && <path d="M24 37 q8 6 16 0" fill="none" stroke={ink} strokeWidth="3" strokeLinecap="round" />}
        {mood === "wink" && <path d="M23 38 q9 -9 18 0" fill="none" stroke={ink} strokeWidth="3.2" strokeLinecap="round" />}
        {mood === "dizzy" && (
          <path className="mascot-spin" d="M32 36 m0 -1 a1 1 0 1 1 -1 1 a3 3 0 1 1 3 3 a5 5 0 1 1 -5 -5 a7 7 0 1 1 7 7" fill="none" stroke={ink} strokeWidth="2.2" strokeLinecap="round" />
        )}
      </g>
      {openEye && <ellipse className="mascot-lid" cx="32" cy="36" rx="13.6" ry="13.6" fill={body} />}
      {/* shy: two little paws over the lens */}
      {mood === "shy" && (
        <g className="mascot-paws">
          <rect x="14" y="26" width="17" height="16" rx="7" fill="var(--color-sun-400)" stroke={ink} strokeWidth="2.2" />
          <rect x="33" y="26" width="17" height="16" rx="7" fill="var(--color-sun-400)" stroke={ink} strokeWidth="2.2" />
          {/* fingers */}
          <path d="M20 26.5 v5 M25 26.5 v5 M39 26.5 v5 M44 26.5 v5" stroke={ink} strokeWidth="1.6" strokeLinecap="round" />
        </g>
      )}
      {/* cheeks, rosier when shy */}
      <circle cx="12" cy="45" r={mood === "shy" ? 4 : 3} fill={mood === "shy" ? "var(--color-pop-400)" : "var(--color-pop-100)"} opacity="0.9" />
      <circle cx="52" cy="45" r={mood === "shy" ? 4 : 3} fill={mood === "shy" ? "var(--color-pop-400)" : "var(--color-pop-100)"} opacity="0.9" />
    </svg>
  );
}
