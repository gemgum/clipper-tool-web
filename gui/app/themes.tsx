"use client";

import { Wand2 } from "lucide-react";
import { useI18n } from "./i18n";
import ColorWheel from "./color-wheel";
import Popover from "./popover";

// Tampilan jadi kartu berita (DESIGN-NEWSCARD §5.2 + empat tambahan pemilik
// 9 Oktober 2026). Warnanya ditetapkan engine (card.themeTable); di sini hanya
// contoh warnanya untuk kotak pilihan. Dipakai halaman News dan Watermark.
export const THEMES = [
  { id: "dark", key: "ncThemeDark", swatch: "#15181D", style: "dark" },
  { id: "light", key: "ncThemeLight", swatch: "#FFFFFF", style: "light" },
  { id: "photo", key: "ncThemePhoto", swatch: "#3C2A1E", style: "dark" },
  { id: "paper", key: "ncThemePaper", swatch: "#F2EBDD", style: "light" },
  { id: "navy", key: "ncThemeNavy", swatch: "#0F2747", style: "dark" },
  { id: "red", key: "ncThemeRed", swatch: "#7F1714", style: "dark" },
  { id: "forest", key: "ncThemeForest", swatch: "#163A2D", style: "dark" },
  { id: "plum", key: "ncThemePlum", swatch: "#2E1F4A", style: "dark" },
] as const;

// ThemePicker = kisi 4x2: tujuh contoh warna + satu warna bebas (nama hanya di title & aria-label) yang
// melebar penuh, plus tombol "Auto from image" opsional di kanan setinggi dua
// baris warna. SATU pemilih untuk News dan Watermark.
export default function ThemePicker({ value, onChange, label, auto }: {
  value: string; onChange: (id: string) => void; label: string;
  // auto = engine mencari warna yang menyambung gambar; value "auto" = terpilih.
  auto?: { onClick: () => void; disabled?: boolean };
}) {
  const { t } = useI18n();
  const custom = /^#[0-9a-f]{6}$/i.test(value);
  return (
    <div className={"theme-pick" + (auto ? " with-auto" : "")}>
      <div className="theme-grid" role="group" aria-label={label}>
        {/* Tujuh tema + kotak terakhir = warna bebas (roda warna); nilainya #rrggbb. */}
        {THEMES.filter((x) => x.id !== "plum").map((x) => (
          <button key={x.id} type="button" className={"theme-tile" + (value === x.id ? " on" : "")}
            aria-pressed={value === x.id} aria-label={t(x.key)} title={t(x.key)} onClick={() => onChange(x.id)}>
            <span className="theme-swatch" style={{ background: x.swatch }} />
          </button>
        ))}
        <Popover width={460} ariaLabel={t("wmCustomColor")} buttonClass={"theme-tile theme-custom" + (custom ? " on" : "")}
          label={<span className="theme-swatch" style={custom ? { background: value } : undefined} />}>
          {() => <ColorWheel value={custom ? value.toLowerCase() : "#2e1f4a"} onChange={onChange} />}
        </Popover>
      </div>
      {auto && (
        <button type="button" className={"ghost theme-auto" + (value === "auto" ? " active" : "")}
          aria-pressed={value === "auto"} disabled={auto.disabled} onClick={auto.onClick}>
          <Wand2 className="ico" aria-hidden="true" /><span>{t("wmLookAuto")}</span></button>
      )}
    </div>
  );
}
