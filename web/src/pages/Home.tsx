import { useState } from "react";
import { api } from "../api";
import { change, toast, useData } from "../data";
import { airTime, clock, dayLabel, displayTitle, facts, pct } from "../format";
import { Row } from "../components/Row";
import { Failed, Loading } from "../components/States";
import { Stepper } from "../components/Stepper";
import { Heatmap, Progress, SeasonName, Stat, StatusTag } from "../components/ui";
import { navigate, openTitle } from "../router";
import type { Item } from "../types";

export function Home() {
  const { data, error, retry } = useData(api.home, []);
  if (!data) return error ? <Failed message={error} retry={retry} /> : <Loading />;
  const { watching, queue, decisions, waiting, counts } = data;
  const empty = watching.length === 0 && queue.length === 0 && !counts.done;

  return (
    <>
      <div className="stats">
        <Stat label="Sedang ditonton" value={watching.length} note={data.ready ? `${data.ready} episode siap ditonton` : "tidak ada yang tertinggal"} />
        <Stat label="Minggu ini" value={`${data.week} ep`} note={`minggu lalu ${data.lastWeek} ep`} />
        <Stat label="Beruntun" value={`${data.streak} hari`} note={data.today ? `${data.today} ep hari ini` : "belum nonton hari ini"} />
        <Stat label="Antrean" value={data.queueTotal} note={waiting.length ? `${waiting.length}+ menunggu rilis` : "siap ditonton"} />
      </div>

      <section className="block">
        <h1>Lanjutkan menonton</h1>
        {watching.length > 0 ? (
          <ul className="rows">
            {watching.map((it) => (
              <Row
                key={it.anime.id}
                item={it}
                sub={watchSub(it)}
                below={<Progress value={it.entry?.episode ?? 0} total={it.anime.episodes || it.released || 0} color={it.anime.color} />}
                aside={<Stepper anime={it.anime} entry={it.entry} />}
              />
            ))}
          </ul>
        ) : (
          <div className="empty">
            <p>{empty ? "Daftarmu masih kosong." : "Belum ada yang sedang ditonton."}</p>
            <p className="quiet">{empty ? "Jelajahi musim ini atau cek Temuan, lalu tandai Rencana." : "Pilih satu dari antrean di bawah dan tekan Mulai."}</p>
            <button type="button" className="btn" onClick={() => navigate("/jelajah")}>
              Jelajahi anime
            </button>
          </div>
        )}
      </section>

      {decisions.length > 0 && (
        <section className="block">
          <h2>Perlu keputusan</h2>
          <p className="quiet small lead">Tontonan yang macet. Putuskan supaya antrean tetap rapi.</p>
          <ul className="rows">
            {decisions.map((d) => (
              <Row
                key={d.anime.id}
                item={d}
                sub={d.reason}
                aside={
                  <span className="actions">
                    {d.entry?.status !== "watching" && (
                      <button type="button" className="btn small" onClick={() => change(d.anime.id, { status: "watching" })}>
                        Lanjut
                      </button>
                    )}
                    {d.entry?.status !== "hold" && (
                      <button type="button" className="btn small" onClick={() => change(d.anime.id, { status: "hold" })}>
                        Tunda
                      </button>
                    )}
                    <button type="button" className="btn small" onClick={() => change(d.anime.id, { status: "drop" })}>
                      Berhenti
                    </button>
                  </span>
                }
              />
            ))}
          </ul>
        </section>
      )}

      <Queue items={queue} total={data.queueTotal} />
      <Schedule />

      <div className="two">
        <section className="block">
          <h2>Aktivitas 12 minggu</h2>
          <Heatmap days={data.activity} />
        </section>
        {waiting.length > 0 && (
          <section className="block">
            <h2>Menunggu rilis</h2>
            <ul className="mini-list">
              {waiting.map((it) => (
                <li key={it.anime.id}>
                  <button type="button" onClick={() => openTitle(it.anime.id)}>
                    <span className="mini-title">{displayTitle(it.anime)}</span>
                    <span className="quiet small">{it.next ? `ep ${it.next.episode} ${airTime(it.next.at).toLowerCase()}` : <SeasonName year={it.anime.year} season={it.anime.season} />}</span>
                  </button>
                </li>
              ))}
            </ul>
          </section>
        )}
      </div>
    </>
  );
}

