import { NextResponse, type NextRequest } from "next/server"
import { buildCsp, makeNonce } from "@/lib/security/csp"

/** Per-request CSP nonce (SPRINT-01-SECURITY M6). The other security headers live in next.config.ts. */
export function middleware(request: NextRequest) {
  const nonce = makeNonce()
  const csp = buildCsp({ nonce, dev: process.env.NODE_ENV === "development", apiUrl: process.env.NEXT_PUBLIC_GRAPHQL_URL ?? "http://localhost:8080/graphql" })
  const headers = new Headers(request.headers)
  headers.set("x-nonce", nonce)
  // Next.js reads the nonce from the *request* CSP header and applies it to its own scripts
  headers.set("Content-Security-Policy", csp)
  const response = NextResponse.next({ request: { headers } })
  response.headers.set("Content-Security-Policy", csp)
  return response
}

export const config = {
  matcher: [
    {
      // documents only: skip static assets and prefetches
      source: "/((?!_next/static|_next/image|favicon.ico|icons/|sw.js|manifest.json).*)",
      missing: [
        { type: "header", key: "next-router-prefetch" },
        { type: "header", key: "purpose", value: "prefetch" },
      ],
    },
  ],
}
