import { useEffect, useState } from "react";
import { api } from "../api";
import { bump, change, toast, useData } from "../data";
import { facts, longDate, pct } from "../format";
import { Row } from "../components/Row";
import { Failed, Loading } from "../components/States";
import { Stat, StatusTag, Tier } from "../components/ui";
import type { Meta } from "../types";

type Filter = "open" | "all" | "yes" | "no";

const KIND: Record<string, string> = { baru: "Baru masuk", tayang: "Sedang tayang", segera: "Segera tayang", lama: "Judul lama" };

export function Temuan() {
  const [filter, setFilter] = useState<Filter>("open");
  const [tasteOpen, setTasteOpen] = useState(false);
  const { data, error, retry } = useData(() => api.temuan(filter), [filter]);
  if (!data) return error ? <Failed message={error} retry={retry} /> : <Loading />;
  const m = data.model;

  return (
    <>
      <section className="block">
        <h1>Temuan untukmu</h1>
        <p className="quiet lead">
          Tiap hari server melatih ulang model dari daftarmu: yang selesai, ditonton, dinilai, diprioritaskan, atau dihentikan. Tiap 7 hari model memilih
          judul baru, sedang tayang, dan beberapa judul lama yang cocok.
        </p>
      </section>

      {m ? (
        <div className="stats">
          <Stat label="Belum ditanggapi" value={data.counts.open} note={`dari ${data.counts.all} temuan`} />
          <Stat label="Akurasi model" value={m.auc ? `${Math.round(m.auc * 100)}%` : "–"} note="AUC di data uji" />
          <Stat label="Bahan belajar" value={m.positives} note={`judul disukai, ${m.negatives} ditolak`} />
          <Stat label="Temuan berikutnya" value={data.next ? longDate(data.next).split(",")[0] : "–"} note={data.next ? longDate(data.next) : ""} />
        </div>
      ) : (
        <div className="empty">
          <p>Model belum dilatih untuk akunmu.</p>
          <p className="quiet">Server melatihnya otomatis tiap hari. Isi daftarmu (Rencana, Selesai, nilai) dan atur selera di bawah supaya temuan pertama lebih tepat.</p>
        </div>
      )}

      {m && (m.likes?.length || m.dislikes?.length) ? (
        <div className="two">
          <section className="block">
            <h2>Seleramu menurut model</h2>
            <div className="why">{m.likes?.map((x) => <span key={x} className="why-yes">{x}</span>)}</div>
          </section>
          <section className="block">
            <h2>Yang cenderung kamu lewati</h2>
            <div className="why">{m.dislikes?.map((x) => <span key={x} className="why-no">{x}</span>)}</div>
          </section>
        </div>
      ) : null}

      <section className="block">
        <div className="block-head">
          <h2>Selera yang kamu atur sendiri</h2>
          <button type="button" className="btn small" aria-expanded={tasteOpen} onClick={() => setTasteOpen(!tasteOpen)}>
            {tasteOpen ? "Tutup" : `Atur selera${Object.keys(data.taste).length ? ` (${Object.keys(data.taste).length})` : ""}`}
          </button>
        </div>
        {!tasteOpen && <TasteSummary taste={data.taste} />}
        {tasteOpen && <TasteEditor initial={data.taste} onSaved={() => setTasteOpen(false)} />}
      </section>

      <section className="block">
        <div className="seg" role="group" aria-label="Saring temuan">
          {(
            [
              ["open", `Belum ditanggapi (${data.counts.open})`],
              ["all", `Semua (${data.counts.all})`],
              ["yes", `Diterima (${data.counts.yes})`],
              ["no", `Ditolak (${data.counts.no})`],
            ] as [Filter, string][]
          ).map(([k, l]) => (
            <button key={k} type="button" aria-pressed={filter === k} onClick={() => setFilter(k)}>
              {l}
            </button>
          ))}
        </div>
        {data.batches.length === 0 && <p className="empty">{filter === "open" ? "Semua temuan sudah kamu tanggapi. Batch berikutnya datang otomatis." : "Tidak ada temuan di sini."}</p>}
        {data.batches.map((b) => (
          <div key={b.date} className="batch">
            <h3>{longDate(b.date)}</h3>
            <ul className="rows">
              {b.items.map((it) => (
                <Row
                  key={it.anime.id}
                  item={it}
                  sub={
                    <>
                      <span className="match">{pct(it.match)} cocok</span>
                      {it.kind && KIND[it.kind] && <span className={`kind k-${it.kind}`}>{KIND[it.kind]}</span>}
                      {facts(it.anime)}
                    </>
                  }
                  below={
                    it.why?.length ? (
                      <span className="why">
                        {it.why.map((w) => (
                          <span key={w}>{w}</span>
                        ))}
                      </span>
                    ) : undefined
                  }
                  aside={<Answer id={it.anime.id} onList={!!it.entry} status={it.entry?.status} tier={it.anime.tier} filter={filter} />}
                />
              ))}
            </ul>
          </div>
        ))}
      </section>
    </>
  );
}

