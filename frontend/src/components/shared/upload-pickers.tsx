"use client"

import { useCallback, useId, useRef, useState, type DragEvent, type ReactNode } from "react"
import { FileTextIcon, ImagePlusIcon, Loader2Icon, RefreshCwIcon, UploadCloudIcon, XIcon } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Progress } from "@/components/ui/progress"
import { useFormat, useT, type Dict } from "@/i18n"
import { acceptFor, resolveMediaUrl, UploadError, uploadFile, UPLOAD_RULES, type UploadedFile, type UploadKind } from "@/lib/uploads"
import { cn } from "@/lib/utils"

export function uploadErrorMessage(t: Dict, err: unknown): string {
  const code = err instanceof UploadError ? err.code : "network"
  return t.uploads.errors[code]
}

/** Upload state machine shared by the pickers: progress 0..1, one file at a time per call. */
export function useUpload(kind: UploadKind) {
  const t = useT()
  const [progress, setProgress] = useState<number | null>(null)
  const upload = useCallback(
    async (file: File): Promise<UploadedFile | null> => {
      setProgress(0)
      try {
        return await uploadFile(file, { kind, onProgress: setProgress })
      } catch (err) {
        if (!(err instanceof UploadError && err.code === "aborted")) toast.error(uploadErrorMessage(t, err))
        return null
      } finally {
        setProgress(null)
      }
    },
    [kind, t],
  )
  return { upload, progress, uploading: progress !== null }
}

function useDrop(onFiles: (files: File[]) => void, disabled?: boolean) {
  const [over, setOver] = useState(false)
  return {
    over,
    props: {
      onDragOver: (e: DragEvent) => {
        if (disabled) return
        e.preventDefault()
        setOver(true)
      },
      onDragLeave: () => setOver(false),
      onDrop: (e: DragEvent) => {
        e.preventDefault()
        setOver(false)
        if (!disabled) onFiles(Array.from(e.dataTransfer.files))
      },
    },
  }
}

function sizeLabel(t: Dict, kind: UploadKind) {
  return t.uploads.maxSize(Math.round(UPLOAD_RULES[kind].maxBytes / (1024 * 1024)))
}

/**
 * Single image (product image, storefront logo/cover). Stores the uploaded URL; `aspect="wide"` for covers.
 */
export function ImagePicker({
  value,
  onChange,
  label,
  aspect = "square",
  disabled,
  className,
}: {
  value: string | null | undefined
  onChange: (url: string | null) => void
  label?: ReactNode
  aspect?: "square" | "wide"
  disabled?: boolean
  className?: string
}) {
  const t = useT()
  const id = useId()
  const input = useRef<HTMLInputElement>(null)
  const { upload, progress, uploading } = useUpload("image")
  const pick = async (files: File[]) => {
    const file = files[0]
    if (!file) return
    const res = await upload(file)
    if (res) onChange(res.url)
  }
  const drop = useDrop((f) => void pick(f), disabled || uploading)
  const box = aspect === "wide" ? "aspect-[3/1] w-full" : "size-28"
  return (
    <div className={cn("space-y-1.5", className)}>
      {label ? (
        <label htmlFor={id} className="text-sm font-medium">
          {label}
        </label>
      ) : null}
      <div className="flex items-end gap-3">
        <div
          {...drop.props}
          className={cn(
            "relative flex shrink-0 items-center justify-center overflow-hidden rounded-xl border border-dashed bg-muted/40 transition-colors",
            box,
            drop.over && "border-primary bg-primary/5",
          )}
        >
          {value ? (
            // eslint-disable-next-line @next/next/no-img-element -- user uploads from the API origin; next/image would need remotePatterns per deployment
            <img src={resolveMediaUrl(value)} alt="" className="size-full object-cover" />
          ) : (
            <button
              type="button"
              id={id}
              disabled={disabled || uploading}
              onClick={() => input.current?.click()}
              className="flex size-full flex-col items-center justify-center gap-1 p-2 text-center text-xs text-muted-foreground hover:text-foreground"
            >
              <ImagePlusIcon className="size-5" aria-hidden />
              {t.uploads.chooseImage}
            </button>
          )}
          {uploading ? (
            <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 bg-background/80 p-3">
              <Loader2Icon className="size-5 animate-spin text-primary" aria-hidden />
              <Progress value={(progress ?? 0) * 100} aria-label={t.uploads.uploading} className="h-1.5" />
            </div>
          ) : null}
        </div>
        <div className="flex min-w-0 flex-col gap-1.5">
          {value ? (
            <>
              <Button type="button" variant="outline" size="sm" disabled={disabled || uploading} onClick={() => input.current?.click()}>
                <RefreshCwIcon /> {t.uploads.replace}
              </Button>
              <Button type="button" variant="ghost" size="sm" disabled={disabled || uploading} onClick={() => onChange(null)}>
                <XIcon /> {t.uploads.remove}
              </Button>
            </>
          ) : null}
          <p className="text-xs text-muted-foreground">{sizeLabel(t, "image")}</p>
        </div>
      </div>
      <input
        ref={input}
        type="file"
        hidden
        accept={acceptFor("image")}
        onChange={(e) => {
          void pick(Array.from(e.target.files ?? []))
          e.target.value = ""
        }}
      />
    </div>
  )
}

