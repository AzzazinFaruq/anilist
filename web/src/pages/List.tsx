import { useMemo, useState } from "react";
import { api } from "../api";
import { useData } from "../data";
import { displayTitle, facts, STATUS, STATUS_ORDER } from "../format";
import { Row } from "../components/Row";
import { Failed, Loading } from "../components/States";
import { setParam, useLocation } from "../router";
import type { Item, Status } from "../types";

type Sort = "updated" | "title" | "rating";

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
  const status = (STATUS_ORDER as string[]).includes(params.get("status") ?? "") ? (params.get("status") as Status) : "watching";
  const [filter, setFilter] = useState("");
  const [sort, setSort] = useState<Sort>("updated");
  const { data, error, retry } = useData(api.list, []);

  const rows = useMemo(() => {
    if (!data) return [];
    const q = filter.trim().toLowerCase();
    const out = data.items.filter(
      (it) =>
        it.entry?.status === status &&
        (!q || [it.anime.title, it.anime.english, it.entry.note].some((s) => s?.toLowerCase().includes(q))),
    );
    if (sort === "title") out.sort((a, b) => displayTitle(a.anime).localeCompare(displayTitle(b.anime)));
    if (sort === "rating") out.sort((a, b) => (b.entry?.rating ?? 0) - (a.entry?.rating ?? 0) || (b.anime.score ?? 0) - (a.anime.score ?? 0));
    return out;
  }, [data, status, filter, sort]);

  if (!data) return error ? <Failed message={error} retry={retry} /> : <Loading />;

  return (
    <section className="block">
      <h1 className="sr-only">Daftarku</h1>
      <nav className="tabs" aria-label="Status">
        {STATUS_ORDER.map((s) => (
          <button key={s} type="button" aria-current={s === status ? "page" : undefined} onClick={() => setParam("status", s === "watching" ? null : s)}>
            {STATUS[s]} <span className="n">{data.counts[s] ?? 0}</span>
          </button>
        ))}
      </nav>

      <div className="toolbar">
        <input type="search" value={filter} onChange={(e) => setFilter(e.target.value)} placeholder="Saring judul atau catatan" aria-label="Saring daftar" />
        <label className="select">
          <span className="sr-only">Urutkan</span>
          <select value={sort} onChange={(e) => setSort(e.target.value as Sort)}>
            <option value="updated">Terakhir diubah</option>
            <option value="title">Judul A–Z</option>
            <option value="rating">Nilai tertinggi</option>
          </select>
        </label>
      </div>

      {rows.length > 0 ? (
        <ul className="rows">
          {rows.map((it) => (
            <Row key={it.anime.id} item={it} sub={sub(it)} aside={it.entry?.rating ? <span className="score">{it.entry.rating}</span> : undefined} />
          ))}
        </ul>
      ) : (
        <div className="empty">
          <p>{filter ? `Tidak ada yang cocok dengan “${filter}”.` : `Belum ada judul berstatus ${STATUS[status].toLowerCase()}.`}</p>
        </div>
      )}
    </section>
  );
}
