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
    <footer className="mx-auto mt-16 w-full max-w-6xl px-4 py-8 lg:px-8">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex items-center gap-3">
          <img src="/logo/gr-logo-blank.png" alt="Gaucho Racing" className="size-12" />
          <span className="font-brand text-3xl font-bold tracking-tight">Gaucho Racing</span>
        </div>
        <span className="font-mono text-xs text-muted-foreground">
          notify{version ? ` · ${version}` : ""}
        </span>
      </div>

      <div className="my-4 h-px w-full bg-gradient-to-r from-[#8412fc] to-[#e105a3]" />

      <div className="flex flex-wrap items-center justify-between gap-4">
        <p className="text-sm text-muted-foreground">
          © 2020 – {new Date().getFullYear()} Gaucho Racing
        </p>
        <div className="flex items-center gap-1">
          {socials.map(({ href, label, Icon }) => (
            <a
              key={label}
              href={href}
              target="_blank"
              rel="noreferrer"
              aria-label={label}
              className="flex size-9 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted/40 hover:text-foreground"
            >
              <Icon className="size-5" />
            </a>
          ))}
        </div>
      </div>
    </footer>
  )
}
