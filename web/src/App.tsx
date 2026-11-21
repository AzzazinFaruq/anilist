import { useEffect, useState } from "react";
import { api, ApiError, setUnauthorizedHandler } from "./api";
import { DetailSheet } from "./components/Detail";
import { Toast } from "./components/Toast";
import { navigate, pagePath, useLocation, type Page } from "./router";
import type { Me } from "./types";
import { Account } from "./pages/Account";
import { Home } from "./pages/Home";
import { List } from "./pages/List";
import { Login } from "./pages/Login";
import { Browse } from "./pages/Browse";
import { Temuan } from "./pages/Temuan";

const PAGES: [Page, string][] = [
  ["nonton", "Nonton"],
  ["jelajah", "Jelajah"],
  ["temuan", "Temuan"],
  ["daftar", "Daftar"],
];
const TITLES: Record<Page, string> = { nonton: "Nonton", jelajah: "Jelajah", temuan: "Temuan", daftar: "Daftar", akun: "Akun" };

function focusSearch() {
  if (!location.pathname.startsWith("/jelajah")) navigate("/jelajah");
  requestAnimationFrame(() => document.getElementById("global-search")?.focus());
}

export function App() {
  const [me, setMe] = useState<Me | null | undefined>(undefined);
  const [bootError, setBootError] = useState("");
  const { page, open } = useLocation();

  useEffect(() => {
    setUnauthorizedHandler(() => setMe(null));
    api.me().then(setMe, (e: ApiError) => (e.status === 401 ? setMe(null) : setBootError(e.message)));
  }, []);

  useEffect(() => {
    document.title = me ? `${TITLES[page]} · Tonton` : "Tonton";
  }, [page, me]);

  // "/" or Ctrl K: search from anywhere.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement;
      const typing = ["INPUT", "TEXTAREA", "SELECT"].includes(t.tagName);
      if ((e.key === "k" && (e.ctrlKey || e.metaKey)) || (e.key === "/" && !typing && !document.querySelector("dialog[open]"))) {
        e.preventDefault();
        focusSearch();
      }
    };
    addEventListener("keydown", onKey);
    return () => removeEventListener("keydown", onKey);
  }, []);

  if (bootError && me === undefined)
    return (
      <main className="login">
        <h1 className="wordmark">Tonton</h1>
        <p className="error">{bootError}</p>
        <button className="btn" type="button" onClick={() => location.reload()}>
          Muat ulang
        </button>
      </main>
    );
  if (me === undefined) return null;
  if (me === null)
    return (
      <>
        <Login onDone={setMe} />
        <Toast />
      </>
    );

  const go = (p: Page) => (e: React.MouseEvent) => {
    if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
    e.preventDefault();
    navigate(pagePath(p));
  };

  return (
    <>
      <a className="skip" href="#main">
        Langsung ke isi
      </a>
      <header className="top">
        <a className="wordmark" href="/" onClick={go("nonton")}>
          Tonton
        </a>
        <nav className="nav" aria-label="Halaman">
          {PAGES.map(([p, label]) => (
            <a key={p} href={pagePath(p)} onClick={go(p)} aria-current={page === p ? "page" : undefined}>
              {label}
            </a>
          ))}
        </nav>
        <button type="button" className="search-btn" onClick={focusSearch} aria-label="Cari judul (/ atau Ctrl K)" title="Cari judul (/ atau Ctrl K)">
          <svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true">
            <circle cx="10.5" cy="10.5" r="6.5" fill="none" stroke="currentColor" strokeWidth="2" />
            <path d="M15.5 15.5 20 20" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
          </svg>
        </button>
        <a className="who" href="/akun" onClick={go("akun")} aria-current={page === "akun" ? "page" : undefined}>
          {me.name}
        </a>
      </header>
      <main id="main" className="page">
        {page === "nonton" && <Home />}
        {page === "daftar" && <List />}
        {page === "jelajah" && <Browse />}
        {page === "temuan" && <Temuan />}
        {page === "akun" && (
          <Account
            me={me}
            onLogout={async () => {
              await api.logout().catch(() => {});
              setMe(null);
              navigate("/");
            }}
          />
        )}
      </main>
      {open && <DetailSheet id={open} key={open} />}
      <Toast />
    </>
  );
}
