const riyadh = { timeZone: "Asia/Riyadh" } as const;

const time = new Intl.DateTimeFormat("ar-SA-u-nu-latn", { ...riyadh, hour: "2-digit", minute: "2-digit" });
const dateTime = new Intl.DateTimeFormat("ar-SA-u-nu-latn", { ...riyadh, hour: "2-digit", minute: "2-digit", day: "numeric", month: "short" });
const dayLong = new Intl.DateTimeFormat("ar-SA-u-nu-latn", { ...riyadh, weekday: "long", day: "numeric", month: "long" });
const rtf = new Intl.RelativeTimeFormat("ar", { numeric: "auto" });

export const fmtTime = (iso: string | Date) => time.format(new Date(iso));
export const fmtDateTime = (iso: string | Date) => dateTime.format(new Date(iso));
export const fmtDay = (iso: string | Date) => dayLong.format(new Date(iso));

export function fmtRelative(iso: string | Date): string {
  const diff = (new Date(iso).getTime() - Date.now()) / 1000;
  const abs = Math.abs(diff);
  if (abs < 60) return rtf.format(Math.round(diff), "second");
  if (abs < 3600) return rtf.format(Math.round(diff / 60), "minute");
  if (abs < 86400) return rtf.format(Math.round(diff / 3600), "hour");
  return rtf.format(Math.round(diff / 86400), "day");
}

export const fmtTB = (n: number | null | undefined) => (n == null ? "—" : `${(n / 1e12).toFixed(1)} TB`);

export const fmtPercent = (n: number) => `${Math.round(n * 100)}%`;

// countAR writes a count with the noun in the form Arabic grammar wants:
// one, two (dual), 3 to 10 (plural), 11 and up (singular accusative).
export function countAR(n: number, forms: { one: string; two: string; few: string; many: string }): string {
  if (n === 1) return forms.one;
  if (n === 2) return forms.two;
  if (n >= 3 && n <= 10) return `${n} ${forms.few}`;
  return `${n} ${forms.many}`;
}

export const alertsAR = (n: number) =>
  countAR(n, { one: "تنبيه حرج واحد", two: "تنبيهان حرجان", few: "تنبيهات حرجة", many: "تنبيهاً حرجاً" });

export const offlineCamsAR = (n: number) =>
  countAR(n, { one: "كاميرا واحدة منقطعة", two: "كاميرتان منقطعتان", few: "كاميرات منقطعة", many: "كاميرا منقطعة" });

// Day key in Riyadh time, used to group events.
export function dayKey(iso: string): string {
  return new Intl.DateTimeFormat("en-CA", { ...riyadh, year: "numeric", month: "2-digit", day: "2-digit" }).format(new Date(iso));
}
