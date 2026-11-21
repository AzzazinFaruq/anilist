import { useState } from "react";
import { poster } from "../format";

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
