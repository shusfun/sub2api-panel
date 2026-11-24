import type {CSSProperties} from "react";

const chevronPaths = {
  closed: "M 9 6 L 15 12 L 9 18",
  open: "M 6 9 L 12 15 L 18 9",
};

export function ChevronMorphIcon({open}: {open: boolean}) {
  const shape = open ? chevronPaths.open : chevronPaths.closed;
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden className="weir-morph-icon">
      <path d={shape} style={{d: `path('${shape}')`} as CSSProperties} />
    </svg>
  );
}

const selectChevronPaths = {
  closed: "M 6 9 L 12 15 L 18 9",
  open: "M 6 15 L 12 9 L 18 15",
};

export function SelectChevronIcon({open}: {open: boolean}) {
  const shape = open ? selectChevronPaths.open : selectChevronPaths.closed;
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden className="weir-morph-icon">
      <path d={shape} style={{d: `path('${shape}')`} as CSSProperties} />
    </svg>
  );
}

const playPausePaths = {
  active: [
    "M 8 6 L 8 18 L 8 18",
    "M 16 6 L 16 18 L 16 18",
  ],
  paused: [
    "M 8 6 L 8 18 L 18 12",
    "M 8 6 L 18 12 L 18 12",
  ],
};

export function PlayPauseIcon({active}: {active: boolean}) {
  const [first, second] = active ? playPausePaths.active : playPausePaths.paused;
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden className="weir-morph-icon">
      <path d={first} style={{d: `path('${first}')`} as CSSProperties} />
      <path d={second} style={{d: `path('${second}')`} as CSSProperties} />
    </svg>
  );
}

// 眼睛显隐连续形变：同一三次贝塞尔拓扑下上眼睑压平闭拢，斜杠从中心点伸展为遮挡线，禁止双图标切换。
const eyeUpperLid = {
  open: "M 2 12 C 5.5 6 18.5 6 22 12",
  closed: "M 2 12 C 6.5 14.5 17.5 14.5 22 12",
};
const eyeLowerLid = {
  open: "M 2 12 C 5.5 18 18.5 18 22 12",
  closed: "M 2 12 C 6.5 16.5 17.5 16.5 22 12",
};
const eyeSlash = {
  open: "M 12 12 L 12 12",
  closed: "M 4 4 L 20 20",
};

export function EyeMorphIcon({open}: {open: boolean}) {
  const upper = open ? eyeUpperLid.open : eyeUpperLid.closed;
  const lower = open ? eyeLowerLid.open : eyeLowerLid.closed;
  const slash = open ? eyeSlash.open : eyeSlash.closed;
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden
      data-closed={!open || undefined}
      className="weir-eye-icon"
    >
      <path d={upper} style={{d: `path('${upper}')`} as CSSProperties} />
      <path d={lower} style={{d: `path('${lower}')`} as CSSProperties} />
      <circle cx="12" cy="12" r="2.7" />
      <path d={slash} style={{d: `path('${slash}')`} as CSSProperties} />
    </svg>
  );
}

export function RefreshMorphIcon() {
  return (
    <svg
      width="15"
      height="15"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden
      className="weir-refresh-icon"
    >
      <path d="M21 12a9 9 0 1 1-2.64-6.36" />
      <path d="M21 3v6h-6" />
    </svg>
  );
}
