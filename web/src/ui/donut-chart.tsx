import {useLayoutEffect, useRef, useState} from "react";
import {Popover} from "@base-ui/react/popover";
import {Button} from "./button";
import {Chart} from "./chart";
import {Tooltip} from "./tooltip";

type Slice = {label: string; value: number; color: string; onSelect?: () => void};
type Leader = {
  label: string;
  color: string;
  x: number;
  y: number;
  elbow: number;
  end: number;
  targetY: number;
};

const wideHeight = 215;
const wideRadius = 81;
const narrowHeight = 151;
const narrowRadius = 61;

function layoutLabels(slices: Slice[], total: number, width: number) {
  const centerX = width / 2;
  const centerY = wideHeight / 2;
  const columnWidth = Math.min(315, Math.round(width * 0.28));
  let angle = -Math.PI / 2;
  const anchors = slices.map((slice, index) => {
    const sweep = (slice.value / total) * 2 * Math.PI;
    const mid = angle + sweep / 2;
    angle += sweep;
    const dx = Math.cos(mid);
    const dy = Math.sin(mid);
    return {
      index,
      right: dx >= 0,
      start: [centerX + (wideRadius + 1) * dx, centerY + (wideRadius + 1) * dy],
      target: centerY + (wideRadius + 13) * dy,
      y: 0,
    };
  });

  const lines: Leader[] = [];
  for (const right of [false, true]) {
    const rows = anchors.filter(anchor => anchor.right === right).sort((a, b) => a.target - b.target);
    const end = right ? width - columnWidth - 5 : columnWidth + 5;
    const elbow = end + (right ? -13 : 13);
    const gap = Math.min(27, (wideHeight - 38) / Math.max(1, rows.length - 1));
    let minY = 19;
    rows.forEach((row, index) => {
      const maxY = wideHeight - 19 - (rows.length - index - 1) * gap;
      row.y = Math.max(minY, Math.min(row.target, maxY));
      minY = row.y + gap;
      const label = slices[row.index].label;
      const color = slices[row.index].color;
      const [x, y] = row.start;
      lines.push({label, color, x, y, elbow, end, targetY: row.y});
    });
  }
  return {columnWidth, anchors, lines};
}

export function DonutChart({ariaLabel, slices, formatShare}: {
  ariaLabel: string;
  slices: Slice[];
  formatShare: (share: number) => string;
}) {
  const container = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(0);
  useLayoutEffect(() => {
    const node = container.current;
    if (!node) return;
    const measure = () => setWidth(node.clientWidth);
    const observer = new ResizeObserver(measure);
    measure();
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  const visible = slices.filter(item => item.value > 0);
  if (visible.length === 0) return null;
  const total = visible.reduce((sum, item) => sum + item.value, 0);
  const layout = width >= 455 ? layoutLabels(visible, total, width) : null;
  const height = layout ? wideHeight : narrowHeight;
  const radius = layout ? wideRadius : narrowRadius;

  return (
    <div ref={container} className="obs-donut" data-leaders={layout ? "true" : undefined}>
      <Chart ariaLabel={ariaLabel} height={height} option={{
        animation: false,
        tooltip: {renderMode: "richText", trigger: "item", formatter: "{b} · {d}%"},
        series: [{
          type: "pie", startAngle: 90, radius: [Math.round(radius * 0.66), radius],
          label: {show: false}, emphasis: {scale: false},
          data: visible.map(item => ({name: item.label, value: item.value, itemStyle: {color: item.color}})),
        }],
      }} onPointClick={({dataIndex}) => {
        if (dataIndex !== undefined) visible[dataIndex]?.onSelect?.();
      }} />
      {layout && <div className="obs-donut-leaders" aria-hidden>
        {layout.lines.map(({label, color, x, y, elbow, end, targetY}) => (
          <div key={label}>
            <span style={{left: x, top: y, width: Math.hypot(elbow - x, targetY - y),
              rotate: Math.atan2(targetY - y, elbow - x) + "rad", background: color}} />
            <span style={{left: Math.min(elbow, end), top: targetY, width: Math.abs(end - elbow), background: color}} />
          </div>
        ))}
      </div>}
      <ul className="obs-donut-labels" aria-label={ariaLabel}>
        {visible.map((item, index) => {
          const anchor = layout?.anchors[index];
          const share = formatShare(item.value / total);
          return (
            <li key={item.label} data-side={anchor && (anchor.right ? "right" : "left")}
              style={anchor && {top: anchor.y, width: layout?.columnWidth}}>
              <Popover.Root>
                <Tooltip side="top" content={<span className="obs-donut-full-name">{item.label}</span>}>
                  <Popover.Trigger className="obs-donut-label">
                    <span className="obs-donut-swatch" style={{backgroundColor: item.color}} aria-hidden />
                    <span className="obs-donut-name">{item.label}</span>
                    <span className="obs-donut-share">{share}</span>
                  </Popover.Trigger>
                </Tooltip>
                <Popover.Portal>
                  <Popover.Positioner side="top" sideOffset={5} className="obs-popover-positioner">
                    <Popover.Popup className="obs-donut-popup">
                      <Popover.Title>{item.label}</Popover.Title>
                      {item.onSelect && (
                        <Popover.Close render={<Button size="xs" variant="outline" onClick={item.onSelect} />}>
                          查看逐笔用量
                        </Popover.Close>
                      )}
                    </Popover.Popup>
                  </Popover.Positioner>
                </Popover.Portal>
              </Popover.Root>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
