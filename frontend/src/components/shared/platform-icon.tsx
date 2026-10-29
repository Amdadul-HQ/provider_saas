import { AtSignIcon, CameraIcon, MailIcon, MessageCircleIcon, MessageSquareIcon, PhoneIcon, SendIcon } from "lucide-react"
import type { Platform, SocialPlatform } from "@/gql/graphql"
import { cn } from "@/lib/utils"

const STYLE: Record<Platform, { className: string; Icon: typeof SendIcon }> = {
  FACEBOOK: { className: "bg-[#1877F2] text-white", Icon: MessageCircleIcon },
  INSTAGRAM: { className: "bg-gradient-to-br from-[#F58529] via-[#DD2A7B] to-[#8134AF] text-white", Icon: CameraIcon },
  WHATSAPP: { className: "bg-[#25D366] text-white", Icon: PhoneIcon },
  TELEGRAM: { className: "bg-[#229ED9] text-white", Icon: SendIcon },
  SMS: { className: "bg-muted text-foreground", Icon: MessageSquareIcon },
  EMAIL: { className: "bg-muted text-foreground", Icon: MailIcon },
}

/** Brand-coloured chip (lucide v1 ships no brand logos). */
export function PlatformIcon({ platform, className }: { platform: Platform | SocialPlatform | null | undefined; className?: string }) {
  const style = platform ? STYLE[platform] : { className: "bg-muted text-muted-foreground", Icon: AtSignIcon }
  const { Icon } = style
  return (
    <span className={cn("inline-flex size-6 shrink-0 items-center justify-center rounded-full", style.className, className)} aria-hidden>
      <Icon className="size-3.5" />
    </span>
  )
}
