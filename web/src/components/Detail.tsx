import { useEffect, useRef, useState } from "react";
import { api } from "../api";
import { change, removeEntry, useData } from "../data";
import { airTime, displayTitle, facts, STATUS, STATUS_ORDER } from "../format";
import { closeTitle } from "../router";
import type { Status } from "../types";
import { Poster } from "./Poster";
import { Failed, Loading } from "./States";
import { Stepper } from "./Stepper";

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
      <div className="sheet-body">
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

  const synopsis = a.synopsis ?? "";
  const long = synopsis.length > 420;

  return (
    <article className="detail">
      <header className="detail-head">
        <Poster src={a.image} size="large" className="lit" />
        <div>
          <h2>{title}</h2>
          {others.length > 0 && <p className="quiet alt">{others.join(" / ")}</p>}
          <p className="facts">{facts(a)}</p>
          <p className="quiet">
            {[a.studios?.join(", "), a.genres?.join(", "), a.score ? `skor ${a.score}` : ""].filter(Boolean).join(". ")}
          </p>
          {next && (
            <p className="facts">
              Episode {next.episode} tayang {airTime(next.at).toLowerCase()}
            </p>
          )}
        </div>
      </header>

      <section className="mine" aria-label="Daftarmu">
        <div className="seg" role="group" aria-label="Status">
          {STATUS_ORDER.map((s) => (
            <button key={s} type="button" aria-pressed={entry?.status === s} onClick={() => setStatus(s)}>
              {STATUS[s]}
            </button>
          ))}
        </div>

        {entry && (
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
            <label className="field">
              <span className="label">Catatan</span>
              <textarea value={note} maxLength={500} rows={2} onChange={(e) => setNote(e.target.value)} onBlur={saveNote} placeholder="Ditonton bareng siapa, sampai mana, kenapa berhenti…" />
            </label>
          </>
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

      {a.links && a.links.length > 0 && (
        <section>
          <h3>Tonton di</h3>
          <ul className="links">
            {a.links.map((l) => (
              <li key={l.url}>
                <a href={l.url} target="_blank" rel="noopener noreferrer">
                  {l.site}
                </a>
              </li>
            ))}
          </ul>
        </section>
      )}

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
