import { useEffect, useRef, useState, type CSSProperties } from "react";
import { api } from "../api";
import { bump, change, removeEntry, toast, useData } from "../data";
import { airTime, COUNTRIES, displayTitle, facts, score10, STATUS, STATUS_ORDER } from "../format";
import { closeTitle, openTitle } from "../router";
import type { Status } from "../types";
import { Poster } from "./Poster";
import { Failed, Loading } from "./States";
import { Stepper } from "./Stepper";
import { Tier } from "./ui";

export function DetailSheet({ id }: { id: number }) {
  const ref = useRef<HTMLDialogElement>(null);
  const { data, error, retry } = useData(() => api.detail(id), [id]);

  useEffect(() => {
    const d = ref.current;
    if (d && !d.open) {
      d.showModal();
      d.focus(); // start on the sheet itself, not on the first button
    }
  }, []);

  return (
    <dialog
      ref={ref}
      className="sheet"
      aria-label="Detail judul"
      tabIndex={-1}
      onCancel={(e) => {
        e.preventDefault();
        closeTitle();
      }}
      onClick={(e) => {
        if (e.target === ref.current) closeTitle(); // click on the backdrop
      }}
    >
      <div className="sheet-body" style={data?.anime.color ? ({ "--c": data.anime.color } as CSSProperties) : undefined}>
        <button type="button" className="close" onClick={closeTitle} aria-label="Tutup">
          Tutup
        </button>
        {error && !data && <Failed message={error} retry={retry} />}
        {!data && !error && <Loading />}
        {data && data.anime.id === id && <Body {...data} key={id} />}
      </div>
    </dialog>
  );
}

type DetailData = Awaited<ReturnType<typeof api.detail>>;

