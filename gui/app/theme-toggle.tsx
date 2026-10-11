"use client";

import { useEffect, useState } from "react";
import { Moon, Sun } from "lucide-react";
import { useI18n } from "./i18n";

export type Theme = "light" | "dark";
export const THEME_KEY = "clipper.theme";

// Tombol terang/gelap di bilah atas, di sebelah gerigi setelan.
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

  const flip = () => {
    const next: Theme = theme === "dark" ? "light" : "dark";
    document.documentElement.dataset.theme = next;
    try { localStorage.setItem(THEME_KEY, next); } catch {}
    setTheme(next);
  };

  const dark = theme === "dark";
  return (
    <button className="rail-tool" onClick={flip}
      title={dark ? t("themeLight") : t("themeDark")}
      aria-label={dark ? t("themeLight") : t("themeDark")}>
      {dark ? <Sun className="ico" aria-hidden="true" /> : <Moon className="ico" aria-hidden="true" />}
    </button>
  );
}
