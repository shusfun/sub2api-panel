import {ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight} from "lucide-react";
import {Button} from "./button";

function pageWindow(current: number, total: number): (number | "…")[] {
  if (total <= 7) return Array.from({length: total}, (_, i) => i + 1);
  const pages: (number | "…")[] = [1];
  const start = Math.max(2, current - 1);
  const end = Math.min(total - 1, current + 1);
  if (start > 2) pages.push("…");
  for (let p = start; p <= end; p++) pages.push(p);
  if (end < total - 1) pages.push("…");
  pages.push(total);
  return pages;
}

export function Pagination({
  number,
  more,
  pages,
  onChange,
  label = "翻页",
}: {
  number: number;
  more: boolean;
  pages?: number;
  onChange: (page: number) => void;
  label?: string;
}) {
  const total = pages && pages > 0 ? Math.max(pages, number) : undefined;
  if (number === 1 && !more && (!total || total <= 1)) return null;
  const items = total ? pageWindow(number, total) : null;
  return (
    <nav className="pagination obs-pagination" aria-label={label}>
      <span className="obs-pagination-meta weir-digits">
        {total ? `第 ${number} / ${total} 页` : `第 ${number} 页`}
      </span>
      <div className="obs-pagination-controls">
        {total && total > 2 && (
          <Button variant="ghost" size="icon-sm" aria-label="首页" title="首页" disabled={number === 1} onClick={() => onChange(1)}>
            <ChevronsLeft />
          </Button>
        )}
        <Button variant="ghost" size="icon-sm" aria-label="上一页" title="上一页" disabled={number === 1} onClick={() => onChange(number - 1)}>
          <ChevronLeft />
        </Button>
        {items?.map((item, idx) => item === "…"
          ? <span key={`gap-${idx}`} className="obs-pagination-gap">…</span>
          : <Button
              key={item}
              variant={item === number ? "secondary" : "ghost"}
              size="sm"
              aria-current={item === number ? "page" : undefined}
              onClick={() => onChange(item)}
              className="obs-pagination-num weir-digits"
            >
              {item}
            </Button>,
        )}
        <Button variant="ghost" size="icon-sm" aria-label="下一页" title="下一页" disabled={!more} onClick={() => onChange(number + 1)}>
          <ChevronRight />
        </Button>
        {total && total > 2 && (
          <Button variant="ghost" size="icon-sm" aria-label="末页" title="末页" disabled={number >= total} onClick={() => onChange(total)}>
            <ChevronsRight />
          </Button>
        )}
      </div>
    </nav>
  );
}
