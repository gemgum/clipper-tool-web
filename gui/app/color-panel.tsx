"use client";

// SATU pemilih warna untuk seluruh aplikasi: panel warna halaman klip (palet
// bulat + pilihan sasaran Text / Highlight / Box / Outline) ditambah warna bebas
// dan kolom kode warna dari halaman watermark. Dulu ada empat pemilih dan tiga
// peta nama warna yang berbeda — tidak ada yang sama dengan engine.

import { useEffect, useState } from "react";
import ColorWheel from "./color-wheel";
import Stepper from "./stepper";
import { Wand2 } from "lucide-react";
import Popover from "./popover";
import { useI18n } from "./i18n";
import type { MessageKey } from "./i18n";

// Nama warna yang dikenal engine (subtitle.assColor) dan arti PERSISNYA di sana.
// Pratinjau WAJIB memakai angka yang sama dengan hasil render.
const NAMED: Record<string, string> = { white: "#ffffff", yellow: "#ffff00", black: "#000000", red: "#b0121b" };
const HEX = /^#[0-9a-f]{6}$/i;

// toHex: nama engine atau #rrggbb → #rrggbb huruf kecil. Selain itu putih,
// sama dengan engine.
export const toHex = (c: string | undefined) => {
  const v = (c || "").toLowerCase();
  return NAMED[v] || (HEX.test(v) ? v : "#ffffff");
};

// Gelap = teks di atasnya harus terang. Ambang sama dengan subtitle.creditColor.
export const isDark = (c: string | undefined) => {
  const n = parseInt(toHex(c).slice(1), 16);
  return 0.299 * (n >> 16) + 0.587 * ((n >> 8) & 255) + 0.114 * (n & 255) < 140;
};

// Palet bersama (DESIGN.md §7.1). Engine membaca #rrggbb untuk semuanya.
export const PALETTE: { hex: string; k: MessageKey }[] = [
  { hex: "#ffffff", k: "colorWhite" }, { hex: "#ffd400", k: "colorYellow" }, { hex: "#f2874b", k: "colorOrange" },
  { hex: "#e5484d", k: "colorRed" }, { hex: "#ff7ab6", k: "colorPink" }, { hex: "#9b7bff", k: "colorPurple" },
  { hex: "#4f8bff", k: "colorBlue" }, { hex: "#3ad1d9", k: "colorCyan" }, { hex: "#38c976", k: "colorGreen" },
  { hex: "#111111", k: "colorBlack" },
];

// parseColor: kode warna apa pun → #rrggbb, satu-satunya bentuk yang dibaca
// engine. CMYK dihitung sendiri (bukan warna CSS); HEX tanpa #, rgb(), hsl(),
// hwb(), dan nama warna diserahkan ke kanvas browser. null = bukan warna.
export function parseColor(input: string): string | null {
  let s = input.trim();
  const m = s.match(/^cmyk\(\s*([\d.]+)%?\s*,\s*([\d.]+)%?\s*,\s*([\d.]+)%?\s*,\s*([\d.]+)%?\s*\)$/i);
  if (m) {
    const [c, mg, y, k] = m.slice(1).map((v) => Math.min(100, Number(v)) / 100);
    const ch = (v: number) => Math.round(255 * (1 - v) * (1 - k)).toString(16).padStart(2, "0");
    return "#" + ch(c) + ch(mg) + ch(y);
  }
  if (/^[0-9a-f]{3}([0-9a-f]{3})?$/i.test(s)) s = "#" + s;
  if (typeof CSS === "undefined" || !CSS.supports("color", s)) return null;
  const ctx = document.createElement("canvas").getContext("2d");
  if (!ctx) return null;
  ctx.fillStyle = "#000";
  ctx.fillStyle = s;
  return HEX.test(String(ctx.fillStyle)) ? String(ctx.fillStyle) : null;
}