function Body({ anime: a, entry, next }: DetailData) {
  const [note, setNote] = useState(entry?.note ?? "");
  const [more, setMore] = useState(false);
  useEffect(() => setNote(entry?.note ?? ""), [entry?.note]);
  const title = displayTitle(a);
  const others = [a.title, a.english, a.native].filter((t) => t && t !== title);

  const setStatus = (s: Status) => change(a.id, { status: s });
  const setRating = (r: number) => change(a.id, { rating: entry?.rating === r ? 0 : r });
  const saveNote = () => {
    if ((entry?.note ?? "") !== note.trim()) change(a.id, { note });
  };
  const notInterested = async () => {
    try {
      await api.hide(a.id, true);
      bump();
      toast("Ditandai tidak tertarik. Temuan tidak akan menyarankannya lagi.");
      closeTitle();
    } catch (e) {
      toast((e as Error).message);
    }
  };

  const synopsis = a.synopsis ?? "";
  const long = synopsis.length > 420;
  const meta = [
    a.studios?.join(", "),
    a.source,
    a.country && a.country !== "JP" ? COUNTRIES[a.country] ?? a.country : "",
    a.score ? `★ ${score10(a.score)}` : "",
    a.popularity ? `${a.popularity.toLocaleString("id-ID")} penonton AniList` : "",
  ].filter(Boolean);

  return (
    <article className="detail">
      <header className="detail-head">
        <Poster src={a.image} size="large" className="lit" />
        <div>
          <h2>{title}</h2>
          {others.length > 0 && <p className="quiet alt">{others.join(" / ")}</p>}
          <p className="facts">{facts(a)}</p>
          <p className="quiet">{meta.join(". ")}</p>
          {a.tier && (
            <p className="tier-line">
              <Tier tier={a.tier} /> Layak tonton {a.worth ? Math.round(a.worth) : ""}
            </p>
          )}
          {next && (
            <p className="facts">
              Episode {next.episode} tayang {airTime(next.at).toLowerCase()}
            </p>
          )}
        </div>
      </header>

      <section className="mine" aria-label="Daftarmu">
        <div className="seg status-seg" role="group" aria-label="Status">
          {STATUS_ORDER.map((s) => (
            <button key={s} type="button" className={`s-${s}`} aria-pressed={entry?.status === s} onClick={() => setStatus(s)}>
              <i className="dot" aria-hidden="true" />
              {STATUS[s]}
            </button>
          ))}
        </div>

        {entry ? (
          <>
            {a.format !== "MOVIE" && (
              <div className="field">
                <span className="label">Episode</span>
                <Stepper anime={a} entry={entry} size="small" />
              </div>
            )}
            <div className="field">
              <span className="label" id="rating-label">
                Nilai
              </span>
              <div className="rating" role="group" aria-labelledby="rating-label">
                {Array.from({ length: 10 }, (_, i) => i + 1).map((r) => (
                  <button key={r} type="button" aria-pressed={entry.rating === r} onClick={() => setRating(r)}>
                    {r}
                  </button>
                ))}
              </div>
            </div>
            <div className="field">
              <span className="label">Prioritas</span>
              <button type="button" className={`btn small${entry.priority ? " solid prio-on" : ""}`} aria-pressed={!!entry.priority} onClick={() => change(a.id, { priority: !entry.priority })}>
                {entry.priority ? "Diprioritaskan" : "Tonton duluan"}
              </button>
            </div>
            <label className="field">
              <span className="label">Catatan</span>
              <textarea value={note} maxLength={500} rows={2} onChange={(e) => setNote(e.target.value)} onBlur={saveNote} placeholder="Ditonton bareng siapa, sampai mana, kenapa berhenti…" />
            </label>
          </>
        ) : (
          <button type="button" className="link" onClick={notInterested}>
            Tidak tertarik
          </button>
        )}
      </section>

      {synopsis && (
        <section className="synopsis">
          <p className={long && !more ? "clamp" : ""}>{synopsis}</p>
          {long && (
            <button type="button" className="link" onClick={() => setMore(!more)}>
              {more ? "Ringkas" : "Baca semua"}
            </button>
          )}
          {a.synopsisLang === "en" && <p className="quiet small">Sinopsis dari AniList, dalam bahasa Inggris.</p>}
        </section>
      )}

      {!!(a.genres?.length || a.tags?.length) && (
        <section>
          <h3>Genre dan tema</h3>
          <div className="tags">
            {a.genres?.map((g) => <span key={g} className="tag-genre">{g}</span>)}
            {a.tags?.slice(0, 12).map((t) => <span key={t}>{t}</span>)}
          </div>
        </section>
      )}

      {(a.trailer || (a.links && a.links.length > 0)) && (
        <section>
          <h3>Tonton</h3>
          <ul className="links">
            {a.trailer && (
              <li>
                <a href={a.trailer} target="_blank" rel="noopener noreferrer" className="trailer">
                  Trailer di YouTube
                </a>
              </li>
            )}
            {a.links?.map((l) => (
              <li key={l.url}>
                <a href={l.url} target="_blank" rel="noopener noreferrer">
                  {l.site}
                </a>
              </li>
            ))}
          </ul>
        </section>
      )}

      <Similar id={a.id} />

      <footer className="detail-foot">
        <a href={`https://anilist.co/anime/${a.id}`} target="_blank" rel="noopener noreferrer" className="quiet">
          Buka di AniList
        </a>
        {entry && (
          <button type="button" className="link danger" onClick={() => removeEntry(a.id)}>
            Hapus dari daftar
          </button>
        )}
      </footer>
    </article>
  );
}

function Similar({ id }: { id: number }) {
  const { data } = useData(() => api.similar(id), [id]);
  if (!data?.items.length) return null;
  return (
    <section>
      <h3>Mirip dengan ini</h3>
      <ul className="strip">
        {data.items.map((it) => (
          <li key={it.anime.id}>
            <button type="button" onClick={() => openTitle(it.anime.id)}>
              <Poster src={it.anime.image} />
              <span className="strip-title">{displayTitle(it.anime)}</span>
              {it.entry && <span className={`strip-status s-${it.entry.status}`}>{STATUS[it.entry.status]}</span>}
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}
