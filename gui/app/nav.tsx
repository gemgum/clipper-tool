"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { LANGUAGES, useI18n, type MessageKey } from "./i18n";

const TABS: { href: string; label: MessageKey }[] = [
  { href: "/", label: "tabClips" },
  { href: "/news", label: "tabNews" },
  { href: "/requirements", label: "tabRequirements" },
];

export default function Nav() {
  const path = usePathname();
  const { lang, setLang, t } = useI18n();

  return (
    <nav className="nav">
      <div className="nav-inner">
        <span className="brand">Clipper</span>
        {TABS.map((tab) => (
          <Link
            key={tab.href}
            href={tab.href}
            className={"nav-tab" + (path === tab.href ? " active" : "")}
          >
            {t(tab.label)}
          </Link>
        ))}
        <div className="lang-switch">
          {LANGUAGES.map((l) => (
            <button
              key={l}
              type="button"
              className={l === lang ? "active" : ""}
              aria-pressed={l === lang}
              onClick={() => setLang(l)}
            >
              {l.toUpperCase()}
            </button>
          ))}
        </div>
      </div>
    </nav>
  );
}
