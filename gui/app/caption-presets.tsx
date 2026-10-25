"use client";

// Template gaya caption (subtitle) ala Opus Clip (keputusan pemilik 25 Oktober
// 2026): kisi kotak contoh yang langsung memperlihatkan rupanya — disorot =
// kata berjalan dengan geraknya, klik = gaya itu dipakai. "Preset cepat" = gaya bawaan di sini; "Template saya" = gaya
// simpanan pengguna, lewat penyimpan template yang SAMA dengan Mark
// (useTemplates, kind "caption").

import { useCallback, useEffect, useRef, useState } from "react";
import { ChevronLeft, ChevronRight, X } from "lucide-react";
import { useI18n } from "./i18n";
import { useSticky } from "./persist";
import { useTemplates } from "./mark";
import { toHex } from "./color-panel";

// SubFx = gerak & bayangan preset; nama field = field JSON config.Subtitle.
export type SubFx = { anim: string; shadow: string; stack: boolean; alternate: boolean };
export const NO_FX: SubFx = { anim: "", shadow: "", stack: false, alternate: false };

// CaptionStyle = semua yang diatur satu template (bukan posisi & kecepatan).
// fx boleh tidak ada: template simpanan sebelum 25 Oktober 2026.
export type CaptionStyle = {
  font: string; size: number; color: string; highlight: string;
  outline: number; box: boolean; mode: string; upper: boolean; fx?: SubFx;
};

const fx = (f: Partial<SubFx>): SubFx => ({ ...NO_FX, ...f });

