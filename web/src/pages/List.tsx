import { useMemo, useState } from "react";
import { api } from "../api";
import { useData } from "../data";
import { displayTitle, facts, STATUS, STATUS_ORDER } from "../format";
import { Row } from "../components/Row";
import { Failed, Loading } from "../components/States";
import { Bars, Columns, Heatmap, Progress, Stat, Tier } from "../components/ui";
import { setParams, useLocation } from "../router";
import type { Item, Status } from "../types";

type Sort = "updated" | "title" | "rating" | "worth";
type Tab = Status | "stats";

function sub(it: Item) {
  const e = it.entry!;
  const parts: string[] = [];
  if ((e.status === "watching" || e.status === "hold" || e.status === "drop") && e.episode)
    parts.push(`episode ${e.episode}${it.anime.episodes ? ` dari ${it.anime.episodes}` : ""}`);
  if (e.note) parts.push(e.note);
  return parts.length ? parts.join(". ") : facts(it.anime);
}

export function List() {
  const { params } = useLocation();
  const raw = params.get("status") ?? "";
  const tab: Tab = raw === "stats" ? "stats" : (STATUS_ORDER as string[]).includes(raw) ? (raw as Status) : "watching";
  const { data, error, retry } = useData(api.list, []);

  if (!data) return error ? <Failed message={error} retry={retry} /> : <Loading />;

  return (
    <section className="block">
      <h1 className="sr-only">Daftarku</h1>
      <nav className="tabs" aria-label="Status">
        {STATUS_ORDER.map((s) => (
          <button key={s} type="button" className={`s-${s}`} aria-current={s === tab ? "page" : undefined} onClick={() => setParams({ status: s === "watching" ? null : s })}>
            <i className="dot" aria-hidden="true" />
            {STATUS[s]} <span className="n">{data.counts[s] ?? 0}</span>
          </button>
        ))}
        <button type="button" aria-current={tab === "stats" ? "page" : undefined} onClick={() => setParams({ status: "stats" })}>
          Statistik
        </button>
      </nav>
      {tab === "stats" ? <Stats /> : <StatusList items={data.items} status={tab} />}
    </section>
  );
}

function StatusList({ items, status }: { items: Item[]; status: Status }) {
  const [filter, setFilter] = useState("");
  const [sort, setSort] = useState<Sort>("updated");
  const [prioOnly, setPrioOnly] = useState(false);

  const rows = useMemo(() => {
    const q = filter.trim().toLowerCase();
    const out = items.filter(
      (it) =>
        it.entry?.status === status &&
        (!prioOnly || it.entry.priority) &&
        (!q || [it.anime.title, it.anime.english, it.entry.note].some((s) => s?.toLowerCase().includes(q))),
    );
    if (sort === "title") out.sort((a, b) => displayTitle(a.anime).localeCompare(displayTitle(b.anime)));
    if (sort === "rating") out.sort((a, b) => (b.entry?.rating ?? 0) - (a.entry?.rating ?? 0) || (b.anime.score ?? 0) - (a.anime.score ?? 0));
    if (sort === "worth") out.sort((a, b) => (b.anime.worth ?? 0) - (a.anime.worth ?? 0));
    if (sort === "updated") out.sort((a, b) => Number(!!b.entry?.priority) - Number(!!a.entry?.priority) || (b.entry?.updated ?? 0) - (a.entry?.updated ?? 0));
    return out;
  }, [items, status, filter, sort, prioOnly]);

  const prioCount = items.filter((it) => it.entry?.status === status && it.entry.priority).length;

  return (
    <>
      <div className="toolbar">
        <input type="search" value={filter} onChange={(e) => setFilter(e.target.value)} placeholder="Saring judul atau catatan" aria-label="Saring daftar" />
        <label className="select">
          <span className="sr-only">Urutkan</span>
          <select value={sort} onChange={(e) => setSort(e.target.value as Sort)}>
            <option value="updated">Prioritas, lalu terbaru</option>
            <option value="title">Judul A–Z</option>
            <option value="rating">Nilaiku tertinggi</option>
            <option value="worth">Paling layak tonton</option>
          </select>
        </label>
        {prioCount > 0 && (
          <button type="button" className={`btn${prioOnly ? " solid" : ""}`} aria-pressed={prioOnly} onClick={() => setPrioOnly(!prioOnly)}>
            Prioritas ({prioCount})
          </button>
        )}
      </div>

      {rows.length > 0 ? (
        <ul className="rows">
          {rows.map((it) => (
            <Row
              key={it.anime.id}
              item={it}
              sub={
                <>
                  {it.entry?.priority && <span className="prio">Prioritas</span>}
                  {sub(it)}
                </>
              }
              below={status === "watching" ? <Progress value={it.entry?.episode ?? 0} total={it.anime.episodes ?? 0} color={it.anime.color} /> : undefined}
              aside={
                <span className="actions">
                  <Tier tier={it.anime.tier} />
                  {it.entry?.rating ? <span className="score">{it.entry.rating}</span> : null}
                </span>
              }
            />
          ))}
        </ul>
      ) : (
        <p className="empty">{filter ? `Tidak ada yang cocok dengan “${filter}”.` : `Belum ada judul berstatus ${STATUS[status].toLowerCase()}.`}</p>
      )}

      <p className="export">
        Unduh seluruh daftarmu: <a href="/api/export?format=csv">CSV (Excel)</a> atau <a href="/api/export?format=json">JSON</a>
      </p>
    </>
  );
}

function Stats() {
  const { data, error, retry } = useData(api.stats, []);
  if (!data) return error ? <Failed message={error} retry={retry} /> : <Loading />;
  const done = data.counts.done ?? 0;
  return (
    <>
      <div className="stats">
        <Stat label="Selesai" value={done} note={`${data.counts.watching ?? 0} sedang ditonton`} />
        <Stat label="Episode ditonton" value={data.episodes.toLocaleString("id-ID")} note={`±${data.hours.toLocaleString("id-ID")} jam`} />
        <Stat label="Rata-rata nilaimu" value={data.rated ? data.avgRating.toFixed(1).replace(".", ",") : "–"} note={`${data.rated} judul dinilai`} />
        <Stat label="Beruntun" value={`${data.streak} hari`} note={`rekor sehari ${data.bestDay} ep`} />
      </div>
      <div className="two">
        <section className="block">
          <h2>Genre yang paling sering kamu tonton</h2>
          {data.genres.length ? <Bars rows={data.genres} colorful /> : <p className="quiet">Belum ada judul selesai atau sedang ditonton.</p>}
        </section>
        <section className="block">
          <h2>Studio favorit</h2>
          {data.studios.length ? <Bars rows={data.studios} /> : <p className="quiet">Belum ada data.</p>}
        </section>
      </div>
      {data.years.length > 0 && (
        <section className="block">
          <h2>Tontonanmu menurut tahun rilis</h2>
          <Columns rows={data.years} />
        </section>
      )}
      <section className="block">
        <h2>Aktivitas 12 minggu</h2>
        <Heatmap days={data.activity} />
      </section>
    </>
  );
}
