import {useEffect, useLayoutEffect, useRef, useState, type ReactNode} from "react";

function motionElements(container: HTMLElement) {
  return container.querySelectorAll<HTMLElement>("[data-motion-key]");
}

export function ReorderMotion({children, order}: {children: ReactNode; order: string}) {
  const root = useRef<HTMLDivElement>(null);
  const previous = useRef(new Map<string, {x: number; y: number}>());
  const animations = useRef(new Map<HTMLElement, Animation>());
  useLayoutEffect(() => {
    const container = root.current;
    if (!container) return;
    const bounds = container.getBoundingClientRect();
    const reduced = matchMedia("(prefers-reduced-motion: reduce)");
    const style = getComputedStyle(container);
    const time = style.getPropertyValue("--weir-mid").trim();
    const duration = parseFloat(time) * (time.endsWith("ms") ? 1 : 1000);
    const easing = style.getPropertyValue("--weir-ease").trim();
    const positions = new Map<string, {x: number; y: number}>();
    for (const animation of animations.current.values()) animation.cancel();
    animations.current.clear();
    for (const element of motionElements(container)) {
      const key = element.dataset.motionKey!;
      const rect = element.getBoundingClientRect();
      const position = {x: rect.left - bounds.left, y: rect.top - bounds.top};
      positions.set(key, position);
      const before = previous.current.get(key);
      if (!before || !(duration > 0) || !easing || reduced.matches || document.hidden || rect.bottom < 0 || rect.top > innerHeight) continue;
      const x = before.x - position.x;
      const y = before.y - position.y;
      if (x === 0 && y === 0) continue;
      const animation = element.animate([{translate: `${x}px ${y}px`}, {translate: "0px 0px"}], {duration, easing});
      animations.current.set(element, animation);
      animation.onfinish = () => { if (animations.current.get(element) === animation) animations.current.delete(element); };
    }
    previous.current = positions;
  }, [order]);
  useEffect(() => {
    const container = root.current;
    if (!container) return;
    const running = animations.current;
    const cancel = () => {
      for (const animation of running.values()) animation.cancel();
      running.clear();
    };
    const resize = new ResizeObserver(() => {
      if (running.size) return;
      const bounds = container.getBoundingClientRect();
      for (const element of motionElements(container)) {
        const rect = element.getBoundingClientRect();
        previous.current.set(element.dataset.motionKey!, {x: rect.left - bounds.left, y: rect.top - bounds.top});
      }
    });
    const reduced = matchMedia("(prefers-reduced-motion: reduce)");
    const onVisibility = () => { if (document.hidden) cancel(); };
    resize.observe(container);
    reduced.addEventListener("change", cancel);
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      cancel();
      resize.disconnect();
      reduced.removeEventListener("change", cancel);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, []);
  return <div className="weir-reorder" ref={root}>{children}</div>;
}

export function ActivityWire() {
  const root = useRef<HTMLSpanElement>(null);
  const [visible, setVisible] = useState(false);
  useEffect(() => {
    if (!root.current) return;
    const observer = new IntersectionObserver(([entry]) => setVisible(entry.isIntersecting));
    observer.observe(root.current);
    return () => observer.disconnect();
  }, []);
  const [hidden, setHidden] = useState(document.hidden);
  useEffect(() => {
    const sync = () => setHidden(document.hidden);
    document.addEventListener("visibilitychange", sync);
    return () => document.removeEventListener("visibilitychange", sync);
  }, []);
  return <span ref={root} className="weir-activity-wire" data-flow={visible && !hidden || undefined} role="img" aria-label="正在处理请求">
    <svg viewBox="0 0 35 13" aria-hidden="true">
      <path d="M1 9h5c4 0 4-5 8-5s4 5 8 5h12" pathLength="100" />
      <path className="weir-activity-pulse" d="M1 9h5c4 0 4-5 8-5s4 5 8 5h12" pathLength="100" />
    </svg>
  </span>;
}
