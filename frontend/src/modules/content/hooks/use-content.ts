"use client"

import { useQuery } from "urql"
import {
  CreatePostDocument,
  DeletePostDocument,
  GeneratePriceListDocument,
  PostsDocument,
  PublishPostNowDocument,
  UpdatePostDocument,
  type PageInput,
  type PostStatus,
} from "@/gql/graphql"
import { typenameContext, useAction } from "@/lib/graphql/use-action"
import { useT } from "@/i18n"

export function usePosts(status: PostStatus | undefined, page?: PageInput) {
  return useQuery({ query: PostsDocument, variables: { status, page }, context: typenameContext("Post") })
}

export function useCreatePost() {
  const t = useT()
  return useAction(CreatePostDocument, { success: t.content.saved, typenames: ["Post"] })
}

export function useUpdatePost() {
  const t = useT()
  return useAction(UpdatePostDocument, { success: t.content.saved })
}

export function usePublishPostNow() {
  const t = useT()
  return useAction(PublishPostNowDocument, { success: t.content.published })
}

export function useDeletePost() {
  const t = useT()
  return useAction(DeletePostDocument, { success: t.content.deleted, typenames: ["Post"] })
}

export function useGeneratePriceList() {
  const t = useT()
  return useAction(GeneratePriceListDocument, { success: t.content.priceListReady })
}
