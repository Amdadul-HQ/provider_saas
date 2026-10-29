"use client"

import type { ReactNode } from "react"
import { Loader2Icon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { useT } from "@/i18n"
import { ResponsiveDialog } from "./responsive-dialog"

export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel,
  destructive = false,
  loading = false,
  onConfirm,
  children,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: ReactNode
  description?: ReactNode
  confirmLabel?: string
  destructive?: boolean
  loading?: boolean
  onConfirm: () => void | Promise<void>
  children?: ReactNode
}) {
  const t = useT()
  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={title}
      description={description}
      footer={
        <>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={loading}>
            {t.common.cancel}
          </Button>
          <Button variant={destructive ? "destructive" : "default"} onClick={() => void onConfirm()} disabled={loading}>
            {loading ? <Loader2Icon className="animate-spin" /> : null}
            {confirmLabel ?? t.common.confirm}
          </Button>
        </>
      }
    >
      {children}
    </ResponsiveDialog>
  )
}
