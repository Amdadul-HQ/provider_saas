"use client"

import dynamic from "next/dynamic"
import { Skeleton } from "@/components/ui/skeleton"

/** recharts is ~100 kB: the public `RevenueChart` loads it after first paint. */
export const RevenueChart = dynamic(() => import("./revenue-chart").then((m) => m.RevenueChart), {
  ssr: false,
  loading: () => <Skeleton className="h-56 w-full rounded-xl" />,
})
