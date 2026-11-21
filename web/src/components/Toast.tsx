import { useToast } from "../data";

export function Toast() {
  const t = useToast();
  return (
    <div className="toast" role="status" aria-live="polite">
      {t && <span key={t.id}>{t.text}</span>}
    </div>
  );
}
