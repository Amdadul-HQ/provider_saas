"use client"

import type { ReactNode } from "react"
import { flexRender, getCoreRowModel, useReactTable, type ColumnDef } from "@tanstack/react-table"
import type { CombinedError } from "@urql/core"
import { ChevronLeftIcon, ChevronRightIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useT } from "@/i18n"
import { cn } from "@/lib/utils"
import { ErrorState } from "./error-state"

export type CursorPageInfo = { hasNextPage: boolean; endCursor?: string | null }

export type DataTablePagination = {
  canPrev: boolean
  onPrev: () => void
  onNext: (endCursor: string | null | undefined) => void
  pageIndex?: number
}

export type DataTableProps<T> = {
  columns: ColumnDef<T>[]
  data: readonly T[] | undefined
  fetching?: boolean
  error?: CombinedError | null
  onRetry?: () => void
  /** Cursor page info from a `*Connection` (or `arrayPageInfo` for plain lists). */
  pageInfo?: CursorPageInfo
  pagination?: DataTablePagination
  totalCount?: number
  toolbar?: ReactNode
  empty?: ReactNode
  onRowClick?: (row: T) => void
  /** Card renderer used below md (phones). Falls back to a horizontally scrollable table. */
  mobileCard?: (row: T) => ReactNode
  getRowId?: (row: T) => string
  skeletonRows?: number
  className?: string
}

/**
 * Generic server-paginated table (ADR-003: cursor `PageInput`). Pure: it never fetches; the caller
 * passes urql state in. Mobile shows cards, desktop shows a table.
 */
export function DataTable<T>({
  columns,
  data,
  fetching = false,
  error,
  onRetry,
  pageInfo,
  pagination,
  totalCount,
  toolbar,
  empty,
  onRowClick,
  mobileCard,
  getRowId,
  skeletonRows = 6,
  className,
}: DataTableProps<T>) {
  const t = useT()
  const rows = (data ?? []) as T[]
  const table = useReactTable({
    data: rows,
    columns,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    getRowId: getRowId ? (row) => getRowId(row) : undefined,
  })

  const initialLoading = fetching && data === undefined
  const showEmpty = !initialLoading && !error && rows.length === 0
  const showPager = pagination && (pagination.canPrev || pageInfo?.hasNextPage)

  return (
    <div className={cn("space-y-3", className)}>
      {toolbar ? <div className="grid grid-cols-2 gap-2 md:flex md:flex-wrap md:items-center [&>*:only-child]:col-span-2">{toolbar}</div> : null}

      {error && rows.length === 0 ? (
        <ErrorState error={error} onRetry={onRetry} />
      ) : showEmpty ? (
        empty
      ) : (
        <>
          {mobileCard ? (
            <ul className="space-y-2 md:hidden" aria-busy={fetching}>
              {initialLoading
                ? Array.from({ length: skeletonRows }).map((_, i) => (
                    <li key={i}>
                      <Skeleton className="h-20 w-full rounded-xl" />
                    </li>
                  ))
                : table.getRowModel().rows.map((row) => (
                    <li key={row.id}>
                      {onRowClick ? (
                        <button
                          type="button"
                          className="block w-full rounded-xl border bg-card p-3 text-left transition-colors active:bg-muted"
                          onClick={() => onRowClick(row.original)}
                        >
                          {mobileCard(row.original)}
                        </button>
                      ) : (
                        <div className="rounded-xl border bg-card p-3">{mobileCard(row.original)}</div>
                      )}
                    </li>
                  ))}
            </ul>
          ) : null}

          <div className={cn("overflow-hidden rounded-xl border bg-card", mobileCard && "hidden md:block")}>
            <Table>
              <TableHeader className="bg-muted/50">
                {table.getHeaderGroups().map((hg) => (
                  <TableRow key={hg.id}>
                    {hg.headers.map((header) => (
                      <TableHead key={header.id} className="h-10 text-xs font-medium tracking-wide text-muted-foreground uppercase">
                        {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
                      </TableHead>
                    ))}
                  </TableRow>
                ))}
              </TableHeader>
              <TableBody aria-busy={fetching}>
                {initialLoading
                  ? Array.from({ length: skeletonRows }).map((_, i) => (
                      <TableRow key={i}>
                        {columns.map((_c, j) => (
                          <TableCell key={j}>
                            <Skeleton className="h-5 w-full max-w-40" />
                          </TableCell>
                        ))}
                      </TableRow>
                    ))
                  : table.getRowModel().rows.map((row) => (
                      <TableRow
                        key={row.id}
                        className={cn(onRowClick && "cursor-pointer")}
                        onClick={onRowClick ? () => onRowClick(row.original) : undefined}
                      >
                        {row.getVisibleCells().map((cell) => (
                          <TableCell key={cell.id} className="py-3">
                            {flexRender(cell.column.columnDef.cell, cell.getContext())}
                          </TableCell>
                        ))}
                      </TableRow>
                    ))}
              </TableBody>
            </Table>
          </div>
        </>
      )}

      {(showPager || typeof totalCount === "number") && !showEmpty ? (
        <div className="flex items-center justify-between gap-2 text-sm text-muted-foreground">
          <span>{typeof totalCount === "number" ? t.common.showing(rows.length, totalCount) : null}</span>
          {showPager ? (
            <div className="flex gap-2">
              <Button variant="outline" size="sm" onClick={pagination.onPrev} disabled={!pagination.canPrev || fetching}>
                <ChevronLeftIcon /> {t.common.previous}
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => pagination.onNext(pageInfo?.endCursor)}
                disabled={!pageInfo?.hasNextPage || fetching}
              >
                {t.common.next} <ChevronRightIcon />
              </Button>
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}
