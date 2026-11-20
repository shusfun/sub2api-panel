import type {ReactNode} from "react";
import {Feedback, Resource} from "./feedback";
import {Pagination} from "./pagination";
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow} from "./table";

export type Column<Row> = {
  id: string;
  title: string;
  align?: "number" | "end";
  render: (row: Row) => ReactNode;
};

export function DataTable<Row>({columns, rows, rowID, label}: {
  columns: readonly Column<Row>[];
  rows: readonly Row[];
  rowID: (row: Row) => string | number;
  label: string;
}) {
  if (!rows.length) return <Feedback>暂无记录</Feedback>;
  return <Table aria-label={label}>
    <TableHeader><TableRow>{columns.map(column =>
      <TableHead key={column.id} scope="col" data-align={column.align}>{column.title}</TableHead>,
    )}</TableRow></TableHeader>
    <TableBody>{rows.map(row => <TableRow key={rowID(row)}>{columns.map(column =>
      <TableCell key={column.id} data-align={column.align}>{column.render(row)}</TableCell>,
    )}</TableRow>)}</TableBody>
  </Table>;
}

export function PagedTable<Row>({state, columns, rowID, label, number, onChange}: {
  state?: {value?: {items: Row[]; more: boolean}; error?: string};
  columns: readonly Column<Row>[];
  rowID: (row: Row) => string | number;
  label: string;
  number: number;
  onChange: (page: number) => void;
}) {
  return <>
    <Resource state={state}>{data => <DataTable rows={data.items} columns={columns} rowID={rowID} label={label} />}</Resource>
    <Pagination number={number} more={state?.value?.more ?? false} onChange={onChange} label={label + "翻页"} />
  </>;
}
