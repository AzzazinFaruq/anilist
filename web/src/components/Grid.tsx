import type { CSSProperties } from "react";
import { displayTitle, facts, score10 } from "../format";
import { openTitle } from "../router";
import type { Item } from "../types";
import { Poster } from "./Poster";
import { StatusTag, Tier } from "./ui";

export function PosterGrid({ items, stale }: { items: Item[]; stale?: boolean }) {
  return (
    <ul className={`grid${stale ? " stale" : ""}`}>
      {items.map((it) => (
        <li key={it.anime.id} style={{ "--c": it.anime.color || "var(--line)" } as CSSProperties}>
          <button type="button" onClick={() => openTitle(it.anime.id)}>
            <span className="grid-poster">
              <Poster src={it.anime.image} />
              <Tier tier={it.anime.tier} />
            </span>
            <span className="grid-title">{displayTitle(it.anime)}</span>
            <span className="grid-sub">
              {it.entry ? <StatusTag status={it.entry.status} /> : [it.anime.score ? `★ ${score10(it.anime.score)}` : "", it.anime.episodes && it.anime.format !== "MOVIE" ? `${it.anime.episodes} ep` : ""].filter(Boolean).join("  ")}
            </span>
          </button>
        </li>
      ))}
    </ul>
  );
}

export function TitleTable({ items, stale }: { items: Item[]; stale?: boolean }) {
  return (
    <ul className={`rows${stale ? " stale" : ""}`}>
      {items.map((it) => (
        <li key={it.anime.id} className="row">
          <button type="button" className="row-main" onClick={() => openTitle(it.anime.id)}>
            <Poster src={it.anime.image} />
            <span className="row-text">
              <span className="row-title">{displayTitle(it.anime)}</span>
              <span className="row-sub">{facts(it.anime)}{it.anime.genres?.length ? `. ${it.anime.genres.slice(0, 3).join(", ")}` : ""}</span>
            </span>
          </button>
          <div className="row-aside table-aside">
            {it.entry && <StatusTag status={it.entry.status} />}
            <Tier tier={it.anime.tier} />
            <span className="score">{score10(it.anime.score)}</span>
          </div>
        </li>
      ))}
    </ul>
  );
}
