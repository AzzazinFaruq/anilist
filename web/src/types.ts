export type Status = "watching" | "plan" | "done" | "hold" | "drop";

export interface Anime {
  id: number;
  title: string;
  english?: string;
  native?: string;
  year: number;
  season?: number;
  format?: string;
  status?: string;
  episodes?: number;
  duration?: number;
  score?: number;
  popularity: number;
  image: string;
  genres?: string[];
  studios?: string[];
  color?: string;
  tier?: string;
  worth?: number;
  sequel?: boolean;
  country?: string;
}

export interface Detail extends Anime {
  synopsis?: string;
  synopsisLang?: "id" | "en";
  links?: { site: string; url: string }[];
  tags?: string[];
  source?: string;
  favorites?: number;
  trailer?: string;
}

export interface Entry {
  status: Status;
  episode?: number;
  rating?: number;
  note?: string;
  priority?: boolean;
  updated: number;
}

export interface Airing {
  id: number;
  episode: number;
  at: number;
}

export interface Item {
  anime: Anime;
  entry?: Entry;
  next?: Airing;
  released?: number;
  why?: string[];
  match?: number;
  kind?: string;
}

export interface Me {
  id: string;
  name: string;
  admin: boolean;
}

export type Counts = Partial<Record<Status, number>>;

export interface DayCount {
  day: string;
  n: number;
}

export interface TagInfo {
  name: string;
  category: string;
  count: number;
}

export interface Meta {
  genres: string[];
  tags: TagInfo[];
  studios: string[];
  tiers: { id: string; name: string; min: number }[];
  years: [number, number];
  sorts: [string, string][];
  current: { year: number; season: number };
}