/** Several images (post media): preview grid with remove buttons, uploads sequentially. */
export function MultiImagePicker({
  values,
  onChange,
  max = 10,
  label,
  disabled,
}: {
  values: string[]
  onChange: (urls: string[]) => void
  max?: number
  label?: ReactNode
  disabled?: boolean
}) {
  const t = useT()
  const input = useRef<HTMLInputElement>(null)
  const { upload, progress, uploading } = useUpload("media")
  const latest = useRef(values)
  latest.current = values
  const pick = async (files: File[]) => {
    const room = Math.max(0, max - latest.current.length)
    if (files.length > room) toast.warning(t.uploads.tooMany(max))
    for (const file of files.slice(0, room)) {
      const res = await upload(file)
      if (res) onChange([...latest.current, res.url])
    }
  }
  const drop = useDrop((f) => void pick(f), disabled || uploading)
  const full = values.length >= max
  return (
    <div className="space-y-1.5">
      {label ? <p className="text-sm font-medium">{label}</p> : null}
      <ul {...drop.props} className={cn("grid grid-cols-3 gap-2 rounded-xl sm:grid-cols-4", drop.over && "ring-2 ring-primary/40")} aria-label={t.uploads.mediaGrid}>
        {values.map((url, i) => (
          <li key={`${url}-${i}`} className="group relative aspect-square overflow-hidden rounded-xl border bg-muted">
            {/* eslint-disable-next-line @next/next/no-img-element -- user uploads from the API origin */}
            <img src={resolveMediaUrl(url)} alt={t.uploads.imageN(i + 1)} className="size-full object-cover" />
            <Button
              type="button"
              size="icon-sm"
              variant="secondary"
              className="absolute top-1 right-1 size-7 rounded-full bg-background/90 shadow"
              aria-label={`${t.uploads.remove} ${i + 1}`}
              disabled={disabled}
              onClick={() => onChange(values.filter((_, j) => j !== i))}
            >
              <XIcon />
            </Button>
          </li>
        ))}
        {uploading ? (
          <li className="flex aspect-square flex-col items-center justify-center gap-2 rounded-xl border border-dashed p-3">
            <Loader2Icon className="size-5 animate-spin text-primary" aria-hidden />
            <Progress value={(progress ?? 0) * 100} aria-label={t.uploads.uploading} className="h-1.5" />
          </li>
        ) : null}
        {!full && !uploading ? (
          <li>
            <button
              type="button"
              disabled={disabled}
              onClick={() => input.current?.click()}
              className="flex aspect-square w-full flex-col items-center justify-center gap-1 rounded-xl border border-dashed bg-muted/40 p-2 text-center text-xs text-muted-foreground transition-colors hover:border-primary/40 hover:text-foreground"
            >
              <ImagePlusIcon className="size-5" aria-hidden />
              {t.uploads.addImages}
            </button>
          </li>
        ) : null}
      </ul>
      <p className="text-xs text-muted-foreground">
        {t.uploads.countOf(values.length, max)} · {sizeLabel(t, "media")}
      </p>
      <input
        ref={input}
        type="file"
        hidden
        multiple
        accept={acceptFor("media")}
        onChange={(e) => {
          void pick(Array.from(e.target.files ?? []))
          e.target.value = ""
        }}
      />
    </div>
  )
}

