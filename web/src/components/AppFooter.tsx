import { GithubIcon, InstagramIcon, LinkedinIcon, TwitterIcon } from "@/components/icons/socials"
import { SOCIAL_LINKS } from "@/lib/links"
import { useNotifyStatus } from "@/lib/status"

const socials = [
  { href: SOCIAL_LINKS.github, label: "GitHub", Icon: GithubIcon },
  { href: SOCIAL_LINKS.instagram, label: "Instagram", Icon: InstagramIcon },
  { href: SOCIAL_LINKS.twitter, label: "Twitter", Icon: TwitterIcon },
  { href: SOCIAL_LINKS.linkedin, label: "LinkedIn", Icon: LinkedinIcon },
]

export function AppFooter() {
  const { version } = useNotifyStatus()

  return (
    <footer className="mx-auto mt-16 w-full max-w-6xl px-4 pb-8 lg:px-8">
      <div className="flex flex-wrap items-center justify-between gap-4 border-t border-border pt-5">
        <div className="flex items-center gap-3">
          <img src="/logo/gr-logo-blank.png" alt="" className="size-7 opacity-70 grayscale" />
          <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-muted-foreground">
            Gaucho Racing · notify{version ? ` ${version}` : ""} · © {new Date().getFullYear()}
          </span>
        </div>
        <div className="flex items-center gap-1">
          {socials.map(({ href, label, Icon }) => (
            <a
              key={label}
              href={href}
              target="_blank"
              rel="noreferrer"
              aria-label={label}
              className="flex size-8 items-center justify-center rounded-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            >
              <Icon className="size-4" />
            </a>
          ))}
        </div>
      </div>
    </footer>
  )
}
