import { useEffect, useMemo, useRef, useState } from "react";
import { api, type Preset } from "../api";
import { toast, useData } from "../data";
import { AIR_STATUS, COUNTRIES, DEFAULT_FORMATS, FORMAT_KEYS, formatName, LENGTHS, SEASON_KEYS, SEASONS } from "../format";
import { PosterGrid, TitleTable } from "../components/Grid";
import { Icon } from "../components/Icon";
import { Failed, Loading } from "../components/States";
import { Chip } from "../components/ui";
import { openTitle, setParams, useLocation } from "../router";
import type { Item, Meta } from "../types";

// Built-in starting points, tuned to what this household watches.
const PRESETS: Preset[] = [
  { name: "Isekai paling dicari", query: "m=all&t=Isekai&fmt=TV&nosq=1&sort=pop" },
  { name: "Isekai hidden gem", query: "m=all&t=Isekai&nosq=1&sort=gem" },
  { name: "Isekai + romance", query: "m=all&t=Isekai&g=Romance&nosq=1&sort=score&pop=5000" },
  { name: "Romansa terbaik", query: "m=all&g=Romance&nosq=1&sort=score&pop=20000" },
  { name: "Romansa fantasi", query: "m=all&g=Romance|Fantasy&nosq=1&sort=score&pop=5000" },
  { name: "Fantasi epik", query: "m=all&g=Fantasy|Adventure&nosq=1&sort=score&pop=20000" },
  { name: "Bangun kerajaan", query: "m=all&t=Kingdom Management&nosq=1&sort=score" },
  { name: "Selesai sekali duduk", query: "m=all&len=short&st=FINISHED&fmt=TV&score=75&nosq=1&sort=score" },
  { name: "Klasik 2000-an", query: "m=all&from=2000&to=2009&nosq=1&sort=score&pop=5000" },
  { name: "Layak tonton tier S", query: "m=all&tier=S&sort=worth" },
];

const QUICK_TAGS = [
  "Isekai", "Reincarnation", "Magic", "Dungeon", "Kingdom Management", "Demons", "Dragons", "Swordplay", "Super Power",
  "Anti-Hero", "Revenge", "Found Family", "Coming of Age", "Love Triangle", "Villainess", "Time Loop", "Iyashikei",
  "Military", "Politics", "School", "Video Games", "Survival", "Mythology", "Travel",
];

// Keys the page keeps in the URL. Everything else is ignored.
const KEYS = ["m", "y", "s", "from", "to", "q", "g", "gx", "t", "tx", "fmt", "st", "len", "co", "score", "pop", "tier", "studio", "nosq", "hide", "sort"];

const list = (v: string | null) => (v ? v.split("|").filter(Boolean) : []);
const join = (l: string[]) => (l.length ? l.join("|") : null);

function useDebounced<T>(v: T, ms: number) {
  const [d, setD] = useState(v);
  useEffect(() => {
    const t = setTimeout(() => setD(v), ms);
    return () => clearTimeout(t);
  }, [v, ms]);
  return d;
}

export function Browse() {
  const { data: meta, error, retry } = useData(api.meta, []);
  if (!meta) return error ? <Failed message={error} retry={retry} /> : <Loading />;
  return <BrowseInner meta={meta} />;
}

