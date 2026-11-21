import { api } from "../api";
import { useData } from "../data";
import { airTime, displayTitle, facts } from "../format";
import { Row } from "../components/Row";
import { Failed, Loading } from "../components/States";
import { Stepper } from "../components/Stepper";
import { navigate, openTitle } from "../router";

export function Home() {
  const { data, error, retry } = useData(api.home, []);
  if (!data) return error ? <Failed message={error} retry={retry} /> : <Loading />;

  const { watching, upcoming, plan, today, week } = data;
  const empty = watching.length === 0 && plan.length === 0;

  return (
    <>
      <section className="block">
        <div className="block-head">
          <h1>Sedang ditonton</h1>
          {(today > 0 || week > 0) && (
            <p className="quiet">
              {today} episode hari ini, {week} minggu ini
            </p>
          )}
        </div>
        {watching.length > 0 ? (
          <ul className="rows">
            {watching.map((it) => (
              <Row
                key={it.anime.id}
                item={it}
                sub={it.next ? `Episode ${it.next.episode} tayang ${airTime(it.next.at).toLowerCase()}` : facts(it.anime)}
                aside={<Stepper anime={it.anime} entry={it.entry} />}
              />
            ))}
          </ul>
        ) : (
          <div className="empty">
            <p>{empty ? "Daftarmu masih kosong." : "Belum ada yang sedang ditonton."}</p>
            <p className="quiet">
              {empty ? "Cari judul, lalu tandai Rencana atau Ditonton." : "Buka judul di rencana dan tekan + saat mulai menonton."}
            </p>
            <button type="button" className="btn" onClick={() => navigate("/cari")}>
              Cari judul
            </button>
          </div>
        )}
      </section>

      {upcoming.length > 0 && (
        <section className="block">
          <h2>Tayang 7 hari ke depan</h2>
          <ol className="schedule">
            {upcoming.map((it) => (
              <li key={it.anime.id}>
                <button type="button" onClick={() => openTitle(it.anime.id)}>
                  <time dateTime={new Date(it.next!.at * 1000).toISOString()}>{airTime(it.next!.at)}</time>
                  <span className="sched-title">{displayTitle(it.anime)}</span>
                  <span className="quiet">ep {it.next!.episode}</span>
                </button>
              </li>
            ))}
          </ol>
        </section>
      )}

      {plan.length > 0 && (
        <section className="block">
          <div className="block-head">
            <h2>Rencana terbaru</h2>
            <button type="button" className="link" onClick={() => navigate("/daftar?status=plan")}>
              Semua rencana ({data.counts.plan ?? 0})
            </button>
          </div>
          <ul className="rows">
            {plan.map((it) => (
              <Row key={it.anime.id} item={it} sub={it.entry?.note || facts(it.anime)} />
            ))}
          </ul>
        </section>
      )}
    </>
  );
}
