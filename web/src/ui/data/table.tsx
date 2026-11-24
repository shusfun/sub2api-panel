// 来源 codedthemes/uiable table；macOS 列表风格的平面表格：表头用标注字号的次要文字，行之间只有细分隔线，
// 行高 37。可点的行（带 onClick）按下叠填充色。
import { ComponentProps } from "react";
import { cn } from "cn";
import { ScrollArea } from "../primitives/scroll-area";

function Table({ className, ...props }: ComponentProps<"table">) {
  return (
    <ScrollArea data-slot="table-container" orientation="horizontal" className="w-full" contentRender={false}>
      <table data-slot="table" className={cn("w-full caption-bottom border-separate border-spacing-0 text-body", className)} {...props} />
    </ScrollArea>
  );
}

function TableHeader({ className, ...props }: ComponentProps<"thead">) {
  return <thead data-slot="table-header" className={cn("text-left", className)} {...props} />;
}

function TableBody({ className, ...props }: ComponentProps<"tbody">) {
  return <tbody data-slot="table-body" className={className} {...props} />;
}

function TableFooter({ className, ...props }: ComponentProps<"tfoot">) {
  return (
    <tfoot data-slot="table-footer" className={cn("border-t border-[var(--weir-line-soft)] bg-[var(--weir-fill-4)] font-medium", className)} {...props} />
  );
}

function TableRow({ className, ...props }: ComponentProps<"tr">) {
  const pressable = !!props.onClick;
  return (
    <tr
      data-slot="table-row"
      className={cn(
        pressable && "cursor-pointer",
        className,
      )}
      {...props}
    />
  );
}

function TableHead({ className, ...props }: ComponentProps<"th">) {
  return (
    <th
      data-slot="table-head"
      className={cn("h-[var(--weir-tap)] px-[var(--weir-table-cell-padding)] align-middle text-fine font-normal text-[var(--weir-ink-3)] whitespace-nowrap [&:has([role=checkbox])]:pr-0", className)}
      {...props}
    />
  );
}

function TableCell({ className, ...props }: ComponentProps<"td">) {
  return (
    <td
      data-slot="table-cell"
      className={cn("h-[var(--weir-row)] px-[var(--weir-table-cell-padding)] py-[var(--weir-gap-5)] align-middle whitespace-nowrap [&:has([role=checkbox])]:pr-0", className)}
      {...props}
    />
  );
}

function TableCaption({ className, ...props }: ComponentProps<"caption">) {
  return <caption data-slot="table-caption" className={cn("mt-4 text-meta text-[var(--weir-ink-3)]", className)} {...props} />;
}

export { Table, TableHeader, TableBody, TableFooter, TableHead, TableRow, TableCell, TableCaption };
