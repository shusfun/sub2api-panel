import type { CSSProperties } from "react";

const sheets = {
  idle: [
    "M 10 8 L 18 8 Q 20 8 20 10 L 20 18 Q 20 20 18 20 L 10 20 Q 8 20 8 18 L 8 10 Q 8 8 10 8",
    "M 6 16 L 5 16 Q 4 16 4 14 L 4 6 Q 4 4 6 4 L 14 4 Q 16 4 16 6",
  ],
  copied: [
    "M 5 12 L 10 17 Q 10 17 10 17 L 20 7 Q 20 7 20 7 L 20 7 Q 20 7 20 7 L 20 7 Q 20 7 20 7",
    "M 10 17 L 10 17 Q 10 17 10 17 L 10 17 Q 10 17 10 17 L 10 17 Q 10 17 10 17",
  ],
};

export function CopyIcon({ copied }: { copied: boolean }) {
  const [front, back] = copied ? sheets.copied : sheets.idle;
  return <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"
    strokeLinejoin="round" aria-hidden className="weir-copy-icon">
    <path d={front} style={{ d: `path('${front}')` } as CSSProperties} />
    <path d={back} style={{ d: `path('${back}')` } as CSSProperties} />
  </svg>;
}
