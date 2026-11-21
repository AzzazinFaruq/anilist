import type { Airing, Counts, DayCount, Detail, Entry, Item, Me, Meta, Status } from "./types";

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
  }
}

let onUnauthorized: () => void = () => {};
export const setUnauthorizedHandler = (fn: () => void) => {
  onUnauthorized = fn;
};

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      credentials: "same-origin",
      headers: method !== "GET" ? { "Content-Type": "application/json" } : undefined,
      body: body !== undefined ? JSON.stringify(body) : method !== "GET" ? "{}" : undefined,
    });
  } catch {
    throw new ApiError(0, "Server tidak terjangkau. Cek Wi-Fi atau Tailscale.");
  }
  if (res.status === 204) return undefined as T;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    if (res.status === 401 && path !== "/api/login") onUnauthorized();
    throw new ApiError(res.status, data.error || `Permintaan gagal (${res.status}).`);
  }
  return data as T;
}

export interface Home {
  watching: Item[];
  upcoming: Item[];
  queue: Item[];
  queueTotal: number;
  waiting: Item[];
  decisions: (Item & { reason: string })[];
  counts: Counts;
  ready: number;
  activity: DayCount[];
  streak: number;
  today: number;
  week: number;
  lastWeek: number;
}

export interface Stats {
  counts: Counts;
  episodes: number;
  hours: number;
  rated: number;
  avgRating: number;
  genres: { name: string; n: number }[];
  studios: { name: string; n: number }[];
  years: { name: string; n: number }[];
  streak: number;
  bestDay: number;
  activity: DayCount[];
}

export interface Temuan {
  batches: { date: string; items: Item[] }[];
  counts: { open: number; all: number; yes: number; no: number };
  taste: Record<string, number>;
  model?: { trained: string; positives: number; negatives: number; auc?: number; likes?: string[]; dislikes?: string[] };
  next?: string;
}

export interface Member extends Me {
  created: string;
}

export interface Change {
  status?: Status;
  episode?: number;
  rating?: number;
  note?: string;
  priority?: boolean;
}

export interface Preset {
  name: string;
  query: string;
}

export const api = {
  profiles: () => call<{ profiles: { name: string }[]; setup: boolean }>("GET", "/api/profiles"),
  setup: (name: string, pin: string) => call<Me>("POST", "/api/setup", { name, pin }),
  login: (name: string, pin: string) => call<Me>("POST", "/api/login", { name, pin }),
  logout: () => call<void>("POST", "/api/logout"),
  me: () => call<Me>("GET", "/api/me"),
  changePIN: (old: string, next: string) => call<void>("POST", "/api/me/pin", { old, new: next }),

  home: () => call<Home>("GET", "/api/home"),
  list: () => call<{ items: Item[]; counts: Counts }>("GET", "/api/list"),
  stats: () => call<Stats>("GET", "/api/stats"),
  update: (id: number, c: Change) => call<Item>("PATCH", `/api/list/${id}`, c),
  remove: (id: number) => call<void>("DELETE", `/api/list/${id}`),
  hide: (id: number, hidden: boolean) => call<void>("PUT", `/api/hidden/${id}`, { hidden }),

  meta: () => call<Meta>("GET", "/api/meta"),
  browse: (query: string) => call<{ total: number; page: number; size: number; items: Item[] }>("GET", `/api/browse?${query}`),
  random: (query: string) => call<{ item: Item }>("GET", `/api/browse?${query}&random=1`),
  pick: (genre = "") => call<{ item: Item }>("GET", `/api/pick${genre ? `?genre=${encodeURIComponent(genre)}` : ""}`),
  schedule: (scope: "mine" | "all") => call<{ items: Item[] }>("GET", `/api/schedule?scope=${scope}`),
  detail: (id: number) => call<{ anime: Detail; entry?: Entry; next?: Airing }>("GET", `/api/anime/${id}`),
  similar: (id: number) => call<{ items: Item[] }>("GET", `/api/anime/${id}/similar`),

  temuan: (filter: string) => call<Temuan>("GET", `/api/temuan?filter=${filter}`),
  setTaste: (taste: Record<string, number>) => call<void>("PUT", "/api/taste", { taste }),
  presets: () => call<{ presets: Preset[] }>("GET", "/api/presets"),
  setPresets: (presets: Preset[]) => call<void>("PUT", "/api/presets", { presets }),

  members: () => call<{ users: Member[] }>("GET", "/api/users"),
  addMember: (name: string, pin: string, admin: boolean) => call<Member>("POST", "/api/users", { name, pin, admin }),
  resetPIN: (id: string, pin: string) => call<Member>("PATCH", `/api/users/${id}`, { pin }),
  removeMember: (id: string) => call<void>("DELETE", `/api/users/${id}`),
};
