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

// 启用/暂停单路径连续形变：同一两条折线在「双竖暂停柱」与「三角启动箭头」间插值，语义清晰不硬切。
export function EnableMorphIcon({enabled}: {enabled: boolean}) {
  const [first, second] = enabled ? playPausePaths.active : playPausePaths.paused;
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden data-enabled={enabled || undefined} className="weir-morph-icon weir-enable-icon">
      <path d={first} style={{d: `path('${first}')`} as CSSProperties} />
      <path d={second} style={{d: `path('${second}')`} as CSSProperties} />
    </svg>
  );
}

// 排序箭头连续翻转：三条等拓扑路径在「未排序双尖」「升序上箭」「降序下箭」间平滑形变。
const sortPaths = {
  none: [
    "M 8 9 L 12 5 L 16 9",
    "M 12 12 L 12 12",
    "M 8 15 L 12 19 L 16 15",
  ],
  asc: [
    "M 7 11 L 12 6 L 17 11",
    "M 12 6 L 12 18",
    "M 12 18 L 12 18 L 12 18",
  ],
  desc: [
    "M 12 6 L 12 6 L 12 6",
    "M 12 6 L 12 18",
    "M 7 13 L 12 18 L 17 13",
  ],
};

export function SortDirectionIcon({direction = "none"}: {direction?: "none" | "asc" | "desc"}) {
  const [top, shaft, bottom] = sortPaths[direction];
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden data-dir={direction} className="weir-morph-icon weir-sort-icon">
      <path d={top} style={{d: `path('${top}')`} as CSSProperties} />
      <path d={shaft} style={{d: `path('${shaft}')`} as CSSProperties} />
      <path d={bottom} style={{d: `path('${bottom}')`} as CSSProperties} />
    </svg>
  );
}

// 加号/叉号连续旋转形变：正交十字沿中心平滑旋至 45°。
export function PlusCloseMorphIcon({open}: {open: boolean}) {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden data-open={open || undefined} className="weir-plus-close-icon">
      <path d="M 12 5 L 12 19" />
      <path d="M 5 12 L 19 12" />
    </svg>
  );
}

// 筛选条连续形变：三条水平筛选线在「默认漏斗」与「已激活聚焦态」间伸缩移动。
const filterPaths = {
  idle: [
    "M 4 7 L 20 7",
    "M 7 12 L 17 12",
    "M 10 17 L 14 17",
  ],
  active: [
    "M 4 7 L 16 7",
    "M 4 12 L 20 12",
    "M 8 17 L 18 17",
  ],
};

export function FilterMorphIcon({active = false}: {active?: boolean}) {
  const [a, b, c] = active ? filterPaths.active : filterPaths.idle;
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden data-active={active || undefined} className="weir-morph-icon weir-filter-icon">
      <path d={a} style={{d: `path('${a}')`} as CSSProperties} />
      <path d={b} style={{d: `path('${b}')`} as CSSProperties} />
      <path d={c} style={{d: `path('${c}')`} as CSSProperties} />
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
