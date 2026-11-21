import type { CSSProperties, ReactNode } from "react";
import { SEASON_KEYS, SEASONS, STATUS } from "../format";
import type { DayCount, Status } from "../types";

/** Coloured dot + word for a list status. */
export function StatusTag({ status }: { status: Status }) {
  return <span className={`status-tag s-${status}`}>{STATUS[status]}</span>;
}

/** "Layak tonton" tier badge, S (must try) to D (skip for now). */
export function Tier({ tier }: { tier?: string }) {
  if (!tier) return null;
  return (
    <span className={`tier t-${tier}`} title={`Layak tonton: tier ${tier}`}>
      {tier}
    </span>
  );
}

export function SeasonName({ year, season }: { year: number; season?: number | null }) {
  if (season === undefined || season === null) return <>{year}</>;
  return (
    <span className={`season-name ${SEASON_KEYS[season]}`}>
      {SEASONS[season]} {year}
    </span>
  );
}

/** Thin progress line in the title's own cover colour. */
export function Progress({ value, total, color }: { value: number; total: number; color?: string }) {
  if (!total) return null;
  const style = { "--w": `${Math.min(100, (value / total) * 100)}%`, "--c": color || "var(--ink)" } as CSSProperties;
  return <span className="progress" style={style} aria-hidden="true" />;
}

/** Toggle chip. `state` lets a chip cycle include → exclude → off. */
export function Chip({
  on,
  off,
  onClick,
  children,
  title,
}: {
  on?: boolean;
  off?: boolean;
  onClick: () => void;
  children: ReactNode;
  title?: string;
}) {
  return (
    <button type="button" className={`chip${on ? " on" : ""}${off ? " off" : ""}`} aria-pressed={on || off ? true : false} onClick={onClick} title={title}>
      {off && <span className="sr-only">kecualikan </span>}
      {children}
    </button>
  );
}

export function Seg<T extends string>({
  value,
  options,
  onChange,
  label,
  small,
}: {
  value: T;
  options: [T, ReactNode][];
  onChange: (v: T) => void;
  label: string;
  small?: boolean;
}) {
  return (
    <div className={`seg${small ? " small" : ""}`} role="group" aria-label={label}>
      {options.map(([v, l]) => (
        <button key={v} type="button" aria-pressed={value === v} onClick={() => onChange(v)}>
          {l}
        </button>
      ))}
    </div>
  );
}

/** Twelve weeks of episodes per day, GitHub style, Monday at the top. */
export function Heatmap({ days }: { days: DayCount[] }) {
  if (!days.length) return null;
  const first = new Date(days[0].day + "T00:00:00");
  const pad = (first.getDay() + 6) % 7;
  const cells: (DayCount | null)[] = [...Array(pad).fill(null), ...days];
  const level = (n: number) => (n === 0 ? 0 : n <= 1 ? 1 : n <= 3 ? 2 : n <= 6 ? 3 : 4);
  const total = days.reduce((s, d) => s + d.n, 0);
  return (
    <div className="heat-wrap">
      <div className="heat" role="img" aria-label={`${total} episode dalam ${days.length} hari terakhir`}>
        {cells.map((d, i) => (d ? <i key={i} className={`h${level(d.n)}`} title={`${d.day}: ${d.n} episode`} /> : <i key={i} className="blank" />))}
      </div>
      <div className="heat-legend" aria-hidden="true">
        <span>sedikit</span>
        <i className="h1" />
        <i className="h2" />
        <i className="h3" />
        <i className="h4" />
        <span>banyak</span>
      </div>
    </div>
  );
}

/** Horizontal bars with the value at the end. */
export function Bars({ rows, colorful }: { rows: { name: string; n: number }[]; colorful?: boolean }) {
  const maxN = Math.max(1, ...rows.map((r) => r.n));
  return (
    <ul className="bars">
      {rows.map((r, i) => (
        <li key={r.name}>
          <span className="bar-name">{r.name}</span>
          <span className="bar-track">
            <span className={`bar-fill${colorful ? ` c${i % 6}` : ""}`} style={{ "--w": `${(r.n / maxN) * 100}%` } as CSSProperties} />
          </span>
          <span className="bar-n">{r.n}</span>
        </li>
      ))}
    </ul>
  );
}

/** Vertical bars, one per label (e.g. titles per release year). */
export function Columns({ rows }: { rows: { name: string; n: number }[] }) {
  const maxN = Math.max(1, ...rows.map((r) => r.n));
  return (
    <div className="columns" role="img" aria-label={rows.map((r) => `${r.name}: ${r.n}`).join(", ")}>
      {rows.map((r) => (
        <span key={r.name} className="col" title={`${r.name}: ${r.n}`}>
          <span className="col-fill" style={{ "--h": `${(r.n / maxN) * 100}%` } as CSSProperties} />
          <span className="col-label">{r.name.slice(-2)}</span>
        </span>
      ))}
    </div>
  );
}

export function Stat({ label, value, note }: { label: string; value: ReactNode; note?: ReactNode }) {
  return (
    <div className="stat">
      <span className="stat-label">{label}</span>
      <b className="stat-value">{value}</b>
      {note && <span className="stat-note">{note}</span>}
    </div>
  );
}
