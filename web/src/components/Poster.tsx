import { useState } from "react";
import { poster } from "../format";

/** Grey until hovered, focused or opened: colour is reserved for what you are looking at. */
export function Poster({ src, size = "medium", className = "" }: { src: string; size?: "medium" | "large"; className?: string }) {
  const [failed, setFailed] = useState(false);
  return (
    <span className={`poster ${className}`} aria-hidden="true">
      {!failed && src && (
        <img src={poster(src, size)} alt="" loading="lazy" decoding="async" referrerPolicy="no-referrer" onError={() => setFailed(true)} />
      )}
    </span>
  );
}
