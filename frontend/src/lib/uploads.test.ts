import { describe, expect, it } from "vitest"
import { acceptFor, parseUploadResponse, UPLOAD_RULES, validateUpload } from "./uploads"

describe("uploads helpers", () => {
  it("validates size and type per kind", () => {
    expect(validateUpload({ name: "a.png", size: 1000, type: "image/png" }, "image")).toBeNull()
    expect(validateUpload({ name: "a.png", size: UPLOAD_RULES.image.maxBytes + 1, type: "image/png" }, "image")?.code).toBe("too_large")
    expect(validateUpload({ name: "a.exe", size: 10, type: "application/x-msdownload" }, "image")?.code).toBe("bad_type")
    // browsers often report "" for .json/.txt chat exports: the extension decides
    expect(validateUpload({ name: "WhatsApp Chat.txt", size: 10, type: "" }, "import")).toBeNull()
    expect(validateUpload({ name: "result.json", size: 10, type: "" }, "import")).toBeNull()
  })

  it("parses the Upload JSON (plain or wrapped) and resolves API-relative urls", () => {
    const plain = parseUploadResponse({ id: "u1", url: "/uploads/t/x.png", kind: "image", sizeBytes: 12 })
    expect(plain?.id).toBe("u1")
    expect(plain?.url).toMatch(/^https?:\/\/.+\/uploads\/t\/x\.png$/)
    expect(parseUploadResponse({ upload: { id: "u2", url: "https://cdn.test/y.png", kind: "image", sizeBytes: 3 } })?.url).toBe("https://cdn.test/y.png")
    expect(parseUploadResponse({ error: "nope" })).toBeNull()
    expect(parseUploadResponse(null)).toBeNull()
  })

  it("builds the accept attribute", () => {
    expect(acceptFor("import")).toContain(".csv")
    expect(acceptFor("image")).toContain("image/png")
  })
})
