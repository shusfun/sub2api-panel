import {useEffect, useState, type ReactNode} from "react";
import type {z} from "zod";
import {api, streamSnapshot, type UsageStreamStatus} from "@/lib/http";
import {useResource} from "@/lib/requests";
import {page} from "@/schemas";
import {PagedTable, type Column, type TableSelection, type TableSort} from "./data-table";

export function RemoteTable<Schema extends z.ZodTypeAny, ID extends string | number = string | number>({
  path,
  schema,
  columns,
  rowID,
  label,
  revision = 0,
  onRowClick,
  activeRowID,
  selection,
  sort,
  onSort,
  onItemsLoaded,
  onTotalLoaded,
  empty,
  fallback,
  pinActions,
  renderItems,
  livePath,
  holdLiveUpdates = false,
  onLiveStatus,
}: {
  path: string;
  schema: Schema;
  columns: readonly Column<z.output<Schema>>[];
  rowID: (row: z.output<Schema>) => ID;
  label: string;
  revision?: number;
  onRowClick?: (row: z.output<Schema>) => void;
  activeRowID?: string | number;
  selection?: TableSelection<ID>;
  sort?: TableSort;
  onSort?: (next: TableSort | undefined) => void;
  onItemsLoaded?: (items: z.output<Schema>[]) => void;
  /** 当前筛选条件下的总条数；上游没给总数时为 undefined。 */
  onTotalLoaded?: (total: number | undefined) => void;
  empty?: ReactNode;
  fallback?: ReactNode;
  pinActions?: boolean;
  livePath?: string;
  holdLiveUpdates?: boolean;
  onLiveStatus?: (status: UsageStreamStatus) => void;
  renderItems?: (rows: z.output<Schema>[], stale: boolean) => ReactNode;
}) {
  const [current, setPage] = useState({path, number: 1});
  const number = current.path === path ? current.number : 1;
  const url = path + (path.includes("?") ? "&" : "?") + "page=" + number;
  const data = useResource(() => api(url, page(schema)), url, true, revision);

  const liveURL = livePath ? livePath + url.slice(url.indexOf("?")) : undefined;
  const [incoming, setIncoming] = useState<{url: string; value: NonNullable<typeof data>["value"]}>();
  const [applied, setApplied] = useState<typeof incoming>();
  useEffect(() => {
    if (!liveURL) return;
    const controller = new AbortController();
    void streamSnapshot(liveURL, page(schema), value => {
      if (!controller.signal.aborted) setIncoming({url: liveURL, value});
    }, status => {
      if (!controller.signal.aborted) onLiveStatus?.(status);
    }, controller.signal);
    return () => controller.abort();
  }, [liveURL, schema, onLiveStatus]);
  useEffect(() => {
    if (!holdLiveUpdates && incoming?.url === liveURL) setApplied(incoming);
  }, [holdLiveUpdates, incoming, liveURL]);
  const live = applied?.url === liveURL ? applied?.value : undefined;
  const state = live ? {value: live, loading: false, stale: false} : data;
  useEffect(() => {
    if (state?.value?.items && !state.stale) onItemsLoaded?.(state.value.items);
  }, [state?.stale, state?.value?.items, onItemsLoaded]);
  const loadedTotal = state?.value && !state.stale ? state.value.total : null;
  useEffect(() => {
    if (loadedTotal !== null) onTotalLoaded?.(loadedTotal);
  }, [loadedTotal, onTotalLoaded]);

  return (
    <PagedTable
      state={state}
      columns={columns}
      rowID={rowID}
      label={label}
      number={number}
      onChange={next => setPage({path, number: next})}
      onRowClick={onRowClick}
      activeRowID={activeRowID}
      selection={selection}
      sort={sort}
      onSort={onSort}
      empty={empty}
      fallback={fallback}
      pinActions={pinActions}
      renderItems={renderItems}
    />
  );
}
