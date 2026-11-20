import {ChevronLeft, ChevronRight} from "lucide-react";
import {Button} from "./button";

export function Pagination({number, more, onChange, label = "翻页"}: {number: number; more: boolean; onChange: (page: number) => void; label?: string}) {
  if (number === 1 && !more) return null;
  return <nav className="pagination" aria-label={label}>
    <Button variant="ghost" size="icon-sm" aria-label="上一页" title="上一页" disabled={number === 1} onClick={() => onChange(number - 1)}><ChevronLeft /></Button>
    <span>第 {number} 页</span>
    <Button variant="ghost" size="icon-sm" aria-label="下一页" title="下一页" disabled={!more} onClick={() => onChange(number + 1)}><ChevronRight /></Button>
  </nav>;
}
