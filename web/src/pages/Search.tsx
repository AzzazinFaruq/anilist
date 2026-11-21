import { useEffect, useRef, useState } from "react";
import { api } from "../api";
import { useData } from "../data";
import { current, displayTitle, facts, SEASONS, STATUS } from "../format";
import { Icon } from "../components/Icon";
import { Poster } from "../components/Poster";
import { Row } from "../components/Row";
import { Failed, Loading } from "../components/States";
import { openTitle, setParam, useLocation } from "../router";
import type { Item } from "../types";

type Kind = "all" | "tv" | "movie" | "other";
const KINDS: [Kind, string][] = [["all", "Semua"], ["tv", "TV"], ["movie", "Film"], ["other", "Lainnya"]];
const kindOf = (f?: string): Kind => (f === "TV" || f === "TV_SHORT" ? "tv" : f === "MOVIE" ? "movie" : "other");

function useDebounced(v: string, ms: number) {
  const [d, setD] = useState(v);
  useEffect(() => {
    const t = setTimeout(() => setD(v), ms);
    return () => clearTimeout(t);
  }, [v, ms]);
  return d;
}

export function Search() {
  const { params } = useLocation();
  const [q, setQ] = useState(params.get("q") ?? "");
  const query = useDebounced(q.trim(), 200);
  const input = useRef<HTMLInputElement>(null);

  useEffect(() => setParam("q", query || null), [query]);
  useEffect(() => {
    // "/" anywhere jumps to the search box.
    const onKey = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement;
      if (e.key === "/" && !["INPUT", "TEXTAREA", "SELECT"].includes(t.tagName)) {
        e.preventDefault();
        input.current?.focus();
      }
    };
    addEventListener("keydown", onKey);
    return () => removeEventListener("keydown", onKey);
  }, []);

  return (
    <section className="block">
      <h1 className="sr-only">Cari</h1>
      <input
        ref={input}
        className="big-search"
        type="search"
        value={q}
        onChange={(e) => setQ(e.target.value)}
        placeholder="Judul anime"
        aria-label="Cari judul anime"
        autoFocus={!q}
        enterKeyHint="search"
      />
      {query.length >= 2 ? <Results q={query} /> : <SeasonBrowser />}
    </section>
  );
}

function onList(it: Item) {
  return it.entry ? STATUS[it.entry.status] : "";
}

function Results({ q }: { q: string }) {
  const { data, error, retry, loading } = useData(() => api.search(q), [q]);
  if (!data) return error ? <Failed message={error} retry={retry} /> : <Loading />;
  if (data.items.length === 0) return <p className="empty">Tidak ada judul yang cocok dengan “{q}”. Coba judul romaji atau Inggrisnya.</p>;
  return (
    <ul className={`rows${loading ? " stale" : ""}`}>
      {data.items.map((it) => (
        <Row key={it.anime.id} item={it} sub={facts(it.anime)} aside={it.entry && <span className="tag">{onList(it)}</span>} />
      ))}
    </ul>
  );
}

function SeasonBrowser() {
  const { params } = useLocation();
  const now = current();
  const year = Number(params.get("tahun")) || now.year;
  const season = params.has("musim") ? Math.min(3, Math.max(0, Number(params.get("musim")))) : now.season;
  const [kind, setKind] = useState<Kind>("tv");
  const { data, error, retry, loading } = useData(() => api.season(year, season), [year, season]);

  const step = (d: number) => {
    let s = season + d, y = year;
    if (s < 0) (s = 3), y--;
    if (s > 3) (s = 0), y++;
    const isNow = y === now.year && s === now.season;
    setParam("tahun", isNow ? null : String(y));
    setParam("musim", isNow ? null : String(s));
  };

  const items = data?.items.filter((it) => kind === "all" || kindOf(it.anime.format) === kind) ?? [];

  return (
    <>
      <div className="season-bar">
        <button type="button" className="btn ghost" onClick={() => step(-1)} aria-label="Musim sebelumnya">
          <Icon name="prev" />
        </button>
        <h2>
          {SEASONS[season]} {year}
        </h2>
        <button type="button" className="btn ghost" onClick={() => step(1)} aria-label="Musim berikutnya">
          <Icon name="next" />
        </button>
        <div className="seg small" role="group" aria-label="Jenis">
          {KINDS.map(([k, label]) => (
            <button key={k} type="button" aria-pressed={kind === k} onClick={() => setKind(k)}>
              {label}
            </button>
          ))}
        </div>
      </div>
      {!data && (error ? <Failed message={error} retry={retry} /> : <Loading />)}
      {data && items.length === 0 && <p className="empty">Belum ada judul untuk musim ini di katalog.</p>}
      <ul className={`grid${loading ? " stale" : ""}`}>
        {items.map((it) => (
          <li key={it.anime.id}>
            <button type="button" onClick={() => openTitle(it.anime.id)}>
              <Poster src={it.anime.image} />
              <span className="grid-title">{displayTitle(it.anime)}</span>
              <span className="quiet small">{it.entry ? onList(it) : it.anime.score ? `skor ${it.anime.score}` : ""}</span>
            </button>
          </li>
        ))}
      </ul>
    </>
  );
}
