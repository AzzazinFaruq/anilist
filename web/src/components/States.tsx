export function Loading() {
  return <p className="quiet loading">Memuat…</p>;
}

export function Failed({ message, retry }: { message: string; retry: () => void }) {
  return (
    <div className="failed">
      <p>{message}</p>
      <button type="button" className="btn" onClick={retry}>
        Coba lagi
      </button>
    </div>
  );
}
