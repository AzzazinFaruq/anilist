import type { Airing, Counts, Detail, Entry, Item, Me, Status } from "./types";

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
  }
}

let onUnauthorized: () => void = () => {};
export const setUnauthorizedHandler = (fn: () => void) => { onUnauthorized = fn; };

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      credentials: "same-origin",
      headers: body !== undefined || method !== "GET" ? { "Content-Type": "application/json" } : undefined,
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
  plan: Item[];
  upcoming: Item[];
  counts: Counts;
  today: number;
  week: number;
}

export interface Member extends Me {
  created: string;
}

export interface Change {
  status?: Status;
  episode?: number;
  rating?: number;
  note?: string;
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
  update: (id: number, c: Change) => call<Item>("PATCH", `/api/list/${id}`, c),
  remove: (id: number) => call<void>("DELETE", `/api/list/${id}`),
  search: (q: string) => call<{ items: Item[] }>("GET", `/api/search?q=${encodeURIComponent(q)}`),
  season: (year: number, season: number) =>
    call<{ year: number; season: number; items: Item[] }>("GET", `/api/season?year=${year}&season=${season}`),
  detail: (id: number) => call<{ anime: Detail; entry?: Entry; next?: Airing }>("GET", `/api/anime/${id}`),

  members: () => call<{ users: Member[] }>("GET", "/api/users"),
  addMember: (name: string, pin: string, admin: boolean) => call<Member>("POST", "/api/users", { name, pin, admin }),
  resetPIN: (id: string, pin: string) => call<Member>("PATCH", `/api/users/${id}`, { pin }),
  removeMember: (id: string) => call<void>("DELETE", `/api/users/${id}`),
};
