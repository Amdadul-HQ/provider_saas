import { describe, expect, it } from "vitest"
import { previewSlug } from "./slug"

describe("previewSlug", () => {
  it("matches tenant.Slugify", () => {
    expect(previewSlug("  Nodi Streaming Shop ")).toBe("nodi-streaming-shop")
    expect(previewSlug("Rafi's  Store!!")).toBe("rafis-store")
  })
  it("returns null when the API would fall back", () => {
    expect(previewSlug("স্ট্রিমিং শপ")).toBeNull()
    expect(previewSlug("ab")).toBeNull()
  })
})
