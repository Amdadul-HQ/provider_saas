import Link from "next/link"

export default function NotFound() {
  return (
    <div className="flex min-h-dvh flex-col items-center justify-center gap-3 p-6 text-center">
      <p className="font-heading text-5xl font-bold text-primary">404</p>
      <p className="text-muted-foreground">পাতাটি খুঁজে পাওয়া যায়নি · Page not found</p>
      <Link href="/dashboard" className="text-sm font-medium text-primary hover:underline">
        ড্যাশবোর্ড · Dashboard
      </Link>
    </div>
  )
}
