import { change } from "../data";
import { Icon } from "./Icon";
import type { Anime, Entry } from "../types";

/** The episode counter: the one big thing on the screen. */
export function Stepper({ anime, entry, size = "big" }: { anime: Anime; entry?: Entry; size?: "big" | "small" }) {
  const ep = entry?.episode ?? 0;
  const total = anime.status === "RELEASING" || anime.status === "NOT_YET_RELEASED" ? 0 : anime.episodes ?? 0;
  const atEnd = total > 0 && ep >= total;
  const set = (n: number) => change(anime.id, { episode: n }, true);
  return (
    <div className={`stepper ${size}`}>
      <span className="count" aria-live="polite">
        <b>{String(ep).padStart(2, "0")}</b>
        <small>{total ? `/${total}` : anime.episodes ? `/${anime.episodes}+` : ""}</small>
      </span>
      <span className="steps">
        <button type="button" className="step" onClick={() => set(ep - 1)} disabled={ep <= 0} aria-label={`Kurangi episode ${anime.title}`}>
          <Icon name="minus" />
        </button>
        <button type="button" className="step plus" onClick={() => set(ep + 1)} disabled={atEnd} aria-label={`Tambah episode ${anime.title}`}>
          <Icon name="plus" />
        </button>
      </span>
    </div>
  );
}
