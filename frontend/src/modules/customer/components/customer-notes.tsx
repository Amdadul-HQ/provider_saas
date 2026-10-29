"use client"

import { useState } from "react"
import { SendIcon, StickyNoteIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { DateText, EmptyState, QueryState } from "@/components/shared"
import { useT } from "@/i18n"
import { useAddCustomerNote, useCustomerNotes } from "../hooks/use-customers"

export function CustomerNotes({ customerId }: { customerId: string }) {
  const t = useT()
  const [{ data, fetching, error }, refetch] = useCustomerNotes(customerId)
  const [addNote, adding] = useAddCustomerNote()
  const [body, setBody] = useState("")

  return (
    <div className="space-y-4">
      <form
        className="flex items-end gap-2"
        onSubmit={async (e) => {
          e.preventDefault()
          if (!body.trim()) return
          if (await addNote({ customerId, body: body.trim() })) setBody("")
        }}
      >
        <Textarea value={body} onChange={(e) => setBody(e.target.value)} placeholder={t.customers.addNote} rows={2} aria-label={t.customers.addNote} />
        <Button type="submit" size="icon" disabled={adding || !body.trim()} aria-label={t.common.save}>
          <SendIcon />
        </Button>
      </form>
      <QueryState
        data={data?.customerNotes}
        fetching={fetching}
        error={error}
        onRetry={() => refetch({ requestPolicy: "network-only" })}
        isEmpty={(d) => d.length === 0}
        empty={<EmptyState compact icon={StickyNoteIcon} title={t.customers.noNotes} />}
      >
        {(notes) => (
          <ul className="space-y-2">
            {notes.map((n) => (
              <li key={n.id} className="rounded-xl bg-muted/60 p-3">
                <p className="text-sm whitespace-pre-wrap">{n.body}</p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {n.author?.name ?? "—"} · <DateText value={n.createdAt} withTime />
                </p>
              </li>
            ))}
          </ul>
        )}
      </QueryState>
    </div>
  )
}
