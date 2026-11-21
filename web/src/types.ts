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
}

export interface Detail extends Anime {
  synopsis?: string;
  synopsisLang?: "id" | "en";
  links?: { site: string; url: string }[];
}

export interface Entry {
  status: Status;
  episode?: number;
  rating?: number;
  note?: string;
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
}

export interface Me {
  id: string;
  name: string;
  admin: boolean;
}

export type Counts = Partial<Record<Status, number>>;
