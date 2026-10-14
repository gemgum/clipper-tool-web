"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Scissors, Newspaper, PenLine, Captions, Droplet, History, Settings } from "lucide-react";
import { useI18n, type MessageKey } from "./i18n";
import AccountButton from "./account";
import { RailAI, useRunning } from "./rail-status";
import { useMe } from "./engine";

// Navigasi kiri — rancangan 14 Oktober 2026: SATU rail sempit di semua lebar
// jendela, ikon dengan label pendek di bawahnya. Alat di atas, Riwayat
// dipisah garis, Pengaturan + status AI + akun di kaki. Nama lengkapnya tetap
// di aria-label dan tooltip.

// perm = izin Sarth Admin yang dibutuhkan halaman itu (engine/internal/api/access.go).
type Item = { href: string; label: MessageKey; short: MessageKey; Icon: typeof Scissors; perm?: string };
const TOOLS: Item[] = [
  { href: "/", label: "navClips", short: "navShortClips", Icon: Scissors, perm: "clip.view" },
  { href: "/news", label: "navNews", short: "navShortNews", Icon: Newspaper, perm: "card.view" },
  { href: "/writer", label: "navWriter", short: "navShortWriter", Icon: PenLine, perm: "article.view" },
  { href: "/captions", label: "navCaptions", short: "navShortCaptions", Icon: Captions, perm: "caption.view" },
  { href: "/watermark", label: "navWatermark", short: "navShortWatermark", Icon: Droplet, perm: "watermark.view" },
];
const HISTORY: Item = { href: "/history", label: "navHistory", short: "navHistory", Icon: History };
const SETTINGS: Item = { href: "/requirements", label: "navSettings", short: "navSettings", Icon: Settings, perm: "settings.view" };

// Ekspor statis Next menghasilkan /news/index.html, jadi alamat yang terbaca
// browser berakhir dengan garis miring ("/news/") sedangkan href di daftar
// ditulis tanpa ("/news"). Membandingkannya mentah-mentah membuat SELURUH
// halaman selain "/" kehilangan penanda posisinya — terukur, bukan dugaan.
const samePath = (a: string, b: string) =>
  (a.replace(/\/+$/, "") || "/") === (b.replace(/\/+$/, "") || "/");

export default function Nav() {
  const path = usePathname();
  const { t } = useI18n();
  const running = useRunning();
  const me = useMe();
  // Tanpa login Sarth Admin (me = null) semua menu tampil, seperti dulu.
  const allowed = (i: Item) => !me || !i.perm || me.permissions.includes(i.perm);

  const item = ({ href, label, short, Icon }: Item) => {
    const on = samePath(path, href);
    const badge = href === "/history" && running.length > 0 ? running.length : 0;
    return (
      <Link key={href} href={href} className={"rail-item" + (on ? " active" : "")}
        aria-current={on ? "page" : undefined} aria-label={t(label)} data-tip={t(label)}>
        <span className="rail-ico-wrap">
          <Icon className="rail-ico" aria-hidden="true" />
          {badge > 0 && <span className="rail-badge" aria-hidden="true">{badge}</span>}
        </span>
        <span className="rail-label">{t(short)}</span>
      </Link>
    );
  };

  return (
    <nav className="rail" aria-label={t("navMain")}>
      <Link href="/" className="rail-brand" aria-label="Clipper" data-tip="Clipper">
        {/* eslint-disable-next-line @next/next/no-img-element -- ekspor statis, tanpa optimasi gambar */}
        <img className="rail-logo" src="/logo.png" alt="" width={30} height={22} />
      </Link>
      {TOOLS.filter(allowed).map(item)}
      <div className="rail-sep" aria-hidden="true" />
      {item(HISTORY)}

      <div className="rail-foot">
        {allowed(SETTINGS) && item(SETTINGS)}
        <RailAI />
        <AccountButton />
      </div>
    </nav>
  );
}
