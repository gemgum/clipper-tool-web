"use client";

// Pemilih "warna bebas" milik aplikasi sendiri (keputusan pemilik 25 Oktober
// 2026), menggantikan kotak <input type="color"> bawaan browser yang tidak
// ikut tema. Bentuknya: cincin hue dengan bulatan warna di tengah, slider
// Saturation & Lightness, kode HEX/RGB/CMYK/HSL, dan deretan warna terakhir.
// Satu-satunya keluaran tetap #rrggbb — bentuk yang dibaca engine.

import { useEffect, useRef, useState } from "react";
import { Pipette } from "lucide-react";
import { useI18n } from "./i18n";
import { parseColor } from "./color-panel";

type HSL = { h: number; s: number; l: number };

const clamp = (v: number, a: number, b: number) => Math.min(b, Math.max(a, v));

export function hexToRgb(hex: string): [number, number, number] {
  const n = parseInt(hex.slice(1), 16);
  return [n >> 16, (n >> 8) & 255, n & 255];
}

function rgbToHex(r: number, g: number, b: number) {
  return "#" + [r, g, b].map((v) => clamp(Math.round(v), 0, 255).toString(16).padStart(2, "0")).join("");
}

function hexToHsl(hex: string): HSL {
  const [r, g, b] = hexToRgb(hex).map((v) => v / 255);
  const max = Math.max(r, g, b), min = Math.min(r, g, b), d = max - min;
  const l = (max + min) / 2;
  let h = 0, s = 0;
  if (d) {
    s = d / (1 - Math.abs(2 * l - 1));
    h = max === r ? ((g - b) / d) % 6 : max === g ? (b - r) / d + 2 : (r - g) / d + 4;
    h = (h * 60 + 360) % 360;
  }
  return { h: Math.round(h), s: Math.round(s * 100), l: Math.round(l * 100) };
}

function hslToHex({ h, s, l }: HSL) {
  const S = s / 100, L = l / 100;
  const c = (1 - Math.abs(2 * L - 1)) * S, x = c * (1 - Math.abs(((h / 60) % 2) - 1)), m = L - c / 2;
  const [r, g, b] = h < 60 ? [c, x, 0] : h < 120 ? [x, c, 0] : h < 180 ? [0, c, x] : h < 240 ? [0, x, c] : h < 300 ? [x, 0, c] : [c, 0, x];
  return rgbToHex((r + m) * 255, (g + m) * 255, (b + m) * 255);
}

function cmyk(hex: string) {
  const [r, g, b] = hexToRgb(hex).map((v) => v / 255);
  const k = 1 - Math.max(r, g, b);
  if (k >= 1) return [0, 0, 0, 100];
  return [(1 - r - k) / (1 - k), (1 - g - k) / (1 - k), (1 - b - k) / (1 - k), k].map((v) => Math.round(v * 100));
}

// Warna terakhir: kenyamanan per penonton saja (localStorage, boleh hilang).
const RECENT_KEY = "clipper.colors.recent";
const readRecent = (): string[] => {
  try { return JSON.parse(localStorage.getItem(RECENT_KEY) || "[]").filter((c: unknown) => typeof c === "string"); } catch { return []; }
};
const pushRecent = (hex: string) => {
  try { localStorage.setItem(RECENT_KEY, JSON.stringify([hex, ...readRecent().filter((c) => c !== hex)].slice(0, 5))); } catch {}
};

// Code = satu kode warna yang bisa diketik; parse → #rrggbb atau null.
function Code({ value, label, parse, onPick }: { value: string; label: string; parse: (v: string) => string | null; onPick: (hex: string) => void }) {
  const [text, setText] = useState(value);
  const [wrong, setWrong] = useState(false);
  useEffect(() => { setText(value); setWrong(false); }, [value]);
  const commit = () => {
    const c = parse(text.trim());
    setWrong(!c);
    if (c) onPick(c);
  };
  return (
    <input className={"cw-code" + (wrong ? " bad" : "")} value={text} spellCheck={false} aria-label={label} aria-invalid={wrong}
      onChange={(e) => setText(e.target.value)} onBlur={commit} onKeyDown={(e) => { if (e.key === "Enter") commit(); }} />
  );
}

