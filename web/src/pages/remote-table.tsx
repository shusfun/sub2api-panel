import {useState} from "react";
import type {z} from "zod";
import {api, page} from "../api";
import {useResource} from "../requests";
import {PagedTable, type Column} from "../ui";

export function RemoteTable<Schema extends z.ZodTypeAny>({path, schema, columns, rowID, label, revision = 0}: {
  path: string;
  schema: Schema;
  columns: readonly Column<z.output<Schema>>[];
  rowID: (row: z.output<Schema>) => string | number;
  label: string;
  revision?: number;
}) {
  const [current, setPage] = useState({path, number: 1});
  const number = current.path === path ? current.number : 1;
  const url = path + (path.includes("?") ? "&" : "?") + "page=" + number;
  const data = useResource(() => api(url, page(schema)), url, true, revision);
  return <PagedTable state={data} columns={columns} rowID={rowID} label={label} number={number}
    onChange={next => setPage({path, number: next})} />;
}