// Preset bawaan, ditiru dari rekaman animasi preset Opus (.ref, Lottie): font
// statis yang dibundel (fetch-fonts.sh), mode, gerak, dan bayangannya. Nama
// sendiri, bukan nama Opus.
export const PRESETS: { id: string; s: CaptionStyle }[] = [
  { id: "wordcut", s: { font: "Inter", size: 96, color: "#ffffff", highlight: "#ffffff", outline: 0, box: false, mode: "word", upper: false, fx: fx({ shadow: "soft" }) } },
  { id: "softword", s: { font: "Figtree", size: 96, color: "#f0e6b5", highlight: "#f0e6b5", outline: 0, box: false, mode: "word", upper: false, fx: fx({ shadow: "soft", anim: "rise" }) } },
  { id: "buildup", s: { font: "League Spartan", size: 90, color: "#ffffff", highlight: "#ffffff", outline: 0, box: false, mode: "build", upper: false, fx: fx({ shadow: "soft" }) } },
  { id: "stack", s: { font: "Poppins Black", size: 104, color: "#ffded9", highlight: "#ffded9", outline: 0, box: false, mode: "build", upper: true, fx: fx({ shadow: "soft", stack: true }) } },
  { id: "punch", s: { font: "Anton", size: 130, color: "#ffffff", highlight: "#ffe600", outline: 0, box: false, mode: "word", upper: true, fx: fx({ shadow: "hard", anim: "pop", alternate: true }) } },
  { id: "elastic", s: { font: "Oswald", size: 100, color: "#ffffff", highlight: "#00e6a1", outline: 8, box: false, mode: "karaoke", upper: true, fx: fx({ shadow: "none", anim: "bounce" }) } },
  { id: "rise", s: { font: "Bebas Neue", size: 124, color: "#ffffff", highlight: "#ffffff", outline: 0, box: false, mode: "build", upper: true, fx: fx({ shadow: "soft", anim: "rise" }) } },
  { id: "marker", s: { font: "Montserrat", size: 90, color: "#ffffff", highlight: "#c9b4ff", outline: 0, box: false, mode: "highlight", upper: true, fx: fx({ shadow: "soft", anim: "pop" }) } },
  { id: "typewriter", s: { font: "JetBrains Mono", size: 80, color: "#ffffff", highlight: "#ffffff", outline: 0, box: false, mode: "build", upper: false, fx: fx({ shadow: "hard" }) } },
  { id: "karaoke", s: { font: "Montserrat", size: 94, color: "#ffffff", highlight: "#ffd400", outline: 5, box: false, mode: "karaoke", upper: true, fx: NO_FX } },
  { id: "box", s: { font: "Montserrat", size: 94, color: "#ffffff", highlight: "#ffd400", outline: 0, box: true, mode: "normal", upper: false, fx: NO_FX } },
  // Ronde kedua (25 Oktober 2026): font display tambahan dari .ref, dipadukan
  // dengan mode & gerak yang sama — tidak ada kemampuan engine baru.
  { id: "hype", s: { font: "Montserrat", size: 110, color: "#ffffff", highlight: "#22e04e", outline: 10, box: false, mode: "karaoke", upper: true, fx: fx({ shadow: "hard", anim: "pop" }) } },
  { id: "comic", s: { font: "Bangers", size: 124, color: "#ffe600", highlight: "#ffffff", outline: 8, box: false, mode: "word", upper: true, fx: fx({ shadow: "hard", anim: "pop", alternate: true }) } },
  { id: "bubble", s: { font: "Titan One", size: 100, color: "#ffffff", highlight: "#ff7a1a", outline: 8, box: false, mode: "karaoke", upper: false, fx: fx({ shadow: "none", anim: "bounce" }) } },
  { id: "sharpie", s: { font: "Permanent Marker", size: 96, color: "#ffffff", highlight: "#ff4fd8", outline: 0, box: false, mode: "karaoke", upper: false, fx: fx({ shadow: "soft", anim: "rise" }) } },
  { id: "arcade", s: { font: "Bungee", size: 84, color: "#00e5ff", highlight: "#ff2bd6", outline: 0, box: false, mode: "karaoke", upper: true, fx: fx({ shadow: "hard" }) } },
  // Press Start 2P selebar 0,73 em per huruf, lebih dari perkiraan 0,6 milik
  // subtitle.maxCharsPerLine: satu kata per baris supaya tidak meluber.
  { id: "pixel", s: { font: "Press Start 2P", size: 64, color: "#ffffff", highlight: "#ffffff", outline: 0, box: false, mode: "build", upper: true, fx: fx({ shadow: "hard", stack: true }) } },
  { id: "script", s: { font: "Pacifico", size: 92, color: "#ffffff", highlight: "#ffffff", outline: 0, box: false, mode: "normal", upper: false, fx: fx({ shadow: "soft", anim: "rise" }) } },
  { id: "newsflash", s: { font: "Archivo Black", size: 92, color: "#ffffff", highlight: "#ff3b30", outline: 0, box: false, mode: "highlight", upper: true, fx: fx({ shadow: "soft", anim: "pop" }) } },
  { id: "notebook", s: { font: "Caveat", size: 124, color: "#fff8e7", highlight: "#fff8e7", outline: 0, box: false, mode: "build", upper: false, fx: fx({ shadow: "soft", anim: "rise" }) } },
  { id: "redtag", s: { font: "Poppins Black", size: 96, color: "#ffffff", highlight: "#fe2c55", outline: 0, box: false, mode: "highlight", upper: true, fx: fx({ shadow: "soft", anim: "pop" }) } },
  { id: "gold", s: { font: "Oswald", size: 110, color: "#ffffff", highlight: "#ffc83d", outline: 6, box: false, mode: "karaoke", upper: true, fx: fx({ shadow: "hard", anim: "bounce" }) } },
  { id: "cinema", s: { font: "Bebas Neue", size: 116, color: "#f5f5f5", highlight: "#f5f5f5", outline: 0, box: false, mode: "word", upper: true, fx: fx({ shadow: "none", anim: "rise" }) } },
  { id: "tower", s: { font: "League Spartan", size: 104, color: "#ffffff", highlight: "#7cf7ff", outline: 6, box: false, mode: "karaoke", upper: true, fx: fx({ shadow: "none", anim: "bounce", stack: true }) } },
  { id: "minimal", s: { font: "Inter", size: 64, color: "#ffffff", highlight: "#ffffff", outline: 0, box: false, mode: "normal", upper: false, fx: fx({ shadow: "soft" }) } },
];

const sameFx = (a: SubFx = NO_FX, b: SubFx = NO_FX) =>
  a.anim === b.anim && a.shadow === b.shadow && a.stack === b.stack && a.alternate === b.alternate;
const same = (a: CaptionStyle, b: CaptionStyle) =>
  a.font === b.font && a.size === b.size && toHex(a.color) === toHex(b.color) && toHex(a.highlight) === toHex(b.highlight)
  && a.outline === b.outline && a.box === b.box && a.mode === b.mode && a.upper === b.upper && sameFx(a.fx, b.fx);

