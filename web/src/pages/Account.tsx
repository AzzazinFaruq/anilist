import { useState, type FormEvent } from "react";
import { api } from "../api";
import { Failed, Loading } from "../components/States";
import { toast, useData } from "../data";
import type { Me } from "../types";

type Theme = "system" | "light" | "dark";

function readTheme(): Theme {
  try {
    const t = localStorage.getItem("tonton-theme");
    return t === "light" || t === "dark" ? t : "system";
  } catch {
    return "system";
  }
}

export function Account({ me, onLogout }: { me: Me; onLogout: () => void }) {
  const [theme, setTheme] = useState<Theme>(readTheme);
  const pickTheme = (t: Theme) => {
    setTheme(t);
    if (t === "system") delete document.documentElement.dataset.theme;
    else document.documentElement.dataset.theme = t;
    try {
      if (t === "system") localStorage.removeItem("tonton-theme");
      else localStorage.setItem("tonton-theme", t);
    } catch {
      /* remembered for this visit only */
    }
  };

  return (
    <>
      <section className="block">
        <h1>{me.name}</h1>
        <p className="quiet">{me.admin ? "Admin keluarga" : "Anggota keluarga"}</p>
      </section>

      <section className="block">
        <h2>Tampilan</h2>
        <div className="seg" role="group" aria-label="Tema">
          {(
            [
              ["system", "Ikuti perangkat"],
              ["light", "Terang"],
              ["dark", "Gelap"],
            ] as [Theme, string][]
          ).map(([t, label]) => (
            <button key={t} type="button" aria-pressed={theme === t} onClick={() => pickTheme(t)}>
              {label}
            </button>
          ))}
        </div>
      </section>

      <ChangePIN />
      {me.admin && <Family me={me} />}

      <section className="block">
        <button type="button" className="btn" onClick={onLogout}>
          Keluar
        </button>
      </section>
    </>
  );
}

function ChangePIN() {
  const [old, setOld] = useState("");
  const [next, setNext] = useState("");
  const [msg, setMsg] = useState("");
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setMsg("");
    try {
      await api.changePIN(old, next);
      setOld("");
      setNext("");
      toast("PIN diganti. Perangkat lain perlu masuk ulang.");
    } catch (err) {
      setMsg((err as Error).message);
    }
  };
  return (
    <section className="block">
      <h2>Ganti PIN</h2>
      <form className="inline-form" onSubmit={submit}>
        <label className="field">
          <span className="label">PIN sekarang</span>
          <input type="password" inputMode="numeric" autoComplete="current-password" value={old} onChange={(e) => setOld(e.target.value)} required />
        </label>
        <label className="field">
          <span className="label">PIN baru</span>
          <input type="password" inputMode="numeric" autoComplete="new-password" minLength={4} maxLength={32} value={next} onChange={(e) => setNext(e.target.value)} required />
        </label>
        <button className="btn" type="submit">
          Ganti PIN
        </button>
      </form>
      {msg && <p className="error" role="alert">{msg}</p>}
    </section>
  );
}

function Family({ me }: { me: Me }) {
  const { data, error, retry } = useData(api.members, []);
  const [name, setName] = useState("");
  const [pin, setPin] = useState("");
  const [msg, setMsg] = useState("");
  const reload = retry;

  const add = async (e: FormEvent) => {
    e.preventDefault();
    setMsg("");
    try {
      const m = await api.addMember(name, pin, false);
      setName("");
      setPin("");
      toast(`${m.name} ditambahkan`);
      reload();
    } catch (err) {
      setMsg((err as Error).message);
    }
  };

  const reset = async (id: string, who: string) => {
    const p = prompt(`PIN baru untuk ${who} (4–32 karakter)`);
    if (!p) return;
    try {
      await api.resetPIN(id, p);
      toast(`PIN ${who} diganti`);
    } catch (err) {
      toast((err as Error).message);
    }
  };

  const remove = async (id: string, who: string) => {
    if (!confirm(`Hapus akun ${who} beserta daftarnya? Salinan cadangan tetap disimpan di server.`)) return;
    try {
      await api.removeMember(id);
      toast(`${who} dihapus`);
      reload();
    } catch (err) {
      toast((err as Error).message);
    }
  };

  return (
    <section className="block">
      <h2>Anggota keluarga</h2>
      {!data ? (
        error ? <Failed message={error} retry={retry} /> : <Loading />
      ) : (
        <ul className="members">
          {data.users.map((u) => (
            <li key={u.id}>
              <span>
                {u.name}
                {u.admin && <span className="quiet"> (admin)</span>}
              </span>
              {u.id !== me.id && (
                <span className="member-actions">
                  <button type="button" className="link" onClick={() => reset(u.id, u.name)}>
                    Atur ulang PIN
                  </button>
                  <button type="button" className="link danger" onClick={() => remove(u.id, u.name)}>
                    Hapus
                  </button>
                </span>
              )}
            </li>
          ))}
        </ul>
      )}
      <form className="inline-form" onSubmit={add}>
        <label className="field">
          <span className="label">Nama</span>
          <input value={name} onChange={(e) => setName(e.target.value)} maxLength={24} required />
        </label>
        <label className="field">
          <span className="label">PIN awal</span>
          <input type="password" inputMode="numeric" autoComplete="new-password" minLength={4} maxLength={32} value={pin} onChange={(e) => setPin(e.target.value)} required />
        </label>
        <button className="btn" type="submit">
          Tambah anggota
        </button>
      </form>
      {msg && <p className="error" role="alert">{msg}</p>}
    </section>
  );
}
