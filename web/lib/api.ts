export const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

const TOKEN_KEY = "camera.token";

// Development sign-in keeps the token for the tab only. Production uses the
// identity provider's code flow through the server, with the refresh token in
// an HttpOnly cookie.
export function getToken(): string | null {
  if (typeof window === "undefined") return null;
  try {
    return sessionStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

export function setToken(token: string | null) {
  try {
    if (token) sessionStorage.setItem(TOKEN_KEY, token);
    else sessionStorage.removeItem(TOKEN_KEY);
  } catch {
    /* storage unavailable */
  }
}

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
  }
}

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const token = getToken();
  const res = await fetch(API_URL + path, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...init?.headers,
    },
  });
  if (res.status === 401) {
    setToken(null);
    if (typeof window !== "undefined" && !location.pathname.startsWith("/login")) location.href = "/login";
    throw new ApiError(401, "unauthorized");
  }
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new ApiError(res.status, body.error ?? res.statusText);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

export function eventStreamUrl(): string {
  return `${API_URL}/v1/events/stream?access_token=${encodeURIComponent(getToken() ?? "")}`;
}

export function thumbUrl(cameraId: string): string {
  return `${API_URL}/v1/cameras/${cameraId}/thumb.jpg`;
}

export type Site = {
  id: string;
  name: string;
  facility_category: string;
  address?: string;
  lat?: number;
  lng?: number;
  retention_days_required: number;
  cameras: number;
  cameras_online: number;
};

export type Camera = {
  id: string;
  device_id: string;
  site_id: string;
  name: string;
  display_name: string;
  vendor?: string;
  model?: string;
  ptz: boolean;
  lat?: number;
  lng?: number;
  heading_deg?: number;
  fov_deg?: number;
  status: string;
  article2_zone?: string;
};

export type Device = {
  id: string;
  site_id: string | null;
  serial: string;
  hw_model: string;
  fw_version: string;
  status: string;
  last_seen_at: string | null;
  disk_total: number | null;
  disk_used: number | null;
  retention_days: number | null;
  health?: { cpu_temp_c?: number; queue_depth?: number; retention_days?: number } | null;
};

export type CameraEvent = {
  time: string;
  device_id: string;
  edge_id: string;
  site_id: string;
  camera_id: string | null;
  camera_name: string | null;
  kind: "object" | "audio" | "system";
  label: string;
  score: number | null;
  zones: string[] | null;
  end_time: string | null;
  snapshot_key?: string;
  severity: "info" | "warning" | "critical";
  acked_at?: string;
};

export type ComplianceCheck = {
  type: string;
  title_ar: string;
  status: "pass" | "warn" | "fail";
  detail_ar: string;
};

export type ComplianceReport = {
  site_id: string;
  site_name: string;
  category: { id: string; name_ar: string; retention_days: number; mandatory: boolean } | null;
  generated_at: string;
  score: number;
  checks: ComplianceCheck[];
};

export type Me = { user_id: string; name: string; tenant_id: string; tenant_name: string; roles: Record<string, string> };

export const LABELS: Record<string, string> = {
  person: "شخص",
  car: "سيارة",
  motorcycle: "دراجة نارية",
  bicycle: "دراجة",
  glass: "كسر زجاج",
  scream: "صراخ",
  yell: "صياح",
  fire_alarm: "إنذار حريق",
  smoke_detector: "كاشف دخان",
  siren: "صفارة",
  gunshot: "طلق ناري",
  retention_short: "مدة الحفظ أقل من المطلوب",
  camera_offline: "كاميرا متوقفة",
};

export const label = (l: string) => LABELS[l] ?? l;

export const CATEGORIES: Record<string, string> = {
  hotel: "فندق أو شقق مفروشة",
  mall: "مجمع تجاري",
  retail: "محل تجاري",
  restaurant: "مطعم أو مقهى",
  fuel_station: "محطة وقود",
  bank: "مصرف أو محل صرافة",
  jewelry: "محل ذهب ومجوهرات",
  health: "منشأة صحية",
  education: "منشأة تعليمية",
  warehouse: "مستودع",
  residential_compound: "مجمع سكني",
  sports: "منشأة رياضية أو ترفيهية",
  office: "مكتب",
};

const timeFmt = new Intl.DateTimeFormat("ar-SA-u-nu-latn", {
  timeZone: "Asia/Riyadh",
  hour: "2-digit",
  minute: "2-digit",
  day: "numeric",
  month: "short",
});

export const formatTime = (iso: string) => timeFmt.format(new Date(iso));