function BrowseInner({ meta }: { meta: Meta }) {
  const { params } = useLocation();
  const p = (k: string) => params.get(k);
  const mode = (p("m") as "season" | "year" | "all") || "season";
  const year = Number(p("y")) || meta.current.year;
  const season = p("s") !== null ? Math.min(3, Math.max(0, Number(p("s")))) : meta.current.season;
  const view = p("v") === "list" ? "list" : "grid";
  const [showFilters, setShowFilters] = useState(false);
  const [q, setQ] = useState(p("q") ?? "");
  const debouncedQ = useDebounced(q.trim(), 250);
  const search = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if ((p("q") ?? "") !== debouncedQ) setParams({ q: debouncedQ || null });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedQ]);

  // The query the API sees, derived from the URL.
  const apiQuery = useMemo(() => {
    const u = new URLSearchParams();
    if (mode === "season") {
      u.set("year", String(year));
      u.set("season", String(season));
    } else if (mode === "year") {
      u.set("year", String(year));
    } else {
      if (p("from")) u.set("from", p("from")!);
      if (p("to")) u.set("to", p("to")!);
    }
    for (const k of ["q", "g", "gx", "t", "tx", "st", "len", "co", "score", "pop", "tier", "studio", "nosq", "hide", "sort"]) {
      if (p(k)) u.set(k, p(k)!);
    }
    const fmt = p("fmt");
    if (fmt !== "all") u.set("fmt", fmt || DEFAULT_FORMATS.join("|"));
    return u.toString();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [params.toString(), mode, year, season]);

  const set = (changes: Record<string, string | null>) => setParams(changes);
  const stepSeason = (d: number) => {
    if (mode === "season") {
      let s = season + d, y = year;
      if (s < 0) (s = 3), y--;
      if (s > 3) (s = 0), y++;
      set({ y: String(y), s: String(s) });
    } else if (mode === "year") set({ y: String(year + d) });
  };

  const random = async () => {
    try {
      const { item } = await api.random(apiQuery);
      openTitle(item.anime.id);
    } catch (e) {
      toast((e as Error).message);
    }
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement;
      if (["INPUT", "TEXTAREA", "SELECT"].includes(t.tagName) || document.querySelector("dialog[open]") || e.metaKey || e.ctrlKey || e.altKey) return;
      if (e.key === "ArrowLeft") stepSeason(-1);
      else if (e.key === "ArrowRight") stepSeason(1);
      else if (e.key === "r" || e.key === "R") random();
      else return;
      e.preventDefault();
    };
    addEventListener("keydown", onKey);
    return () => removeEventListener("keydown", onKey);
  });

  const active = countFilters(params);

  return (
    <section className="block">
      <h1 className="sr-only">Jelajah anime</h1>
      <input
        ref={search}
        id="global-search"
        className="big-search"
        type="search"
        value={q}
        onChange={(e) => setQ(e.target.value)}
        placeholder="Cari judul, romaji, Inggris, Jepang"
        aria-label="Cari judul anime"
        enterKeyHint="search"
      />

      <div className="time-bar">
        <div className="seg" role="group" aria-label="Rentang waktu">
          {(
            [
              ["season", "Per musim"],
              ["year", "Per tahun"],
              ["all", "Semua tahun"],
            ] as const
          ).map(([m, label]) => (
            <button key={m} type="button" aria-pressed={mode === m} onClick={() => set({ m: m === "season" ? null : m })}>
              {label}
            </button>
          ))}
        </div>

        {mode !== "all" ? (
          <div className="stepper-nav">
            <button type="button" className="btn ghost" onClick={() => stepSeason(-1)} aria-label="Sebelumnya (panah kiri)">
              <Icon name="prev" />
            </button>
            {mode === "season" ? (
              <span className={`when ${SEASON_KEYS[season]}`}>
                {SEASONS[season]} {year}
              </span>
            ) : (
              <span className="when">{year}</span>
            )}
            <button type="button" className="btn ghost" onClick={() => stepSeason(1)} aria-label="Berikutnya (panah kanan)">
              <Icon name="next" />
            </button>
            {mode === "season" && (
              <div className="season-picks" role="group" aria-label="Pilih musim">
                {SEASONS.map((name, i) => (
                  <button key={name} type="button" className={SEASON_KEYS[i]} aria-pressed={i === season} onClick={() => set({ s: String(i), y: String(year) })}>
                    {name}
                  </button>
                ))}
              </div>
            )}
            <label className="select year-select">
              <span className="sr-only">Tahun</span>
              <select value={year} onChange={(e) => set({ y: e.target.value })}>
                {range(meta.years[1], meta.years[0]).map((y) => (
                  <option key={y}>{y}</option>
                ))}
              </select>
            </label>
            {(year !== meta.current.year || (mode === "season" && season !== meta.current.season)) && (
              <button type="button" className="link" onClick={() => set({ y: null, s: null })}>
                Musim ini
              </button>
            )}
          </div>
        ) : (
          <div className="stepper-nav">
            <YearSelect label="Dari" value={p("from")} years={meta.years} onChange={(v) => set({ from: v })} />
            <span className="quiet">sampai</span>
            <YearSelect label="Sampai" value={p("to")} years={meta.years} onChange={(v) => set({ to: v })} />
          </div>
        )}
      </div>

      <Presets current={params} />

      <div className="toolbar">
        <button type="button" className={`btn${active ? " solid" : ""}`} aria-expanded={showFilters} aria-controls="filters" onClick={() => setShowFilters(!showFilters)}>
          Filter{active ? ` (${active})` : ""}
        </button>
        <label className="select">
          <span className="sr-only">Urutkan</span>
          <select value={p("sort") || "pop"} onChange={(e) => set({ sort: e.target.value === "pop" ? null : e.target.value })}>
            {meta.sorts.map(([k, l]) => (
              <option key={k} value={k}>
                {l}
              </option>
            ))}
          </select>
        </label>
        <div className="seg small" role="group" aria-label="Tampilan">
          <button type="button" aria-pressed={view === "grid"} onClick={() => set({ v: null })}>
            Poster
          </button>
          <button type="button" aria-pressed={view === "list"} onClick={() => set({ v: "list" })}>
            Daftar
          </button>
        </div>
        <button type="button" className="btn" onClick={random} title="Pilih judul acak dari hasil (R)">
          Pilih acak
        </button>
      </div>

      {showFilters && <Filters meta={meta} params={params} />}

      <Results query={apiQuery} view={view} />
    </section>
  );
}

