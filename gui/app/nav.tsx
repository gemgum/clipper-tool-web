"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Scissors, Newspaper, PenLine, Captions, Droplet, History, Settings, PanelLeft } from "lucide-react";
import { useI18n, type MessageKey } from "./i18n";
import { RailAI, useRunning } from "./rail-status";
import { useMe } from "./engine";
import { ScrollKeep, useSticky } from "./persist";

// Navigasi kiri (10 November 2026, pola sidebar "Claude Phone"): TERTUTUP =
// kolom ikon 72 px dengan tooltip; logo di atas berganti jadi tombol buka saat
// mouse ada di sidebar. TERBUKA = 216 px: logo + nama + tombol tutup, judul kelompok, baris
// ikon + nama alat. Pilihan buka/tutup diingat browser (useSticky). Lebar
// kolom halaman mengikuti lewat CSS (.app:has(.rail.open)), tanpa JS.

// perm = izin Sarth Admin yang dibutuhkan halaman itu (engine/internal/api/access.go).
// kind = warna penanda alat (.k-<kind> di globals.css), sama dengan lencana
// Riwayat dan ikon kepala halaman. Label rail = nama alat yang SAMA dengan
// tooltip, judul halaman, dan saringan Riwayat (rancangan 10 November 2026).
type Item = { href: string; label: MessageKey; Icon: typeof Scissors; perm?: string; kind?: ToolKind };
export type ToolKind = "clips" | "cards" | "writer" | "captions" | "watermark";
const TOOLS: Item[] = [
  { href: "/", label: "navClips", Icon: Scissors, perm: "clip.view", kind: "clips" },
  { href: "/news", label: "navNews", Icon: Newspaper, perm: "card.view", kind: "cards" },
  { href: "/writer", label: "navWriter", Icon: PenLine, perm: "article.view", kind: "writer" },
  { href: "/captions", label: "navCaptions", Icon: Captions, perm: "caption.view", kind: "captions" },
  { href: "/watermark", label: "navWatermark", Icon: Droplet, perm: "watermark.view", kind: "watermark" },
];
// Nama + ikon tiap alat untuk kepala halaman dan Riwayat — satu sumber dengan rail.
export const TOOL = Object.fromEntries(TOOLS.map((i) => [i.kind, i])) as Record<ToolKind, Item>;
const HISTORY: Item = { href: "/history", label: "navHistory", Icon: History };
const SETTINGS: Item = { href: "/requirements", label: "navSettings", Icon: Settings, perm: "settings.view" };

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
  const [open, setOpen] = useSticky("nav.open", false);
  const toggle = t(open ? "navCollapse" : "navExpand");

  const item = ({ href, label, Icon, kind }: Item) => {
    const on = samePath(path, href);
    const badge = href === "/history" && running.length > 0 ? running.length : 0;
    return (
      <Link key={href} href={href} className={"rail-item" + (kind ? " k-" + kind : "") + (on ? " active" : "")}
        aria-current={on ? "page" : undefined} aria-label={t(label)} data-tip={open ? undefined : t(label)}>
        <span className="rail-ico-wrap">
          <Icon className="rail-ico" aria-hidden="true" />
          {badge > 0 && <span className="rail-badge" aria-hidden="true">{badge}</span>}
        </span>
        <span className="rail-label">{t(label)}</span>
      </Link>
    );
  };

  return (
    <nav className={"rail" + (open ? " open" : "")} aria-label={t("navMain")}>
      {/* Posisi gulir tiap halaman diingat browser (persist.ts). */}
      <ScrollKeep />
      <div className="rail-head">
        {/* Tertutup: logo = tombol buka (ikon panel muncul saat mouse di sidebar).
            Terbuka: logo + nama, tombol tutup di kanan. */}
        <button type="button" className="rail-brand" onClick={() => !open && setOpen(true)} tabIndex={open ? -1 : 0}
          aria-label={open ? "Clipper" : toggle} data-tip={open ? undefined : toggle}>
          {/* eslint-disable-next-line @next/next/no-img-element -- ekspor statis, tanpa optimasi gambar */}
          <img className="rail-logo" src="/logo.png" alt="" width={30} height={22} />
          <PanelLeft className="rail-brand-ico" aria-hidden="true" />
        </button>
        <span className="rail-name">Clipper</span>
        <button type="button" className="rail-toggle" onClick={() => setOpen(false)} aria-label={toggle} title={toggle}>
          <PanelLeft className="ico" aria-hidden="true" />
        </button>
      </div>
      <p className="rail-group">{t("navTools")}</p>
      {TOOLS.filter(allowed).map(item)}
      <div className="rail-sep" aria-hidden="true" />
      {item(HISTORY)}

      <div className="rail-foot">
        {allowed(SETTINGS) && item(SETTINGS)}
        <RailAI />
      </div>
    </nav>
  );
}
