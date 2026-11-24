import type {ReactNode} from "react";
import {Checkbox} from "../primitives/checkbox";
import {SortDirectionIcon} from "../icons/icons";
import {EmptyState, Resource, TableSkeleton} from "../patterns/feedback";
import {Pagination} from "./pagination";
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow} from "./table";

export type TableSort = {
  id: string;
  dir: "asc" | "desc";
};

export type Column<Row> = {
  id: string;
  title: string;
  align?: "number" | "end" | "center";
  grow?: boolean;
  sortable?: boolean;
  hideWhenNarrow?: boolean;
  hideWhenCompact?: boolean;
  render: (row: Row) => ReactNode;
};

export type TableSelection<ID extends string | number = number> = {
  selected: readonly ID[];
  onChange: (next: ID[]) => void;
};

export function DataTable<Row, ID extends string | number = string | number>({
  columns,
  rows,
  rowID,
  label,
  onRowClick,
  activeRowID,
  selection,
  sort,
  onSort,
  stale,
  empty,
  motion = false,
  pinActions = false,
}: {
  columns: readonly Column<Row>[];
  rows: readonly Row[];
  rowID: (row: Row) => ID;
  label: string;
  onRowClick?: (row: Row) => void;
  activeRowID?: string | number;
  selection?: TableSelection<ID>;
  sort?: TableSort;
  onSort?: (next: TableSort | undefined) => void;
  stale?: boolean;
  empty?: ReactNode;
  motion?: boolean;
  pinActions?: boolean;
}) {
  if (!rows.length) return <>{empty ?? <EmptyState title="暂无记录" />}</>;

  const visibleIDs = rows.map(row => rowID(row));
  const allVisibleSelected = selection
    ? visibleIDs.length > 0 && visibleIDs.every(id => selection.selected.includes(id))
    : false;

  const toggleAll = (checked: boolean) => {
    if (!selection) return;
    if (checked) {
      selection.onChange([...new Set([...selection.selected, ...visibleIDs])]);
    } else {
      selection.onChange(selection.selected.filter(id => !visibleIDs.includes(id)));
    }
  };

  const toggleOne = (id: ID, checked: boolean) => {
    if (!selection) return;
    selection.onChange(
      checked
        ? [...new Set([...selection.selected, id])]
        : selection.selected.filter(item => item !== id),
    );
  };

  const cycleSort = (id: string) => {
    if (!onSort) return;
    if (!sort || sort.id !== id) onSort({id, dir: "desc"});
    else if (sort.dir === "desc") onSort({id, dir: "asc"});
    else onSort(undefined);
  };

  return (
    <div className="obs-table-wrap" data-stale={stale || undefined}>
      {stale && <div className="obs-loading-bar" aria-hidden />}
      <Table aria-label={label} data-selection={!!selection || undefined} data-pin-actions={pinActions || undefined} data-compact={columns.length <= 7 || undefined}>
        <TableHeader>
          <TableRow>
            {selection && (
              <TableHead scope="col" className="swift-select-col">
                <Checkbox checked={allVisibleSelected} onChange={toggleAll} label={`全选当前页 ${label}`} />
              </TableHead>
            )}
            {columns.map((column, index) => {
              const dir = sort?.id === column.id ? sort.dir : "none";
              const ariaSort = column.sortable && onSort ? (dir === "asc" ? "ascending" : dir === "desc" ? "descending" : "none") : undefined;
              return (
                <TableHead
                  key={column.id}
                  scope="col"
                  aria-sort={ariaSort}
                  data-align={column.align}
                  data-grow={column.grow || undefined}
                  data-pinned={index === 0 ? "identity" : pinActions && index === columns.length - 1 ? "actions" : undefined}
                  data-hide-narrow={column.hideWhenNarrow || undefined}
                  data-hide-compact={column.hideWhenCompact || undefined}
                >
                  {column.sortable && onSort ? (
                    <button
                      type="button"
                      className="weir-table-sort-btn"
                      data-align={column.align}
                      onClick={() => cycleSort(column.id)}
                    >
                      <span>{column.title}</span>
                      <SortDirectionIcon direction={dir} />
                    </button>
                  ) : (
                    column.title
                  )}
                </TableHead>
              );
            })}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map(row => {
            const id = rowID(row);
            const selected = activeRowID !== undefined && activeRowID === id;
            const checked = selection ? selection.selected.includes(id) : false;
            return (
              <TableRow
                key={id}
                data-motion-key={motion ? id : undefined}
                data-state={selected ? "active" : checked ? "selected" : undefined}
                data-clickable={onRowClick ? "true" : undefined}
                onClick={onRowClick ? () => onRowClick(row) : undefined}
                tabIndex={onRowClick ? 0 : undefined}
                onKeyDown={onRowClick ? event => {
                  if (event.target !== event.currentTarget || (event.key !== "Enter" && event.key !== " ")) return;
                  event.preventDefault();
                  onRowClick(row);
                } : undefined}
              >
                {selection && (
                  <TableCell className="swift-select-col" onClick={e => e.stopPropagation()}>
                    <Checkbox checked={checked} onChange={next => toggleOne(id, next)} label={`选择 ${String(id)}`} />
                  </TableCell>
                )}
                {columns.map((column, index) => (
                  <TableCell key={column.id} data-align={column.align} data-grow={column.grow || undefined} data-pinned={index === 0 ? "identity" : pinActions && index === columns.length - 1 ? "actions" : undefined} data-hide-narrow={column.hideWhenNarrow || undefined} data-hide-compact={column.hideWhenCompact || undefined}>
                    {index === 0 ? <div className="table-identity-content">{column.render(row)}</div> : column.render(row)}
                  </TableCell>
                ))}
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}

export function PagedTable<Row, ID extends string | number = string | number>({
  state,
  columns,
  rowID,
  label,
  number,
  onChange,
  onRowClick,
  activeRowID,
  selection,
  sort,
  onSort,
  empty,
  fallback,
  pinActions,
  renderItems,
}: {
  state?: {value?: {items: Row[]; more: boolean; pages?: number}; error?: string; loading?: boolean; stale?: boolean};
  columns: readonly Column<Row>[];
  rowID: (row: Row) => ID;
  label: string;
  number: number;
  onChange: (page: number) => void;
  onRowClick?: (row: Row) => void;
  activeRowID?: string | number;
  selection?: TableSelection<ID>;
  sort?: TableSort;
  onSort?: (next: TableSort | undefined) => void;
  empty?: ReactNode;
  fallback?: ReactNode;
  pinActions?: boolean;
  renderItems?: (rows: Row[], stale: boolean) => ReactNode;
}) {
  return <>
    <Resource state={state} fallback={fallback ?? <TableSkeleton rows={5} />}>
      {(data, {stale}) => renderItems ? (data.items.length ? renderItems(data.items, stale) : <>{empty ?? <EmptyState title="暂无记录" />}</>) : (
        <DataTable
          rows={data.items}
          columns={columns}
          rowID={rowID}
          label={label}
          onRowClick={onRowClick}
          activeRowID={activeRowID}
          selection={selection}
          sort={sort}
          onSort={onSort}
          stale={stale}
          empty={empty}
          pinActions={pinActions}
        />
      )}
    </Resource>
    <Pagination
      number={number}
      more={state?.value?.more ?? false}
      pages={state?.value?.pages}
      onChange={onChange}
      label={label + "翻页"}
    />
  </>;
}