// captionShadow = garis tepi + bayangan dalam em (satuan ukuran huruf), dengan
// angka yang sama dengan subtitle.assHeader: soft = size/20 pada 22% pekat,
// hard = size/16 pekat, bawaan = 2 pada 44%. Satuan em membuatnya benar di
// kotak contoh maupun di pratinjau 9:16, berapa pun skalanya.
export function captionShadow(s: Pick<CaptionStyle, "size" | "outline" | "box" | "fx">) {
  if (s.box) return "none";
  const em = (v: number) => `${(v / Math.max(1, s.size)).toFixed(3)}em`;
  const out: string[] = [];
  if (s.outline > 0) {
    const o = em(s.outline);
    out.push(`-${o} -${o} 0 #000`, `${o} -${o} 0 #000`, `-${o} ${o} 0 #000`, `${o} ${o} 0 #000`);
  }
  const sh = s.fx?.shadow;
  const [d, a] = sh === "soft" ? [Math.max(2, s.size / 20), 0.22] : sh === "hard" ? [Math.max(3, s.size / 16), 1] : sh === "none" ? [0, 0] : [2, 0.44];
  if (d > 0) out.push(`${em(d)} ${em(d)} 0 rgba(0,0,0,${a})`);
  return out.join(",") || "none";
}

// CaptionWords = baris contoh dengan aturan mode yang SAMA dengan engine
// (subtitle.WriteASS): word = satu kata (ganjil berwarna sorot bila
// alternate), build = kata sesudah yang aktif tak terlihat tapi memesan
// tempat, karaoke = kata aktif berwarna, highlight = di atas blok warna;
// stack = satu kata per baris, tiga kata. active kosong = kata terakhir.
// Kata aktif memutar ulang geraknya (cp-a-*) tiap kali active berganti.
export function CaptionWords({ s, lines, active }: { s: CaptionStyle; lines: string[][]; active?: number }) {
  const f = s.fx ?? NO_FX;
  const rows = f.stack ? lines.flat().slice(0, 3).map((w) => [w]) : lines;
  const n = rows.flat().length;
  const a = active === undefined ? n - 1 : active % n;
  const hl = toHex(s.highlight);
  const anim = f.anim ? "cp-a cp-a-" + f.anim : undefined;
  if (s.mode === "word") {
    const w = rows.flat()[a];
    return <div><span key={a} className={anim} style={f.alternate && a % 2 ? { color: hl } : undefined}>{w}</span></div>;
  }
  let i = 0;
  return <>{rows.map((ln, r) => (
    <div key={r}>{ln.map((w, j) => {
      const idx = i++;
      const on = idx === a;
      const style: React.CSSProperties | undefined = on && s.mode === "karaoke" ? { color: hl }
        : on && s.mode === "highlight" ? { background: hl, color: "#15181D", textShadow: "none", padding: "0 .15em", borderRadius: ".12em" }
        : s.mode === "build" && idx > a ? { visibility: "hidden" } : undefined;
      return <span key={idx + (on ? "a" : "")}>{j > 0 && " "}<span className={on ? anim : undefined} style={style}>{w}</span></span>;
    })}</div>
  ))}</>;
}

// Sample = kotak contoh satu preset; tick = kata aktif saat disorot tetikus
// (kosong = diam di kata terakhir).
function Sample({ s, tick }: { s: CaptionStyle; tick?: number }) {
  const { t } = useI18n();
  const words = [t("cpSample1").split(" "), t("cpSample2").split(" ")];
  return (
    <span className="cp-sample" style={{
      fontFamily: `"${s.font}", sans-serif`, fontWeight: 700, fontSize: Math.round((s.size / 94) * 13), color: toHex(s.color),
      textShadow: captionShadow(s), textTransform: s.upper ? "uppercase" : "none",
      background: s.box ? "rgba(0,0,0,.6)" : "transparent", padding: s.box ? "2px 8px" : 0, borderRadius: s.box ? 5 : 0,
    }}>
      <CaptionWords s={s} lines={words} active={tick} />
    </span>
  );
}

// useHoverTick = penghitung kata selama sebuah kotak disorot (satu sekaligus).
function useHoverTick() {
  const [hover, setHover] = useState<string | null>(null);
  const [tick, setTick] = useState(0);
  useEffect(() => {
    if (!hover) return;
    setTick(0);
    const id = setInterval(() => setTick((v) => v + 1), 420);
    return () => clearInterval(id);
  }, [hover]);
  return { tickOf: (id: string) => (hover === id ? tick : undefined),
    on: (id: string) => ({ onMouseEnter: () => setHover(id), onMouseLeave: () => setHover((h) => (h === id ? null : h)), onFocus: () => setHover(id), onBlur: () => setHover(null) }) };
}