/** Any file (import wizard): drop zone → upload → `onUploaded`. Shows name, size and progress. */
export function FilePicker({
  kind = "import",
  onUploaded,
  uploaded,
  onClear,
  hint,
  disabled,
}: {
  kind?: UploadKind
  onUploaded: (file: UploadedFile, original: File) => void
  /** the file already chosen (shown instead of the drop zone) */
  uploaded?: { name: string; sizeBytes: number } | null
  onClear?: () => void
  hint?: ReactNode
  disabled?: boolean
}) {
  const t = useT()
  const f = useFormat()
  const input = useRef<HTMLInputElement>(null)
  const { upload, progress, uploading } = useUpload(kind)
  const [current, setCurrent] = useState<File | null>(null)
  const pick = async (files: File[]) => {
    const file = files[0]
    if (!file) return
    setCurrent(file)
    const res = await upload(file)
    if (res) onUploaded(res, file)
    else setCurrent(null)
  }
  const drop = useDrop((fs) => void pick(fs), disabled || uploading)
  const kb = (bytes: number) => `${f.number(Math.max(1, Math.round(bytes / 1024)))} KB`

  if (uploaded && !uploading) {
    return (
      <div className="flex items-center gap-3 rounded-xl border bg-card p-3">
        <span className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-secondary text-primary">
          <FileTextIcon className="size-5" aria-hidden />
        </span>
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium">{uploaded.name}</p>
          <p className="text-xs text-muted-foreground">{kb(uploaded.sizeBytes)} · {t.uploads.uploaded}</p>
        </div>
        {onClear ? (
          <Button type="button" variant="ghost" size="sm" onClick={onClear} disabled={disabled}>
            <RefreshCwIcon /> {t.uploads.replace}
          </Button>
        ) : null}
      </div>
    )
  }
  return (
    <div
      {...drop.props}
      className={cn(
        "flex flex-col items-center justify-center gap-3 rounded-2xl border-2 border-dashed bg-muted/30 p-6 text-center transition-colors",
        drop.over && "border-primary bg-primary/5",
      )}
    >
      {uploading && current ? (
        <div className="w-full max-w-sm space-y-2">
          <p className="truncate text-sm font-medium">{current.name}</p>
          <Progress value={(progress ?? 0) * 100} aria-label={t.uploads.uploading} className="h-2" />
          <p className="tabular text-xs text-muted-foreground">
            {t.uploads.uploading} {f.percent(progress ?? 0)}
          </p>
        </div>
      ) : (
        <>
          <span className="flex size-12 items-center justify-center rounded-full bg-secondary text-primary">
            <UploadCloudIcon className="size-6" aria-hidden />
          </span>
          <div className="space-y-1">
            <p className="text-sm font-medium">{t.uploads.dropHere}</p>
            {hint ? <p className="text-xs text-muted-foreground">{hint}</p> : null}
            <p className="text-xs text-muted-foreground">{sizeLabel(t, kind)}</p>
          </div>
          <Button type="button" variant="outline" disabled={disabled} onClick={() => input.current?.click()}>
            {t.uploads.chooseFile}
          </Button>
        </>
      )}
      <input
        ref={input}
        type="file"
        hidden
        accept={acceptFor(kind)}
        onChange={(e) => {
          void pick(Array.from(e.target.files ?? []))
          e.target.value = ""
        }}
      />
    </div>
  )
}
