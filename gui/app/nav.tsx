"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Scissors, Newspaper, PenLine, Captions, Stamp, History, Settings } from "lucide-react";
import { useI18n, type MessageKey } from "./i18n";
import ThemeToggle from "./theme-toggle";
import AccountButton from "./account";
import { RailJob, RailAI, useRunning } from "./rail-status";
import { useMe } from "./engine";

// Navigasi kiri — DESIGN-Navigasi (bagian kedua DEsign-clipper-lanjutan.md).
//
// Dua kelompok: BUAT (alat) dan KELOLA (hasil + setelan). Satu pola lebar per
// lebar jendela, tidak ada di antaranya: ≥1100px sidebar berlabel 220px
// (varian B), di bawahnya rail ikon 72px dengan tooltip (varian A). Bentuknya
// diatur CSS saja — komponen ini merender keduanya sama.

// perm = izin Sarth Admin yang dibutuhkan halaman itu (engine/internal/api/access.go).
type Item = { href: string; label: MessageKey; Icon: typeof Scissors; perm?: string };
const CREATE: Item[] = [
  { href: "/", label: "navClips", Icon: Scissors, perm: "clip.view" },
  { href: "/news", label: "navNews", Icon: Newspaper, perm: "card.view" },
  { href: "/writer", label: "navWriter", Icon: PenLine, perm: "article.view" },
  { href: "/captions", label: "navCaptions", Icon: Captions, perm: "caption.view" },
  { href: "/watermark", label: "navWatermark", Icon: Stamp, perm: "watermark.view" },
];
const MANAGE: Item[] = [
  { href: "/history", label: "navHistory", Icon: History },
  { href: "/requirements", label: "navSettings", Icon: Settings, perm: "settings.view" },
];

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

  const item = ({ href, label, Icon }: Item) => {
    const on = samePath(path, href);
    const badge = href === "/history" && running.length > 0 ? running.length : 0;
    return (
      <Link key={href} href={href} className={"rail-item" + (on ? " active" : "")}
        aria-current={on ? "page" : undefined} aria-label={t(label)} data-tip={t(label)}>
        <span className="rail-ico-wrap">
          <Icon className="rail-ico" aria-hidden="true" />
          {badge > 0 && <span className="rail-badge" aria-hidden="true">{badge}</span>}
        </span>
        <span className="rail-label">{t(label)}</span>
        {badge > 0 && <span className="rail-count" aria-hidden="true">{badge}</span>}
      </Link>
    );
  };

  return (
    <nav className="rail" aria-label={t("navMain")}>
      <Link href="/" className="rail-brand" aria-label="Clipper">
        <span className="rail-logo" aria-hidden="true">C</span>
        <span className="rail-label">Clipper</span>
      </Link>
      <div className="rail-group">{t("navCreate")}</div>
      {CREATE.filter(allowed).map(item)}
      <div className="rail-group">{t("navManage")}</div>
      {MANAGE.filter(allowed).map(item)}

      <div className="rail-foot">
        <RailJob running={running} />
        <RailAI />
        <div className="rail-account">
          <AccountButton />
          <ThemeToggle />
        </div>
      </div>
    </nav>
  );
}
