import { useEffect, useRef, useState, type FormEvent } from "react";
import { api } from "../api";
import { Failed, Loading } from "../components/States";
import { useData } from "../data";
import type { Me } from "../types";

export function Login({ onDone }: { onDone: (me: Me) => void }) {
  const { data, error, retry } = useData(api.profiles, []);
  const [name, setName] = useState("");
  const [pin, setPin] = useState("");
  const [msg, setMsg] = useState("");
  const [busy, setBusy] = useState(false);
  const pinRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (name) pinRef.current?.focus();
  }, [name]);

  // Only one member yet: skip the picker.
  useEffect(() => {
    if (data && !data.setup && data.profiles.length === 1) setName(data.profiles[0].name);
  }, [data]);

  if (!data) return <main className="login">{error ? <Failed message={error} retry={retry} /> : <Loading />}</main>;
  const setup = data.setup;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setMsg("");
    try {
      onDone(setup ? await api.setup(name, pin) : await api.login(name, pin));
    } catch (err) {
      setMsg((err as Error).message);
      setPin("");
      pinRef.current?.focus();
    } finally {
      setBusy(false);
    }
  };

  return (
    <main className="login">
      <h1 className="wordmark">Tonton</h1>
      {setup ? (
        <p className="lede">Buat akun pertama. Akun ini jadi admin dan bisa menambah anggota keluarga.</p>
      ) : (
        <p className="lede">Siapa yang nonton?</p>
      )}

      {!setup && (
        <ul className="profiles">
          {data.profiles.map((p) => (
            <li key={p.name}>
              <button type="button" aria-pressed={name === p.name} onClick={() => { setName(p.name); setMsg(""); }}>
                {p.name}
              </button>
            </li>
          ))}
        </ul>
      )}

      {(setup || name) && (
        <form className="pin-form" onSubmit={submit}>
          {setup && (
            <label className="field">
              <span className="label">Nama</span>
              <input value={name} onChange={(e) => setName(e.target.value)} maxLength={24} required autoFocus autoComplete="username" />
            </label>
          )}
          <label className="field">
            <span className="label">{setup ? "Buat PIN (4–32 karakter)" : `PIN ${name}`}</span>
            <input
              ref={pinRef}
              type="password"
              inputMode="numeric"
              autoComplete={setup ? "new-password" : "current-password"}
              value={pin}
              onChange={(e) => setPin(e.target.value)}
              minLength={4}
              maxLength={32}
              required
            />
          </label>
          {msg && <p className="error" role="alert">{msg}</p>}
          <button className="btn solid" type="submit" disabled={busy}>
            {setup ? "Buat akun" : "Masuk"}
          </button>
        </form>
      )}
    </main>
  );
}