// Rail = satu baris kotak yang digulir ke samping, dengan panah kiri/kanan
// (mati di ujungnya). Tinggi tetap satu baris berapa pun jumlah presetnya.
function Rail({ children }: { children: React.ReactNode }) {
  const { t } = useI18n();
  const box = useRef<HTMLDivElement>(null);
  const [end, setEnd] = useState({ l: true, r: true });
  const check = useCallback(() => {
    const e = box.current;
    if (e) setEnd({ l: e.scrollLeft <= 1, r: e.scrollLeft + e.clientWidth >= e.scrollWidth - 1 });
  }, []);
  useEffect(() => {
    check();
    const e = box.current;
    if (!e) return;
    const ro = new ResizeObserver(check);
    ro.observe(e);
    return () => ro.disconnect();
  }, [check, children]);
  const go = (dir: number) => box.current?.scrollBy({ left: dir * box.current.clientWidth * 0.8, behavior: "smooth" });
  return (
    <div className="cp-rail">
      <button type="button" className="ghost tiny icon-only cp-arrow" aria-label={t("cpPrev")} title={t("cpPrev")} disabled={end.l} onClick={() => go(-1)}>
        <ChevronLeft className="ico" aria-hidden="true" /></button>
      <div className="cp-grid" ref={box} onScroll={check}>{children}</div>
      <button type="button" className="ghost tiny icon-only cp-arrow" aria-label={t("cpNext")} title={t("cpNext")} disabled={end.r} onClick={() => go(1)}>
        <ChevronRight className="ico" aria-hidden="true" /></button>
    </div>
  );
}

export default function CaptionPresets({ value, onApply, setError }: {
  value: CaptionStyle; onApply: (s: CaptionStyle) => void; setError: (e: string) => void;
}) {
  const { t } = useI18n();
  const [tab, setTab] = useSticky<"quick" | "mine">("clips.captionTab", "quick");
  const tpl = useTemplates(setError, "caption");
  const [name, setName] = useState("");
  const hov = useHoverTick();
  return (
    <div className="cp-presets">
      {tpl.confirmEl}
      <div className="cp-ptabs" role="tablist">
        {([["quick", t("cpQuick")], ["mine", t("cpMine")]] as const).map(([k, label]) => (
          <button key={k} type="button" role="tab" aria-selected={tab === k} className={tab === k ? "on" : ""} onClick={() => setTab(k)}>{label}</button>
        ))}
      </div>
      {tab === "quick" ? (
        <Rail>
          {PRESETS.map((p) => (
            <button key={p.id} type="button" className={"cp-tile" + (same(p.s, value) ? " on" : "")} onClick={() => onApply(p.s)} {...hov.on(p.id)}>
              <span className="cp-face"><Sample s={p.s} tick={hov.tickOf(p.id)} /></span>
              <span className="cp-name" title={t(("cp_" + p.id) as "cp_box")}>{t(("cp_" + p.id) as "cp_box")}</span>
            </button>
          ))}
        </Rail>
      ) : <>
        {tpl.templates.length > 0 ? (
          <Rail>
            {tpl.templates.map((x) => {
              const s = x.data as unknown as CaptionStyle;
              return (
                <div key={x.name} className="cp-tile-wrap">
                  <button type="button" className={"cp-tile" + (same(s, value) ? " on" : "")} onClick={() => onApply(s)} {...hov.on("mine:" + x.name)}>
                    <span className="cp-face"><Sample s={s} tick={hov.tickOf("mine:" + x.name)} /></span>
                    <span className="cp-name" title={x.name}>{x.name}</span>
                  </button>
                  <button type="button" className="ghost tiny icon-only cp-del" aria-label={t("wmTplDelete")} title={t("wmTplDelete")}
                    onClick={() => tpl.remove(x.name)}><X className="ico" aria-hidden="true" /></button>
                </div>
              );
            })}
          </Rail>
        ) : <p className="meta">{t("cpMineEmpty")}</p>}
        {/* Simpan gaya yang sedang dipakai sebagai template baru (nama sama = ditimpa). */}
        <div className="cp-save">
          <input value={name} placeholder={t("cpSaveName")} aria-label={t("cpSaveName")} onChange={(e) => setName(e.target.value)} maxLength={60} />
          <button type="button" className="primary" disabled={!name.trim()}
            onClick={async () => { if (await tpl.save(name.trim(), value as unknown as Record<string, unknown>)) setName(""); }}>{t("save")}</button>
        </div>
      </>}
    </div>
  );
}
