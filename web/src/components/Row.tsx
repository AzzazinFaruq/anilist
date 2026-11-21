import type { CSSProperties, ReactNode } from "react";
import { displayTitle } from "../format";
import { openTitle } from "../router";
import type { Item } from "../types";
import { Poster } from "./Poster";

/** One title as a line: poster, name, one line of context, optional controls on the right. */
export function Row({ item, sub, aside, below }: { item: Item; sub?: ReactNode; aside?: ReactNode; below?: ReactNode }) {
  const a = item.anime;
  return (
    <li className="row" style={a.color ? ({ "--c": a.color } as CSSProperties) : undefined}>
      <button type="button" className="row-main" onClick={() => openTitle(a.id)}>
        <Poster src={a.image} />
        <span className="row-text">
          <span className="row-title">{displayTitle(a)}</span>
          {sub && <span className="row-sub">{sub}</span>}
          {below}
        </span>
      </button>
      {aside && <div className="row-aside">{aside}</div>}
    </li>
  );
}