function range(from: number, to: number) {
  const out: number[] = [];
  for (let y = from; y >= to; y--) out.push(y);
  return out;
}

function YearSelect({ label, value, years, onChange }: { label: string; value: string | null; years: [number, number]; onChange: (v: string | null) => void }) {
  return (
    <label className="select year-select">
      <span className="sr-only">{label}</span>
      <select value={value ?? ""} onChange={(e) => onChange(e.target.value || null)}>
        <option value="">{label === "Dari" ? "Paling awal" : "Paling baru"}</option>
        {range(years[1], years[0]).map((y) => (
          <option key={y}>{y}</option>
        ))}
      </select>
    </label>
  );
}

function countFilters(params: URLSearchParams) {
  let n = 0;
  for (const k of ["g", "gx", "t", "tx", "st", "len", "co", "score", "pop", "tier", "studio", "nosq", "hide"]) if (params.get(k)) n += k.length <= 2 && k !== "co" && k !== "st" ? list(params.get(k)).length : 1;
  if (params.get("fmt")) n++;
  return n;
}

function Presets({ current }: { current: URLSearchParams }) {
  const { data, retry } = useData(api.presets, []);
  const mine = data?.presets ?? [];
  const here = new URLSearchParams([...current].filter(([k]) => KEYS.includes(k) && k !== "q")).toString();

  const apply = (query: string) => {
    const changes: Record<string, string | null> = {};
    for (const k of KEYS) changes[k] = null;
    for (const [k, v] of new URLSearchParams(query)) changes[k] = v;
    setParams(changes);
  };
  const save = async () => {
    const name = prompt("Nama filter ini", "")?.trim();
    if (!name) return;
    try {
      await api.setPresets([...mine.filter((x) => x.name !== name), { name, query: here }]);
      toast(`Filter “${name}” disimpan`);
      retry();
    } catch (e) {
      toast((e as Error).message);
    }
  };
  const remove = async (name: string) => {
    try {
      await api.setPresets(mine.filter((x) => x.name !== name));
      toast(`Filter “${name}” dihapus`);
      retry();
    } catch (e) {
      toast((e as Error).message);
    }
  };

  return (
    <div className="presets" aria-label="Filter siap pakai">
      {PRESETS.map((x) => (
        <Chip key={x.name} on={here === new URLSearchParams(x.query).toString()} onClick={() => apply(x.query)}>
          {x.name}
        </Chip>
      ))}
      {mine.map((x) => (
        <span key={x.name} className="mine-preset">
          <Chip on={here === x.query} onClick={() => apply(x.query)}>
            {x.name}
          </Chip>
          <button type="button" className="x" aria-label={`Hapus filter ${x.name}`} onClick={() => remove(x.name)}>
            ×
          </button>
        </span>
      ))}
      <button type="button" className="link" onClick={save} disabled={!here}>
        Simpan filter ini
      </button>
    </div>
  );
}

