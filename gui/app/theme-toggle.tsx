"use client";

import { useEffect, useState } from "react";
import { Moon, Sun } from "lucide-react";
import { useI18n } from "./i18n";

export type Theme = "light" | "dark";
export const THEME_KEY = "clipper.theme";

// Pilihan terang/gelap di popup akun.
//
// Temanya dipasang sebagai atribut `data-theme` di <html>, dan SELURUH
// perbedaannya cuma nilai token warna di globals.css — tidak ada satu pun
// komponen yang perlu tahu tema mana yang sedang berlaku. Itu memang maksud
// struktur token yang disiapkan sejak awal (notes/29).
//
// Nilai awalnya dibaca skrip kecil di layout.tsx SEBELUM halaman digambar;
// kalau dibaca di sini saja, halaman sempat berkedip putih dulu tiap kali
// dibuka dalam tema gelap.
export default function ThemeToggle() {
  const { t } = useI18n();
  const [theme, setTheme] = useState<Theme>("light");

  useEffect(() => {
    const now = (document.documentElement.dataset.theme as Theme) || "light";
    setTheme(now);
  }, []);

  const pick = (next: Theme) => {
    document.documentElement.dataset.theme = next;
    try { localStorage.setItem(THEME_KEY, next); } catch {}
    setTheme(next);
  };

  // Dua tombol berlabel, bukan satu ikon: pilihan tema harus terbaca tanpa ditebak.
  return (
    <div className="lang-switch" role="group" aria-label={t("navTheme")}>
      {(["light", "dark"] as Theme[]).map((v) => (
        <button key={v} type="button" className={v === theme ? "active" : ""} aria-pressed={v === theme} onClick={() => pick(v)}>
          {v === "dark" ? <Moon className="ico" aria-hidden="true" /> : <Sun className="ico" aria-hidden="true" />}
          {t(v === "dark" ? "themeDark" : "themeLight")}
        </button>
      ))}
    </div>
  );
}