function Answer({ id, onList, status, tier, filter }: { id: number; onList: boolean; status?: string; tier?: string; filter: Filter }) {
  const hide = async (hidden: boolean) => {
    try {
      await api.hide(id, hidden);
      bump();
      toast(hidden ? "Ditandai tidak tertarik" : "Tanda tidak tertarik dibatalkan");
    } catch (e) {
      toast((e as Error).message);
    }
  };
  if (onList && status) return <span className="actions"><Tier tier={tier} /><StatusTag status={status as never} /></span>;
  if (filter === "no")
    return (
      <button type="button" className="btn small" onClick={() => hide(false)}>
        Batalkan
      </button>
    );
  return (
    <span className="actions stack">
      <button type="button" className="btn small solid" onClick={() => change(id, { status: "plan" })}>
        + Rencana
      </button>
      <button type="button" className="btn small" onClick={() => hide(true)}>
        Tidak tertarik
      </button>
    </span>
  );
}

const LEVELS: [number, string][] = [
  [3, "Suka"],
  [2, "Boleh"],
  [1, "Hindari"],
];
const PURE = "x:romansa-murni";
const TASTE_TAGS = ["Isekai", "Reincarnation", "Magic", "Military", "School", "Super Power", "Revenge", "Iyashikei", "Kingdom Management", "Time Loop", "Video Games", "Female Harem", "Male Harem", "Ecchi"];

const labelOf = (k: string) => (k === PURE ? "Romansa murni (tanpa harem/ecchi)" : k.slice(2));

function TasteSummary({ taste }: { taste: Record<string, number> }) {
  const groups = LEVELS.map(([lvl, name]) => [name, Object.keys(taste).filter((k) => taste[k] === lvl)] as const).filter(([, ks]) => ks.length);
  if (!groups.length) return <p className="quiet">Belum diatur. Model hanya belajar dari daftarmu.</p>;
  return (
    <div className="taste-summary">
      {groups.map(([name, ks]) => (
        <p key={name}>
          <b className={`lvl-${name.toLowerCase()}`}>{name}</b> {ks.map(labelOf).join(", ")}
        </p>
      ))}
    </div>
  );
}

function TasteEditor({ initial, onSaved }: { initial: Record<string, number>; onSaved: () => void }) {
  const { data: meta } = useData<Meta>(api.meta, []);
  const [taste, setTaste] = useState(initial);
  const [busy, setBusy] = useState(false);
  useEffect(() => setTaste(initial), [initial]);
  if (!meta) return <Loading />;

  const set = (k: string, v: number) => setTaste((t) => {
    const n = { ...t };
    if (n[k] === v) delete n[k];
    else n[k] = v;
    return n;
  });
  const save = async () => {
    setBusy(true);
    try {
      await api.setTaste(taste);
      bump();
      toast("Selera disimpan. Temuan minggu ini dipilih ulang dalam ±10 menit.");
      onSaved();
    } catch (e) {
      toast((e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  const row = (k: string) => (
    <li key={k}>
      <span>{labelOf(k)}</span>
      <span className="seg small" role="group" aria-label={labelOf(k)}>
        {LEVELS.map(([v, l]) => (
          <button key={v} type="button" className={`lvl-${l.toLowerCase()}`} aria-pressed={taste[k] === v} onClick={() => set(k, v)}>
            {l}
          </button>
        ))}
      </span>
    </li>
  );
  return (
    <div className="taste">
      <p className="quiet small">Tekan lagi pilihan yang aktif untuk mengosongkannya. Suka menaikkan skor, Hindari menurunkannya cukup kuat.</p>
      <div className="two">
        <div>
          <h3>Genre</h3>
          <ul className="taste-list">{meta.genres.map((g) => row("g:" + g))}</ul>
        </div>
        <div>
          <h3>Tema</h3>
          <ul className="taste-list">
            {row(PURE)}
            {TASTE_TAGS.map((t) => row("t:" + t))}
          </ul>
        </div>
      </div>
      <button type="button" className="btn solid" onClick={save} disabled={busy}>
        Simpan selera
      </button>
    </div>
  );
}
