import type { Anime, Status } from "./types";

export const STATUS: Record<Status, string> = {
  watching: "Ditonton",
  plan: "Rencana",
  done: "Selesai",
  hold: "Ditunda",
  drop: "Berhenti",
};
export const STATUS_ORDER: Status[] = ["watching", "plan", "done", "hold", "drop"];

export const SEASONS = ["Dingin", "Semi", "Panas", "Gugur"];
export const SEASON_KEYS = ["winter", "spring", "summer", "fall"];

export const COUNTRIES: Record<string, string> = { JP: "Jepang", CN: "China", KR: "Korea", TW: "Taiwan" };
export const AIR_STATUS: Record<string, string> = {
  FINISHED: "Tamat", RELEASING: "Sedang tayang", NOT_YET_RELEASED: "Belum tayang", CANCELLED: "Batal", HIATUS: "Hiatus",
};
export const LENGTHS: [string, string][] = [["short", "≤ 13 ep"], ["mid", "14–26 ep"], ["long", "27–60 ep"], ["xl", "> 60 ep"]];
export const FORMAT_KEYS = ["TV", "TV_SHORT", "MOVIE", "ONA", "OVA", "SPECIAL"];
export const DEFAULT_FORMATS = ["TV", "TV_SHORT", "MOVIE", "ONA"];

const FORMATS: Record<string, string> = {
  TV: "TV", TV_SHORT: "TV pendek", MOVIE: "Film", SPECIAL: "Spesial", OVA: "OVA", ONA: "ONA", MUSIC: "Musik",
};

export const formatName = (f?: string) => (f ? FORMATS[f] ?? f : "");

export function seasonName(a: Pick<Anime, "season" | "year">) {
  return a.season !== undefined && a.season !== null ? `${SEASONS[a.season]} ${a.year}` : String(a.year);
}

/** "TV, 28 episode, Gugur 2023" — the facts that tell titles apart. */
export function facts(a: Anime) {
  const parts = [formatName(a.format)];
  if (a.episodes && a.format !== "MOVIE") parts.push(`${a.episodes} episode`);
  if (a.format === "MOVIE" && a.duration) parts.push(`${a.duration} menit`);
  parts.push(seasonName(a));
  if (a.status === "RELEASING") parts.push("sedang tayang");
  if (a.status === "NOT_YET_RELEASED") parts.push("belum tayang");
  return parts.filter(Boolean).join(", ");
}

export const poster = (url: string, size: "medium" | "large" = "medium") =>
  size === "large" ? url.replace("/cover/medium/", "/cover/large/") : url;

const dayFmt = new Intl.DateTimeFormat("id-ID", { weekday: "short" });
const timeFmt = new Intl.DateTimeFormat("id-ID", { hour: "2-digit", minute: "2-digit" });

function startOfDay(d: Date) {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
}

/** "Hari ini 22.00", "Besok 01.30", "Jum 22.00". */
export function airTime(unix: number, now = new Date()) {
  const d = new Date(unix * 1000);
  const days = Math.round((startOfDay(d) - startOfDay(now)) / 86400000);
  const day = days === 0 ? "Hari ini" : days === 1 ? "Besok" : days === -1 ? "Kemarin" : dayFmt.format(d);
  return `${day} ${timeFmt.format(d)}`;
}

export function current() {
  const d = new Date();
  return { year: d.getFullYear(), season: Math.floor(d.getMonth() / 3) };
}

export const score10 = (s?: number) => (s ? (s / 10).toFixed(1).replace(".", ",") : "");
export const pct = (p?: number) => (p ? `${Math.round(p * 100)}%` : "");

const dateFmt = new Intl.DateTimeFormat("id-ID", { weekday: "long", day: "numeric", month: "long" });
export const longDate = (iso: string) => dateFmt.format(new Date(iso + "T00:00:00"));

/** The shorter of the English and romaji titles: easiest to recognise in a list. */
export const displayTitle = (a: Pick<Anime, "title" | "english">) =>
  a.english && a.english.length <= a.title.length ? a.english : a.title;

const longDay = new Intl.DateTimeFormat("id-ID", { weekday: "long", day: "numeric", month: "short" });
/** "Hari ini", "Besok", "Sabtu, 23 Nov". */
export function dayLabel(unix: number, now = new Date()) {
  const d = new Date(unix * 1000);
  const days = Math.round((new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime() - new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()) / 86400000);
  return days === 0 ? "Hari ini" : days === 1 ? "Besok" : longDay.format(d);
}
export const clock = (unix: number) => timeFmt.format(new Date(unix * 1000));