function Filters({ meta, params }: { meta: Meta; params: URLSearchParams }) {
  const [tagSearch, setTagSearch] = useState("");
  const get = (k: string) => list(params.get(k));
  const toggle = (k: string, v: string) => {
    const cur = get(k);
    setParams({ [k]: join(cur.includes(v) ? cur.filter((x) => x !== v) : [...cur, v]) });
  };
  // include → exclude → off
  const tri = (inK: string, outK: string, v: string) => {
    const inc = get(inK), exc = get(outK);
    if (inc.includes(v)) setParams({ [inK]: join(inc.filter((x) => x !== v)), [outK]: join([...exc, v]) });
    else if (exc.includes(v)) setParams({ [outK]: join(exc.filter((x) => x !== v)) });
    else setParams({ [inK]: join([...inc, v]) });
  };
  const fmt = params.get("fmt") === "all" ? [] : params.get("fmt") ? get("fmt") : DEFAULT_FORMATS;
  const toggleFmt = (f: string) => {
    const next = fmt.includes(f) ? fmt.filter((x) => x !== f) : [...fmt, f];
    const same = next.length === DEFAULT_FORMATS.length && DEFAULT_FORMATS.every((x) => next.includes(x));
    setParams({ fmt: same ? null : next.length ? next.join("|") : "all" });
  };
  const tags = [...new Set([...QUICK_TAGS, ...get("t"), ...get("tx")])];
  const flag = (k: string) => setParams({ [k]: params.get(k) ? null : "1" });

  return (
    <div className="filters" id="filters">
      <fieldset>
        <legend>Genre</legend>
        <p className="hint">Klik sekali wajib ada, dua kali kecualikan, tiga kali batal.</p>
        <div className="chips">
          {meta.genres.map((g) => (
            <Chip key={g} on={get("g").includes(g)} off={get("gx").includes(g)} onClick={() => tri("g", "gx", g)}>
              {g}
            </Chip>
          ))}
        </div>
      </fieldset>

      <fieldset>
        <legend>Tema</legend>
        <div className="chips">
          {tags.map((t) => (
            <Chip key={t} on={get("t").includes(t)} off={get("tx").includes(t)} onClick={() => tri("t", "tx", t)}>
              {t}
            </Chip>
          ))}
        </div>
        <input
          className="tag-search"
          list="tag-list"
          value={tagSearch}
          placeholder="Tambah tema lain, mis. Cooking"
          aria-label="Tambah tema"
          onChange={(e) => {
            const v = e.target.value;
            setTagSearch(v);
            const hit = meta.tags.find((t) => t.name.toLowerCase() === v.toLowerCase());
            if (hit) {
              if (!get("t").includes(hit.name)) setParams({ t: join([...get("t"), hit.name]) });
              setTagSearch("");
            }
          }}
        />
        <datalist id="tag-list">
          {meta.tags.map((t) => (
            <option key={t.name} value={t.name} />
          ))}
        </datalist>
      </fieldset>

      <div className="filter-cols">
        <fieldset>
          <legend>Format</legend>
          <div className="chips">
            {FORMAT_KEYS.map((f) => (
              <Chip key={f} on={fmt.includes(f)} onClick={() => toggleFmt(f)}>
                {formatName(f)}
              </Chip>
            ))}
          </div>
        </fieldset>
        <fieldset>
          <legend>Status tayang</legend>
          <div className="chips">
            {["FINISHED", "RELEASING", "NOT_YET_RELEASED"].map((s) => (
              <Chip key={s} on={get("st").includes(s)} onClick={() => toggle("st", s)}>
                {AIR_STATUS[s]}
              </Chip>
            ))}
          </div>
        </fieldset>
        <fieldset>
          <legend>Panjang</legend>
          <div className="chips">
            {LENGTHS.map(([k, l]) => (
              <Chip key={k} on={params.get("len") === k} onClick={() => setParams({ len: params.get("len") === k ? null : k })}>
                {l}
              </Chip>
            ))}
          </div>
        </fieldset>
        <fieldset>
          <legend>Asal</legend>
          <div className="chips">
            {["JP", "CN", "KR"].map((c) => (
              <Chip key={c} on={get("co").includes(c)} onClick={() => toggle("co", c)}>
                {COUNTRIES[c]}
              </Chip>
            ))}
          </div>
        </fieldset>
        <fieldset>
          <legend>Layak tonton</legend>
          <div className="chips">
            {meta.tiers.map((t) => (
              <Chip key={t.id} on={params.get("tier") === t.id} onClick={() => setParams({ tier: params.get("tier") === t.id ? null : t.id })} title={t.name}>
                <span className={`tier t-${t.id}`}>{t.id}</span> {t.name}
              </Chip>
            ))}
          </div>
        </fieldset>
        <fieldset>
          <legend>Lainnya</legend>
          <div className="chips">
            <Chip on={!!params.get("nosq")} onClick={() => flag("nosq")}>
              Tanpa sekuel
            </Chip>
            <Chip on={!!params.get("hide")} onClick={() => flag("hide")}>
              Sembunyikan yang ada di daftarku
            </Chip>
          </div>
        </fieldset>
      </div>

      <div className="filter-cols">
        <label className="field">
          <span className="label">Skor minimum</span>
          <select value={params.get("score") ?? ""} onChange={(e) => setParams({ score: e.target.value || null })}>
            <option value="">Semua</option>
            {[60, 65, 70, 75, 80, 85].map((s) => (
              <option key={s} value={s}>
                ★ {(s / 10).toFixed(1).replace(".", ",")} ke atas
              </option>
            ))}
          </select>
        </label>
        <label className="field">
          <span className="label">Popularitas minimum</span>
          <select value={params.get("pop") ?? ""} onChange={(e) => setParams({ pop: e.target.value || null })}>
            <option value="">Semua</option>
            {[1000, 5000, 20000, 50000, 100000].map((n) => (
              <option key={n} value={n}>
                {n.toLocaleString("id-ID")}+ penonton AniList
              </option>
            ))}
          </select>
        </label>
        <label className="field">
          <span className="label">Studio</span>
          <input list="studio-list" defaultValue={params.get("studio") ?? ""} placeholder="mis. MAPPA" onChange={(e) => setParams({ studio: e.target.value.trim() || null })} />
          <datalist id="studio-list">
            {meta.studios.map((s) => (
              <option key={s} value={s} />
            ))}
          </datalist>
        </label>
      </div>

      <button
        type="button"
        className="link"
        onClick={() => setParams(Object.fromEntries(["g", "gx", "t", "tx", "fmt", "st", "len", "co", "score", "pop", "tier", "studio", "nosq", "hide"].map((k) => [k, null])))}
      >
        Hapus semua filter
      </button>
    </div>
  );
}

