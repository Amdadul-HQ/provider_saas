/**
 * `POST /uploads` client (REQ-005, ADR-003: uploads are REST, everything else is GraphQL).
 *
 * Contract (frontend assumption, see SPRINT-02 row 10): multipart/form-data with fields `file` and
 * `kind` (`image` | `media` | `import`); `Authorization: Bearer <access token>` +
 * `X-Requested-With: XMLHttpRequest`; the response is the GraphQL `Upload` shape as JSON
 * (`{ id, url, kind, sizeBytes, createdAt }`, optionally wrapped in `{ upload }`). `url` may be
 * relative to the API origin. XMLHttpRequest (not fetch) because only XHR reports upload progress.
 */
import { refreshSession } from "@/lib/auth/session"
import { apiUrl } from "@/lib/config"

export type UploadKind = "image" | "media" | "import"

export type UploadedFile = { id: string; url: string; kind: string; sizeBytes: number; createdAt?: string }

export type UploadErrorCode = "too_large" | "bad_type" | "network" | "unauthenticated" | "forbidden" | "server" | "aborted"

export class UploadError extends Error {
  constructor(
    readonly code: UploadErrorCode,
    readonly status?: number,
  ) {
    super(`upload failed: ${code}${status ? ` (${status})` : ""}`)
    this.name = "UploadError"
  }
}

const MB = 1024 * 1024
const IMAGE_TYPES = ["image/jpeg", "image/png", "image/webp", "image/gif"]

export const UPLOAD_RULES: Record<UploadKind, { maxBytes: number; types: string[]; extensions: string[] }> = {
  image: { maxBytes: 5 * MB, types: IMAGE_TYPES, extensions: [".jpg", ".jpeg", ".png", ".webp", ".gif"] },
  media: { maxBytes: 10 * MB, types: IMAGE_TYPES, extensions: [".jpg", ".jpeg", ".png", ".webp", ".gif"] },
  import: {
    maxBytes: 20 * MB,
    types: ["text/csv", "text/plain", "application/json", "application/zip", "application/x-zip-compressed", "application/vnd.ms-excel"],
    extensions: [".csv", ".txt", ".json", ".zip"],
  },
}

/** `accept` attribute for an `<input type=file>` of this kind. */
export function acceptFor(kind: UploadKind): string {
  const r = UPLOAD_RULES[kind]
  return [...r.types, ...r.extensions].join(",")
}

/** Client-side check before spending bandwidth; the API validates again. */
export function validateUpload(file: Pick<File, "name" | "size" | "type">, kind: UploadKind): UploadError | null {
  const rule = UPLOAD_RULES[kind]
  if (file.size > rule.maxBytes) return new UploadError("too_large")
  const ext = file.name.toLowerCase().match(/\.[a-z0-9]+$/)?.[0] ?? ""
  if (!rule.types.includes(file.type) && !rule.extensions.includes(ext)) return new UploadError("bad_type")
  return null
}

/** Uploaded/served URLs may be API-relative (`/uploads/<tenant>/x.png`). */
export function resolveMediaUrl(url: string): string {
  return url.startsWith("/") ? apiUrl(url) : url
}

export function parseUploadResponse(body: unknown): UploadedFile | null {
  if (!body || typeof body !== "object") return null
  const o = body as Record<string, unknown>
  const u = (o.upload ?? o.data ?? o) as Record<string, unknown>
  if (typeof u.url !== "string" || !u.url) return null
  return {
    id: typeof u.id === "string" ? u.id : u.url,
    url: resolveMediaUrl(u.url),
    kind: typeof u.kind === "string" ? u.kind : "",
    sizeBytes: typeof u.sizeBytes === "number" ? u.sizeBytes : typeof u.size_bytes === "number" ? u.size_bytes : 0,
    createdAt: typeof u.createdAt === "string" ? u.createdAt : undefined,
  }
}

type Options = { kind: UploadKind; onProgress?: (ratio: number) => void; signal?: AbortSignal }

function send(file: File, { kind, onProgress, signal }: Options): Promise<{ status: number; body: unknown }> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open("POST", apiUrl("/uploads"))
    xhr.responseType = "text"
    xhr.withCredentials = true // the httpOnly psaas_at cookie authenticates (ADR-011)
    xhr.setRequestHeader("X-Requested-With", "XMLHttpRequest")
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && onProgress) onProgress(Math.min(1, e.loaded / e.total))
    }
    xhr.onload = () => {
      let body: unknown = null
      try {
        body = xhr.responseText ? JSON.parse(xhr.responseText) : null
      } catch {
        body = null
      }
      resolve({ status: xhr.status, body })
    }
    xhr.onerror = () => reject(new UploadError("network"))
    xhr.onabort = () => reject(new UploadError("aborted"))
    if (signal) {
      if (signal.aborted) return reject(new UploadError("aborted"))
      signal.addEventListener("abort", () => xhr.abort(), { once: true })
    }
    const form = new FormData()
    form.append("file", file, file.name)
    form.append("kind", kind)
    xhr.send(form)
  })
}

/** Uploads one file; refreshes the session once on 401. Throws `UploadError`. */
export async function uploadFile(file: File, options: Options): Promise<UploadedFile> {
  const invalid = validateUpload(file, options.kind)
  if (invalid) throw invalid
  let res = await send(file, options)
  if (res.status === 401) {
    if ((await refreshSession()) !== "ok") throw new UploadError("unauthenticated", 401)
    res = await send(file, options)
  }
  if (res.status === 401) throw new UploadError("unauthenticated", 401)
  if (res.status === 403) throw new UploadError("forbidden", 403)
  if (res.status === 413) throw new UploadError("too_large", 413)
  if (res.status === 415) throw new UploadError("bad_type", 415)
  if (res.status < 200 || res.status >= 300) throw new UploadError("server", res.status)
  const parsed = parseUploadResponse(res.body)
  if (!parsed) throw new UploadError("server", res.status)
  options.onProgress?.(1)
  return parsed
}
