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

/** The shorter of the English and romaji titles: easiest to recognise in a list. */
export const displayTitle = (a: Pick<Anime, "title" | "english">) =>
  a.english && a.english.length <= a.title.length ? a.english : a.title;