export default function ColorWheel({ value, onChange }: { value: string; onChange: (hex: string) => void }) {
  const { t } = useI18n();
  const [hsl, setHsl] = useState<HSL>(() => hexToHsl(value));
  // exact = kode yang dipilih/diketik apa adanya; HSL bulat tidak selalu bisa
  // kembali ke kode yang sama (10,200,30 → #0AC71D), jadi kode itu yang dipakai
  // sampai roda/slider digeser.
  const [exact, setExact] = useState<string | null>(value);
  const [recent, setRecent] = useState<string[]>([]);
  const emitted = useRef(value);
  const ring = useRef<HTMLDivElement>(null);

  // Nilai dari luar (palet, kode) → ikut. Yang baru saja kita kirim tidak
  // dibaca balik, supaya hue tidak melompat saat saturasi 0.
  useEffect(() => { if (value !== emitted.current) { emitted.current = value; setHsl(hexToHsl(value)); setExact(value); } }, [value]);
  useEffect(() => { setRecent(readRecent()); }, []);
  // Ditutup = warna terakhir masuk deretan "terakhir".
  const last = useRef(value);
  last.current = value;
  useEffect(() => () => pushRecent(last.current), []);

  const hex = exact ?? hslToHex(hsl);
  const set = (next: HSL) => {
    setHsl(next);
    setExact(null);
    const h = hslToHex(next);
    emitted.current = h;
    onChange(h);
  };
  const pick = (h: string) => { emitted.current = h; setHsl(hexToHsl(h)); setExact(h); onChange(h); };

  // Cincin hue: 0° di atas (merah), searah jarum jam — sama dengan conic-gradient.
  const hueAt = (e: React.PointerEvent) => {
    const r = ring.current!.getBoundingClientRect();
    const dx = e.clientX - (r.left + r.width / 2), dy = e.clientY - (r.top + r.height / 2);
    set({ ...hsl, h: Math.round((Math.atan2(dx, -dy) * 180 / Math.PI + 360) % 360) });
  };
  const [r, g, b] = hexToRgb(hex);
  const rad = (hsl.h * Math.PI) / 180;
  const eye = typeof window !== "undefined" && "EyeDropper" in window;

  return (
    <div className="cw">
      <div className="cw-main">
        <div className="cw-ring" ref={ring} role="slider" aria-label="Hue" aria-valuemin={0} aria-valuemax={359} aria-valuenow={hsl.h} tabIndex={0}
          onPointerDown={(e) => { e.currentTarget.setPointerCapture(e.pointerId); hueAt(e); }}
          onPointerMove={(e) => { if (e.buttons) hueAt(e); }}
          onKeyDown={(e) => {
            if (e.key === "ArrowRight" || e.key === "ArrowUp") set({ ...hsl, h: (hsl.h + 2) % 360 });
            if (e.key === "ArrowLeft" || e.key === "ArrowDown") set({ ...hsl, h: (hsl.h + 358) % 360 });
          }}>
          <div className="cw-hole">
            <div className="cw-swatch" style={{ background: hex }} />
          </div>
          <span className="cw-knob" style={{ left: `${50 + 42 * Math.sin(rad)}%`, top: `${50 - 42 * Math.cos(rad)}%`, background: `hsl(${hsl.h} 100% 50%)` }} />
        </div>
        <div className="cw-side">
          <label className="cw-label">{t("cwSaturation")}</label>
          <input type="range" className="cw-range" min={0} max={100} value={hsl.s} aria-label={t("cwSaturation")}
            style={{ background: `linear-gradient(90deg, hsl(${hsl.h} 0% ${hsl.l}%), hsl(${hsl.h} 100% ${hsl.l}%))`, "--thumb": hex } as React.CSSProperties}
            onChange={(e) => set({ ...hsl, s: Number(e.target.value) })} />
          <label className="cw-label">{t("cwLightness")}</label>
          <input type="range" className="cw-range" min={0} max={100} value={hsl.l} aria-label={t("cwLightness")}
            style={{ background: `linear-gradient(90deg, #000, hsl(${hsl.h} ${hsl.s}% 50%), #fff)`, "--thumb": hex } as React.CSSProperties}
            onChange={(e) => set({ ...hsl, l: Number(e.target.value) })} />
          {/* Semua kode bisa diketik (Enter / keluar kolom = dipakai); yang salah ditandai. */}
          <dl className="cw-codes">
            <dt>HEX</dt><dd><Code value={hex.toUpperCase()} label="HEX" parse={(v) => parseColor(v)} onPick={pick} /></dd>
            <dt>RGB</dt><dd><Code value={`${r},${g},${b}`} label="RGB" parse={(v) => parseColor(`rgb(${v})`)} onPick={pick} /></dd>
            <dt>CMYK</dt><dd><Code value={cmyk(hex).join(",")} label="CMYK" parse={(v) => parseColor(`cmyk(${v})`)} onPick={pick} /></dd>
            <dt>HSL</dt>
            <dd className="cw-hsl">
              {(["h", "s", "l"] as const).map((k) => (
                <input key={k} type="number" min={0} max={k === "h" ? 359 : 100} value={hsl[k]} aria-label={`HSL ${k.toUpperCase()}`}
                  onChange={(e) => set({ ...hsl, [k]: clamp(Number(e.target.value) || 0, 0, k === "h" ? 359 : 100) })} />
              ))}
            </dd>
          </dl>
        </div>
      </div>
      <div className="cw-recent" role="group" aria-label={t("cwRecent")}>
        {eye && (
          <button type="button" className="cw-eye" aria-label={t("cwEyedropper")} title={t("cwEyedropper")}
            onClick={async () => {
              try {
                // eslint-disable-next-line @typescript-eslint/no-explicit-any
                const res = await new (window as any).EyeDropper().open();
                if (/^#[0-9a-f]{6}$/i.test(res.sRGBHex)) pick(res.sRGBHex.toLowerCase());
              } catch {}
            }}>
            <Pipette className="ico" aria-hidden="true" />
          </button>
        )}
        {recent.map((c) => (
          <button key={c} type="button" className={"cw-chip" + (c === hex ? " on" : "")} aria-label={c} title={c.toUpperCase()}
            style={{ background: c }} onClick={() => pick(c)} />
        ))}
      </div>
    </div>
  );
}