function Results({ query, view }: { query: string; view: "grid" | "list" }) {
  const { data, error, retry, loading } = useData(() => api.browse(query + "&page=1"), [query]);
  const [more, setMore] = useState<Item[]>([]);
  const [page, setPage] = useState(1);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    setMore([]);
    setPage(1);
  }, [query]);

  if (!data) return error ? <Failed message={error} retry={retry} /> : <Loading />;
  const items = [...data.items, ...more];
  const loadMore = async () => {
    setBusy(true);
    try {
      const next = await api.browse(`${query}&page=${page + 1}`);
      setMore((m) => [...m, ...next.items]);
      setPage(page + 1);
    } catch (e) {
      toast((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <p className="count" aria-live="polite">
        {data.total.toLocaleString("id-ID")} judul
      </p>
      {data.total === 0 ? (
        <p className="empty">Tidak ada judul yang cocok. Longgarkan filternya atau ganti musim.</p>
      ) : view === "grid" ? (
        <PosterGrid items={items} stale={loading} />
      ) : (
        <TitleTable items={items} stale={loading} />
      )}
      {items.length < data.total && (
        <div className="more">
          <button type="button" className="btn" onClick={loadMore} disabled={busy}>
            Muat {Math.min(data.size, data.total - items.length)} lagi
          </button>
        </div>
      )}
    </>
  );
}
