"use client"

import { useState } from "react"
import { ImageIcon, MegaphoneIcon, SendIcon, Trash2Icon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { ConfirmDialog, DateText, EmptyState, ErrorState, FilterSelect, ListSkeleton, PlatformIcon, StatusBadge, ToneBadge } from "@/components/shared"
import type { PostFieldsFragment, PostStatus } from "@/gql/graphql"
import { arrayPageInfo, useCursorPagination } from "@/hooks/use-cursor-pagination"
import { POST_STATUSES } from "@/lib/enums"
import { enumLabel, useT } from "@/i18n"
import { useDeletePost, useGeneratePriceList, usePosts, usePublishPostNow } from "../hooks/use-content"

const PAGE = 20

export function PriceListButton() {
  const t = useT()
  const [generate, busy] = useGeneratePriceList()
  return (
    <Button
      variant="outline"
      disabled={busy}
      onClick={async () => {
        const res = await generate({ format: "png" })
        if (res?.generatePriceList) window.open(res.generatePriceList, "_blank", "noopener")
      }}
    >
      <ImageIcon /> {t.content.priceList}
    </Button>
  )
}

function PostCard({ post, onDelete }: { post: PostFieldsFragment; onDelete: () => void }) {
  const t = useT()
  const [publish, publishing] = usePublishPostNow()
  const canPublish = post.status === "DRAFT" || post.status === "SCHEDULED" || post.status === "FAILED" || post.status === "PARTIAL"
  return (
    <article className="space-y-3 rounded-xl border bg-card p-4">
      <div className="flex items-start justify-between gap-2">
        <StatusBadge enum="PostStatus" value={post.status} />
        <span className="text-xs text-muted-foreground">
          {post.scheduledAt ? <DateText value={post.scheduledAt} withTime /> : <DateText value={post.createdAt} relative />}
        </span>
      </div>
      <p className="line-clamp-4 text-sm whitespace-pre-wrap">{post.body}</p>
      <ul className="flex flex-wrap gap-1.5">
        {post.targets.map((tg) => (
          <li key={tg.id} title={tg.error ?? undefined}>
            <ToneBadge tone={tg.status === "published" ? "success" : tg.status === "failed" ? "danger" : "neutral"}>
              <PlatformIcon platform={tg.socialAccount.platform} className="size-4 [&_svg]:size-2.5" /> {tg.socialAccount.name}
            </ToneBadge>
          </li>
        ))}
      </ul>
      <div className="flex gap-2">
        {canPublish ? (
          <Button size="sm" disabled={publishing} onClick={() => void publish({ id: post.id })}>
            <SendIcon /> {t.content.publishNow}
          </Button>
        ) : null}
        <Button size="sm" variant="ghost" className="text-destructive" onClick={onDelete}>
          <Trash2Icon /> {t.common.delete}
        </Button>
      </div>
    </article>
  )
}

export function PostList() {
  const t = useT()
  const [status, setStatus] = useState<PostStatus | undefined>()
  const pager = useCursorPagination(PAGE)
  const [{ data, fetching, error }, refetch] = usePosts(status, pager.page)
  const [del, deleting] = useDeletePost()
  const [target, setTarget] = useState<PostFieldsFragment | null>(null)
  const posts = data?.posts
  const info = arrayPageInfo(posts, PAGE)

  return (
    <div className="space-y-3">
      <FilterSelect<PostStatus>
        label={t.common.status}
        value={status}
        onChange={(s) => {
          pager.reset()
          setStatus(s)
        }}
        options={POST_STATUSES.map((s) => ({ value: s, label: enumLabel(t, "PostStatus", s) }))}
      />
      {!posts ? (
        error ? (
          <ErrorState error={error} onRetry={() => refetch({ requestPolicy: "network-only" })} />
        ) : fetching ? (
          <ListSkeleton rows={3} />
        ) : null
      ) : posts.length === 0 ? (
        <EmptyState icon={MegaphoneIcon} title={t.content.empty} />
      ) : (
        <>
          <ul className="grid gap-3 lg:grid-cols-2">
            {posts.map((p) => (
              <li key={p.id}>
                <PostCard post={p} onDelete={() => setTarget(p)} />
              </li>
            ))}
          </ul>
          {pager.canPrev || info.hasNextPage ? (
            <div className="flex justify-end gap-2">
              <Button variant="outline" size="sm" disabled={!pager.canPrev} onClick={pager.prev}>
                {t.common.previous}
              </Button>
              <Button variant="outline" size="sm" disabled={!info.hasNextPage} onClick={() => pager.next(info.endCursor)}>
                {t.common.next}
              </Button>
            </div>
          ) : null}
        </>
      )}
      <ConfirmDialog
        open={target !== null}
        onOpenChange={(o) => !o && setTarget(null)}
        title={t.common.delete}
        description={t.content.deleteConfirm}
        destructive
        loading={deleting}
        onConfirm={async () => {
          if (target && (await del({ id: target.id }))) setTarget(null)
        }}
      />
    </div>
  )
}