function watchSub(it: Item) {
  const parts: string[] = [];
  const left = (it.released ?? 0) - (it.entry?.episode ?? 0);
  if (left > 0) parts.push(it.anime.status === "RELEASING" ? `${left} episode baru siap` : `${left} episode belum ditonton`);
  if (it.next) parts.push(`ep ${it.next.episode} tayang ${airTime(it.next.at).toLowerCase()}`);
  return parts.length ? parts.join(", ") : facts(it.anime);
}

function Queue({ items, total }: { items: Item[]; total: number }) {
  const [busy, setBusy] = useState(false);
  const gacha = async () => {
    setBusy(true);
    try {
      const { item } = await api.pick();
      openTitle(item.anime.id);
    } catch (e) {
      toast((e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  return (
    <section className="block">
      <div className="block-head">
        <h2>Antrean cerdas</h2>
        <span className="actions">
          <button type="button" className="btn small" onClick={gacha} disabled={busy || total === 0}>
            Pilih acak
          </button>
          <button type="button" className="link" onClick={() => navigate("/daftar?status=plan")}>
            Semua rencana ({total})
          </button>
        </span>
      </div>
      <p className="quiet small lead">Prioritas dulu, lalu yang paling cocok dengan seleramu menurut model Temuan.</p>
      {items.length ? (
        <ul className="rows">
          {items.map((it) => (
            <Row
              key={it.anime.id}
              item={it}
              sub={
                <>
                  {it.entry?.priority && <span className="prio">Prioritas</span>}
                  {it.match ? <span className="match">{pct(it.match)} cocok</span> : null}
                  {it.entry?.note || facts(it.anime)}
                </>
              }
              aside={
                <button type="button" className="btn small solid" onClick={() => change(it.anime.id, { status: "watching" })}>
                  Mulai
                </button>
              }
            />
          ))}
        </ul>
      ) : (
        <p className="empty">Antrean kosong. Tambahkan judul lewat Jelajah atau Temuan.</p>
      )}
    </section>
  );
}

function Schedule() {
  const [scope, setScope] = useState<"mine" | "all">("mine");
  const { data, error, retry } = useData(() => api.schedule(scope), [scope]);
  const days = new Map<string, Item[]>();
  for (const it of data?.items ?? []) {
    const d = new Date(it.next!.at * 1000);
    const key = d.toDateString();
    if (!days.has(key)) days.set(key, []);
    days.get(key)!.push(it);
  }
  return (
    <section className="block">
      <div className="block-head">
        <h2>Jadwal tayang 7 hari</h2>
        <div className="seg small" role="group" aria-label="Cakupan jadwal">
          <button type="button" aria-pressed={scope === "mine"} onClick={() => setScope("mine")}>
            Dari daftarku
          </button>
          <button type="button" aria-pressed={scope === "all"} onClick={() => setScope("all")}>
            Semua
          </button>
        </div>
      </div>
      {!data && (error ? <Failed message={error} retry={retry} /> : <Loading />)}
      {data && data.items.length === 0 && (
        <p className="empty">{scope === "mine" ? "Tidak ada judul di daftarmu yang tayang minggu ini." : "Jadwal tayang belum tersedia."}</p>
      )}
      {[...days].map(([key, items]) => (
        <div key={key} className="day">
          <h3>{dayLabel(items[0].next!.at)}</h3>
          <ol className="schedule">
            {items.map((it) => (
              <li key={`${it.anime.id}-${it.next!.episode}`}>
                <button type="button" onClick={() => openTitle(it.anime.id)}>
                  <time dateTime={new Date(it.next!.at * 1000).toISOString()}>{clock(it.next!.at)}</time>
                  <span className="sched-title">{displayTitle(it.anime)}</span>
                  <span className="sched-ep">
                    {it.entry && <StatusTag status={it.entry.status} />}
                    <span className="quiet">ep {it.next!.episode}</span>
                  </span>
                </button>
              </li>
            ))}
          </ol>
        </div>
      ))}
    </section>
  );
}
