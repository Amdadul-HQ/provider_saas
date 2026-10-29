"use client"

import type { MouseEvent, ReactNode } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { flexRender, getCoreRowModel, useReactTable, type ColumnDef, type RowData } from "@tanstack/react-table"
import type { CombinedError } from "@urql/core"
import { ChevronLeftIcon, ChevronRightIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useT } from "@/i18n"
import { cn } from "@/lib/utils"
import { ErrorState } from "./error-state"

declare module "@tanstack/react-table" {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars -- must mirror the library's generics to merge
  interface ColumnMeta<TData extends RowData, TValue> {
    /** Pin the column to the right edge so row actions stay reachable when the table scrolls (BUG-16). */
    sticky?: "right"
  }
}

const STICKY_RIGHT = "sticky right-0 z-10 bg-card shadow-[inset_1px_0_0_var(--color-border)]"

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
  /**
   * Detail URL for a row (BUG-10). Rows become real links: mobile cards are `<a>`, the desktop
   * row's `linkColumn` cell is wrapped in an `<a>` (keyboard, open-in-new-tab) and a click anywhere
   * else on the row navigates too. Prefer this over `onRowClick` for navigation.
   */
  rowHref?: (row: T) => string
  /** Column id whose cell holds the row link (default: first column). Must not contain other controls. */
  linkColumn?: string
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
  rowHref,
  linkColumn,
  mobileCard,
  getRowId,
  skeletonRows = 6,
  className,
}: DataTableProps<T>) {
  const t = useT()
  const router = useRouter()
  const rows = (data ?? []) as T[]
  const linkColumnId = linkColumn ?? columns[0]?.id
  const activateRow = (e: MouseEvent, row: T) => {
    if (rowHref) {
      // let real controls (and the cell link itself) handle their own clicks; keep text selection usable
      if ((e.target as HTMLElement).closest("a,button,input,select,textarea,label,[role=combobox],[role=checkbox],[role=menuitem]")) return
      if (window.getSelection()?.toString()) return
      const href = rowHref(row)
      if (e.metaKey || e.ctrlKey) window.open(href, "_blank", "noopener")
      else router.push(href)
      return
    }
    onRowClick?.(row)
  }
  const clickable = Boolean(rowHref || onRowClick)
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
            <ul className="stagger space-y-2 md:hidden" aria-busy={fetching}>
              {initialLoading
                ? Array.from({ length: skeletonRows }).map((_, i) => (
                    <li key={i}>
                      <Skeleton className="h-20 w-full rounded-2xl" />
                    </li>
                  ))
                : table.getRowModel().rows.map((row) => (
                    <li key={row.id}>
                      {rowHref ? (
                        <Link
                          href={rowHref(row.original)}
                          className="block w-full rounded-2xl border border-border/80 bg-card p-3.5 text-left shadow-xs transition-[background-color,transform] duration-(--dur-fast) focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none active:scale-[0.99] active:bg-muted"
                        >
                          {mobileCard(row.original)}
                        </Link>
                      ) : onRowClick ? (
                        <button
                          type="button"
                          className="block w-full rounded-2xl border border-border/80 bg-card p-3.5 text-left shadow-xs transition-[background-color,transform] duration-(--dur-fast) active:scale-[0.99] active:bg-muted"
                          onClick={() => onRowClick(row.original)}
                        >
                          {mobileCard(row.original)}
                        </button>
                      ) : (
                        <div className="rounded-2xl border border-border/80 bg-card p-3.5 shadow-xs">{mobileCard(row.original)}</div>
                      )}
                    </li>
                  ))}
            </ul>
          ) : null}

          <div className={cn("overflow-hidden rounded-2xl border border-border/80 bg-card shadow-xs", mobileCard && "hidden md:block")}>
            <Table>
              <TableHeader className="bg-muted/50">
                {table.getHeaderGroups().map((hg) => (
                  <TableRow key={hg.id}>
                    {hg.headers.map((header) => (
                      <TableHead
                        key={header.id}
                        className={cn(
                          "h-10 text-xs font-medium text-muted-foreground",
                          header.column.columnDef.meta?.sticky === "right" && cn(STICKY_RIGHT, "bg-muted"),
                        )}
                      >
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
                        className={cn(clickable && "cursor-pointer")}
                        onClick={clickable ? (e) => activateRow(e, row.original) : undefined}
                      >
                        {row.getVisibleCells().map((cell) => (
                          <TableCell key={cell.id} className={cn("py-3", cell.column.columnDef.meta?.sticky === "right" && STICKY_RIGHT)}>
                            {rowHref && cell.column.id === linkColumnId ? (
                              <Link
                                href={rowHref(row.original)}
                                className="block rounded-sm hover:underline focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
                              >
                                {flexRender(cell.column.columnDef.cell, cell.getContext())}
                              </Link>
                            ) : (
                              flexRender(cell.column.columnDef.cell, cell.getContext())
                            )}
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
