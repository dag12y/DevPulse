"use client";

import { useMemo, useState, type ReactNode } from "react";
import { downloadCSV, toCSV } from "@/lib/csv";

export interface ReportColumn<T> {
  key: string;
  label: string;
  numeric?: boolean;
  value: (row: T) => string | number;
  render?: (row: T) => ReactNode;
}

interface ReportTableProps<T> {
  rows: T[];
  columns: ReportColumn<T>[];
  rowKey: (row: T) => string;
  searchPlaceholder: string;
  csvFilename: string;
  emptyMessage: string;
  defaultSortKey?: string;
  pageSize?: number;
}

type SortDirection = "asc" | "desc";

const PAGE_SIZE_DEFAULT = 15;

export default function ReportTable<T>({
  rows,
  columns,
  rowKey,
  searchPlaceholder,
  csvFilename,
  emptyMessage,
  defaultSortKey,
  pageSize = PAGE_SIZE_DEFAULT,
}: ReportTableProps<T>) {
  const [query, setQuery] = useState("");
  const [sortKey, setSortKey] = useState<string | null>(defaultSortKey ?? null);
  const [sortDirection, setSortDirection] = useState<SortDirection>("desc");
  const [page, setPage] = useState(0);

  const toggleSort = (key: string) => {
    setPage(0);
    if (sortKey !== key) {
      setSortKey(key);
      setSortDirection("desc");
      return;
    }
    setSortDirection((direction) => (direction === "desc" ? "asc" : "desc"));
  };

  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return rows;
    return rows.filter((row) =>
      columns.some((column) => String(column.value(row)).toLowerCase().includes(needle)),
    );
  }, [rows, columns, query]);

  const sorted = useMemo(() => {
    if (!sortKey) return filtered;
    const column = columns.find((candidate) => candidate.key === sortKey);
    if (!column) return filtered;
    const direction = sortDirection === "asc" ? 1 : -1;
    return [...filtered].sort((a, b) => {
      const left = column.value(a);
      const right = column.value(b);
      if (typeof left === "number" && typeof right === "number") return (left - right) * direction;
      return String(left).localeCompare(String(right)) * direction;
    });
  }, [filtered, columns, sortKey, sortDirection]);

  const pageCount = Math.max(1, Math.ceil(sorted.length / pageSize));
  const safePage = Math.min(page, pageCount - 1);
  const visible = sorted.slice(safePage * pageSize, safePage * pageSize + pageSize);

  const sortIndicator = (key: string): string => {
    if (sortKey !== key) return "";
    return sortDirection === "desc" ? " ▼" : " ▲";
  };

  return (
    <div className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <label className="sr-only" htmlFor={`${csvFilename}-search`}>
          {searchPlaceholder}
        </label>
        <input
          id={`${csvFilename}-search`}
          type="search"
          value={query}
          onChange={(event) => {
            setQuery(event.target.value);
            setPage(0);
          }}
          placeholder={searchPlaceholder}
          className="min-w-48 flex-1 rounded-md border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-3 py-1.5 text-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600"
        />
        <button
          type="button"
          onClick={() => downloadCSV(csvFilename, toCSV(columns, sorted))}
          disabled={sorted.length === 0}
          className="rounded-md border border-zinc-300 dark:border-zinc-700 px-3 py-1.5 text-sm font-medium text-zinc-700 dark:text-zinc-300 hover:bg-zinc-100 dark:hover:bg-zinc-800 disabled:opacity-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600"
        >
          Export CSV
        </button>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-zinc-500 border-b border-zinc-200 dark:border-zinc-800">
              {columns.map((column) => (
                <th
                  key={column.key}
                  aria-sort={
                    sortKey === column.key
                      ? sortDirection === "desc"
                        ? "descending"
                        : "ascending"
                      : "none"
                  }
                  className={`pb-2 font-medium ${column.numeric ? "text-right" : ""}`}
                >
                  <button
                    type="button"
                    onClick={() => toggleSort(column.key)}
                    aria-label={`Sort by ${column.label}`}
                    className="rounded px-1 py-0.5 hover:text-zinc-900 dark:hover:text-zinc-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600"
                  >
                    {column.label}
                    <span aria-hidden="true">{sortIndicator(column.key)}</span>
                  </button>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {visible.map((row) => (
              <tr key={rowKey(row)} className="border-b border-zinc-100 dark:border-zinc-800/50">
                {columns.map((column) => (
                  <td
                    key={column.key}
                    className={`py-2 ${column.numeric ? "text-right text-zinc-600 dark:text-zinc-400" : "text-zinc-800 dark:text-zinc-200"}`}
                  >
                    {column.render ? column.render(row) : formatCell(column.value(row))}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {sorted.length === 0 && (
        <p className="py-6 text-center text-sm text-zinc-500">{query ? `No matches for “${query}”.` : emptyMessage}</p>
      )}

      {pageCount > 1 && (
        <nav aria-label="Table pages" className="mt-3 flex items-center justify-between text-sm">
          <p className="text-zinc-500">
            Page {safePage + 1} of {pageCount} · {sorted.length} rows
          </p>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={() => setPage((current) => Math.max(0, current - 1))}
              disabled={safePage === 0}
              className="rounded-md border border-zinc-300 dark:border-zinc-700 px-3 py-1 disabled:opacity-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600"
            >
              Previous
            </button>
            <button
              type="button"
              onClick={() => setPage((current) => Math.min(pageCount - 1, current + 1))}
              disabled={safePage >= pageCount - 1}
              className="rounded-md border border-zinc-300 dark:border-zinc-700 px-3 py-1 disabled:opacity-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600"
            >
              Next
            </button>
          </div>
        </nav>
      )}
    </div>
  );
}

function formatCell(value: string | number): string {
  return typeof value === "number" ? value.toLocaleString() : value;
}
