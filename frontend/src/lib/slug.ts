/** Mirrors backend tenant.Slugify for a store-link preview; null when the API would fall back to "shop-…". */
export function previewSlug(name: string): string | null {
  let s = name.trim().toLowerCase().replaceAll("'", "").replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "")
  if (s.length > 40) s = s.slice(0, 40).replace(/^-+|-+$/g, "")
  return s.length >= 3 ? s : null
}
