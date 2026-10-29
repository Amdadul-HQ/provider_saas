"use client"

import { useState } from "react"
import { CheckIcon, CopyIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { useT } from "@/i18n"

export function CopyButton({ value, label, size = "sm" }: { value: string; label?: string; size?: "sm" | "icon-sm" }) {
  const t = useT()
  const [copied, setCopied] = useState(false)
  return (
    <Button
      type="button"
      variant="outline"
      size={size}
      aria-label={label ?? t.common.copy}
      onClick={async () => {
        await navigator.clipboard.writeText(value)
        setCopied(true)
        setTimeout(() => setCopied(false), 1500)
      }}
    >
      {copied ? <CheckIcon /> : <CopyIcon />}
      {size === "sm" ? (copied ? t.common.copied : (label ?? t.common.copy)) : null}
    </Button>
  )
}
