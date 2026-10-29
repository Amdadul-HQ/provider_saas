import type { NextConfig } from "next"

const isProd = process.env.NODE_ENV === "production"

// Content-Security-Policy is NOT here: production needs a per-request nonce, so it is built in
// src/middleware.ts (see src/lib/security/csp.ts). Everything static lives here.
const securityHeaders = [
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" },
  // HSTS only in production builds: on http://localhost it would be ignored at best, sticky at worst
  ...(isProd ? [{ key: "Strict-Transport-Security", value: "max-age=31536000; includeSubDomains" }] : []),
]

const nextConfig: NextConfig = {
  reactStrictMode: true,
  poweredByHeader: false,
  // lets `pnpm build` run next to a live `next dev` without fighting over .next (NEXT_DIST_DIR=.next-build)
  distDir: process.env.NEXT_DIST_DIR || ".next",
  async headers() {
    return [
      { source: "/:path*", headers: securityHeaders },
      // magic-link portal and invite links: never leak the token through the Referer header
      { source: "/p/:path*", headers: [{ key: "Referrer-Policy", value: "no-referrer" }] },
      { source: "/invite/:path*", headers: [{ key: "Referrer-Policy", value: "no-referrer" }] },
      { source: "/sw.js", headers: [{ key: "Cache-Control", value: "no-cache" }, { key: "Service-Worker-Allowed", value: "/" }] },
    ]
  },
}

export default nextConfig
