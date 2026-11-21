import { useSyncExternalStore } from "react";

// Pages are paths; the open title is ?judul=<id> so Back closes it.
export type Page = "nonton" | "jelajah" | "temuan" | "daftar" | "akun";

const listeners = new Set<() => void>();
const notify = () => listeners.forEach((l) => l());
addEventListener("popstate", notify);

function subscribe(l: () => void) {
  listeners.add(l);
  return () => listeners.delete(l);
}

const snapshot = () => location.pathname + location.search;

export function useLocation() {
  const loc = useSyncExternalStore(subscribe, snapshot);
  const url = new URL(loc, location.origin);
  const seg = url.pathname.split("/")[1];
  const page: Page = seg === "jelajah" || seg === "temuan" || seg === "daftar" || seg === "akun" ? seg : seg === "cari" ? "jelajah" : "nonton";
  const open = Number(url.searchParams.get("judul")) || null;
  return { page, open, params: url.searchParams };
}

export function navigate(to: string, replace = false, state: unknown = null) {
  if (to === snapshot()) return;
  history[replace ? "replaceState" : "pushState"](state, "", to);
  scrollTo(0, 0);
  notify();
}

export const pagePath = (p: Page) => (p === "nonton" ? "/" : "/" + p);

function withTitle(id: number | null) {
  const url = new URL(location.href);
  if (id) url.searchParams.set("judul", String(id));
  else url.searchParams.delete("judul");
  return url.pathname + url.search;
}

export function openTitle(id: number) {
  const replace = new URL(location.href).searchParams.has("judul");
  history[replace ? "replaceState" : "pushState"]({ title: true }, "", withTitle(id));
  notify();
}

export function closeTitle() {
  // Opened from inside the app: step back so the Back button stays meaningful.
  if (history.state?.title) history.back();
  else {
    history.replaceState(null, "", withTitle(null));
    notify();
  }
}

/** Change query params in place (no new history entry). null removes a key. */
export function setParams(changes: Record<string, string | null>) {
  const url = new URL(location.href);
  for (const [k, v] of Object.entries(changes)) {
    if (v) url.searchParams.set(k, v);
    else url.searchParams.delete(k);
  }
  history.replaceState(history.state, "", url.pathname + url.search);
  notify();
}

export const setParam = (key: string, value: string | null) => setParams({ [key]: value });
