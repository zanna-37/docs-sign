import type { CSSProperties } from "react";

// checkerBackground returns a CSS checkerboard pattern (a transparency indicator) with the
// given tile size in pixels. Used behind signatures and PDF pages where transparency shows.
export function checkerBackground(tile = 16): CSSProperties {
  const half = tile / 2;
  return {
    backgroundColor: "#fff",
    backgroundImage:
      "linear-gradient(45deg,#eee 25%,transparent 25%),linear-gradient(-45deg,#eee 25%,transparent 25%),linear-gradient(45deg,transparent 75%,#eee 75%),linear-gradient(-45deg,transparent 75%,#eee 75%)",
    backgroundSize: `${tile}px ${tile}px`,
    backgroundPosition: `0 0,0 ${half}px,${half}px -${half}px,-${half}px 0`,
  };
}
