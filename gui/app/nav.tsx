"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Scissors, Newspaper, PenLine, Captions, Stamp, History, Settings } from "lucide-react";
import { useI18n, type MessageKey } from "./i18n";
import ThemeToggle from "./theme-toggle";
import AccountButton from "./account";
import { RailJob, RailAI, useRunning } from "./rail-status";

// Navigasi kiri — DESIGN-Navigasi (bagian kedua DEsign-clipper-lanjutan.md).
//
// Dua kelompok: BUAT (alat) dan KELOLA (hasil + setelan). Satu pola lebar per
// lebar jendela, tidak ada di antaranya: ≥1100px sidebar berlabel 220px
// (varian B), di bawahnya rail ikon 72px dengan tooltip (varian A). Bentuknya
// diatur CSS saja — komponen ini merender keduanya sama.

type Item = { href: string; label: MessageKey; Icon: typeof Scissors };
const CREATE: Item[] = [
  { href: "/", label: "navClips", Icon: Scissors },
  { href: "/news", label: "navNews", Icon: Newspaper },
  { href: "/writer", label: "navWriter", Icon: PenLine },
  { href: "/captions", label: "navCaptions", Icon: Captions },
  { href: "/watermark", label: "navWatermark", Icon: Stamp },
];
const MANAGE: Item[] = [
  { href: "/history", label: "navHistory", Icon: History },
  { href: "/requirements", label: "navSettings", Icon: Settings },
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
      {CREATE.map(item)}
      <div className="rail-group">{t("navManage")}</div>
      {MANAGE.map(item)}

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
