import { useCallback, useEffect, useState, useSyncExternalStore } from "react";
import { api, type Change } from "./api";
import type { Item } from "./types";

// One revision number for "the list changed somewhere"; every page refetches when it moves.
let revision = 0;
const revListeners = new Set<() => void>();
export function bump() {
  revision++;
  revListeners.forEach((l) => l());
}
const useRevision = () =>
  useSyncExternalStore((l) => (revListeners.add(l), () => revListeners.delete(l)), () => revision);

export function useData<T>(load: () => Promise<T>, deps: unknown[]) {
  const rev = useRevision();
  const [state, setState] = useState<{ data?: T; error?: string; loading: boolean }>({ loading: true });
  const [nonce, setNonce] = useState(0);
  useEffect(() => {
    let live = true;
    setState((s) => ({ ...s, loading: true }));
    load().then(
      (data) => live && setState({ data, loading: false }),
      (e: Error) => live && setState((s) => ({ data: s.data, error: e.message, loading: false })),
    );
    return () => {
      live = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, rev, nonce]);
  const retry = useCallback(() => setNonce((n) => n + 1), []);
  return { ...state, retry };
}

// ---------- toast ----------

let toastMsg: { text: string; id: number } | null = null;
const toastListeners = new Set<() => void>();
let toastTimer = 0;
export function toast(text: string) {
  toastMsg = { text, id: Date.now() };
  toastListeners.forEach((l) => l());
  clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => {
    toastMsg = null;
    toastListeners.forEach((l) => l());
  }, 2600);
}
export const useToast = () =>
  useSyncExternalStore((l) => (toastListeners.add(l), () => toastListeners.delete(l)), () => toastMsg);

// ---------- mutations ----------

export async function change(id: number, c: Change, quiet = false): Promise<Item | null> {
  try {
    const it = await api.update(id, c);
    bump();
    if (!quiet) toast("Tersimpan");
    return it;
  } catch (e) {
    toast((e as Error).message);
    return null;
  }
}

export async function removeEntry(id: number) {
  try {
    await api.remove(id);
    bump();
    toast("Dihapus dari daftar");
    return true;
  } catch (e) {
    toast((e as Error).message);
    return false;
  }
}