// Kolom kode warna: diketik bebas, dipakai saat Enter atau keluar kolom.
// SATU komponen untuk panel warna dan roda warna (HEX/RGB/CMYK); parse =
// pembaca kodenya (bawaan parseColor), className = rupa kolomnya.
export function ColorCode({ value, onPick, label, bad, parse = parseColor, className = "cp-code" }: {
  value: string; onPick: (c: string) => void; label: string; bad?: string; parse?: (v: string) => string | null; className?: string;
}) {
  const [text, setText] = useState(value.toUpperCase());
  const [wrong, setWrong] = useState(false);
  useEffect(() => { setText(value.toUpperCase()); setWrong(false); }, [value]);
  const commit = () => {
    const c = parse(text.trim());
    setWrong(!c);
    if (c) onPick(c);
  };
  return (
    <input className={className + (wrong ? " bad" : "")} value={text} spellCheck={false} aria-label={label}
      title={wrong ? bad || label : label} aria-invalid={wrong}
      onChange={(e) => setText(e.target.value)} onBlur={commit} onKeyDown={(e) => { if (e.key === "Enter") commit(); }} />
  );
}

// Satu sasaran warna: nama di tombolnya, nilai sekarang, dan penyetelnya.
// off = sasaran yang belum berlaku: tombolnya tetap ada tapi mati (jumlah tombol
// tidak pernah berubah, jadi panelnya tidak pernah berubah tinggi).
// auto = tombol tongkat ajaib (warna otomatis) menggantikan satu bulatan palet;
// width = tebal garis (Outline) sebaris dengan kolom kode warna.
export type ColorTarget = {
  key: string; name: string; value: string; set: (hex: string) => void; off?: boolean;
  auto?: { onClick: () => void; title: string };
  width?: { value: number; onChange: (v: number) => void };
};

// ColorPanel = isi panel kanan <Split side={…}>. Sasaran yang hilang (mis. Box saat kotak
// judul mati) jatuh ke sasaran pertama.
export default function ColorPanel({ targets }: { targets: ColorTarget[] }) {
  const { t } = useI18n();
  const [sel, setSel] = useState(targets[0].key);
  const cur = targets.find((x) => x.key === sel && !x.off) || targets[0];
  const hex = toHex(cur.value);
  return (
    <>
      <span className="mc-r-l">{t("color")}</span>
      <div className="segmented cp-targets" role="group" aria-label={t("color")}>
        {targets.map((x) => (
          <button key={x.key} type="button" className={x.key === cur.key ? "on" : ""} aria-pressed={x.key === cur.key} disabled={x.off} onClick={() => setSel(x.key)}>
            <span className="mc-cdot" style={{ background: toHex(x.value) }} />{x.name}
          </button>
        ))}
      </div>
      <div className="mc-cdots" role="group" aria-label={cur.name}>
        {/* Sasaran bertongkat ajaib: merah muda keluar dari palet, tempatnya
            dipakai tombol warna otomatis — jumlah bulatan tetap sepuluh. */}
        {(cur.auto ? PALETTE.filter((c) => c.k !== "colorPink") : PALETTE).map((c) => {
          const on = hex === c.hex;
          return <button key={c.hex} type="button" className={"mc-cswatch" + (on ? " on" : "")} aria-pressed={on}
            aria-label={t(c.k)} title={t(c.k)} onClick={() => cur.set(c.hex)}>
            <span style={{ background: c.hex }} />
          </button>;
        })}
        {cur.auto && (
          <button type="button" className="mc-cswatch cp-wand" aria-label={cur.auto.title} title={cur.auto.title} onClick={cur.auto.onClick}>
            <span><Wand2 className="ico" aria-hidden="true" /></span>
          </button>
        )}
      </div>
      {/* Warna di luar palet: roda warna milik aplikasi (color-wheel.tsx) ATAU
          kode (HEX, rgb, hsl, cmyk, nama). */}
      <div className="cp-custom">
        <Popover width={460} ariaLabel={t("wmCustomColor")} buttonClass={"cp-any" + (PALETTE.some((c) => c.hex === hex) ? "" : " on")}
          label={<span className="cp-any-ring" aria-hidden="true" />}>
          {() => <ColorWheel value={hex} onChange={cur.set} />}
        </Popover>
        <ColorCode value={hex} onPick={cur.set} label={`${cur.name}: ${t("wmColorCodeTip")}`} bad={t("wmColorCodeBad")} />
        {cur.width && <Stepper value={cur.width.value} onChange={cur.width.onChange} min={0} max={12} step={1} suffix="px" />}
      </div>
    </>
  );
}
