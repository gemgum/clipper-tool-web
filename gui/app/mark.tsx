"use client";

// Mark = logo + judul yang dibakar ke video/foto (halaman Mark) DAN ke setiap
// kartu News. SATU sumber untuk keduanya: state & turunannya (useMark), daftar
// template bersama (useTemplates), panel setelannya (MarkLogoGroup,
// MarkHeadline, MarkTemplates), dan lapisan pratinjaunya (MarkLayers). Engine
// juga satu jalur: watermark.BurnTo (notes/41, notes/44).

import { useCallback, useEffect, useRef, useState } from "react";
import { ImageIcon, Lock, Magnet, Plus, Star, X } from "lucide-react";
import { eng, post, upload } from "./engine";
import { useI18n } from "./i18n";
import { DEFAULT_WATERMARK, frameSize, headlineAnchor, headlineBox, outlineColor, watermarkToAPI, wrapHeadline } from "./watermark-model";
import type { Format, NewsLayout, Watermark } from "./watermark-model";
import { useConfirm } from "./confirm";
import { useLayerDrag } from "./drag";
import type { Font } from "./preview-panel";
import { AlignPicker, Segmented, Toggle } from "./clip-steps";
import { Group, Pad, Row, Split } from "./tab-card";
import ColorPanel, { isDark, toHex, type ColorTarget } from "./color-panel";
import type { MessageKey } from "./i18n";
import { baseName } from "./file-list";
import Stepper from "./stepper";
import { useSticky } from "./persist";
import ThemePicker from "./themes";
import { hexToRgb } from "./color-wheel";

export const HL_FONT = "Montserrat";
export const SIZES = [48, 64, 88];
export const outlineFor = (size: number) => Math.max(2, Math.round(size / 20));
// Jarak (ruang bingkai) di mana tepi logo yang diseret menempel ke tepi kotak judul.
const STICK = 24;

// Titik tengah logo untuk jangkar 0..8 (baris lalu kolom), menurut tabel §4:
// tepi kiri/kanan 5%, atas 4%, tengah mulai 46%, bawah 16% dari dasar.
function anchorPoint(i: number, wPx: number, hPx: number, fw: number, fh: number) {
  const col = i % 3, row = Math.floor(i / 3);
  const x = col === 0 ? 0.05 * fw + wPx / 2 : col === 1 ? fw / 2 : fw - 0.05 * fw - wPx / 2;
  const y = row === 0 ? 0.04 * fh + hPx / 2 : row === 1 ? 0.46 * fh + hPx / 2 : fh - 0.16 * fh - hPx / 2;
  return { x: Math.round(x), y: Math.round(y) };
}

// Jangkar ke-10: TERKUNCI ke kotak judul berita. dx/dy = tepi kiri & tepi
// bawah logo relatif terhadap tepi kiri-atas kotak, jadi 0,0 = duduk di tepi
// atas kotak, rata kiri — dan menggeser kotaknya membawa logo ikut serta.
export const PIN = 9;
// Kisi posisi logo: 3x3 kotak besar (kiri-atas … kanan-bawah), tiap kotak
// berisi SUB x SUB sel → PAD x PAD titik penempatan.
const SUB = 5;
const PAD = 3 * SUB;
// Jarak minimum logo dari tepi bingkai saat ditaruh lewat kisi (sel paling pinggir).
const PAD_EDGE = 0.04;
// Satu logo. Tingginya tidak disimpan: diturunkan dari lebar dan rasio
// gambarnya, jadi kotaknya selalu pas dengan gambarnya. anchor -1 = posisi
// bebas hasil seretan.
export type LogoItem = { image: string; x: number; y: number; width: number; anchor: number; dx?: number; dy?: number };
// Template = setelan Mark TANPA berkas dan teks judul; default = otomatis
// terpasang di kartu News baru.
export type Template = { name: string; data: Record<string, unknown>; default?: boolean };

// Sasaran warna judul Mark — SATU daftar untuk Mark, News, dan Clips.
// withBox: "Box" selalu tampil, mati selagi kotak judulnya mati.
// auto = warna otomatis (tongkat ajaib) untuk Text & Box, dari useMark.autoColor
// (halaman yang punya gambar dasar). Tebal Outline ikut sasaran Outline.
export const watermarkColors = (w: Watermark, set: (patch: Partial<Watermark>) => void, t: (k: MessageKey) => string, withBox = false,
  auto?: (target: "text" | "box") => void): ColorTarget[] => [
  { key: "text", name: t("mcColText"), value: w.hlColor, set: (c) => set({ hlColor: c }),
    ...(auto ? { auto: { onClick: () => auto("text"), title: t("cpAutoText") } } : {}) },
  ...(withBox ? [{ key: "box", name: t("wmColBox"), value: w.hlBoxColor || "white", set: (c: string) => set({ hlBoxColor: c }), off: !w.hlBox,
    ...(auto ? { auto: { onClick: () => auto("box"), title: t("cpAutoBox") } } : {}) }] : []),
  { key: "outline", name: t("outline"), value: outlineColor(w), set: (c) => set({ hlOutlineColor: c }),
    width: { value: w.hlOutline, onChange: (v: number) => set({ hlOutline: v }) } },
];

// useMark memegang setelan Mark untuk satu bingkai berformat `format`.
// first = berkas/foto dasar (warna Theme "photo"/"auto"); timed = waktu
// tampil berlaku (Mark & Clips); llm = judul boleh dari LLM per klip (Clips):
// pratinjaunya memakai teks contoh.
export function useMark({ format, first, timed = false, grid, llm = false }: { format: Format; first: string; timed?: boolean; grid: number; llm?: boolean }) {
  const { t } = useI18n();
  const { w: fw, h: fh } = frameSize(format);
  const [watermark, setWatermarkState] = useState<Watermark>(DEFAULT_WATERMARK);
  const setWatermark = useCallback((patch: Partial<Watermark>) => {
    setWatermarkState((b) => ({ ...b, ...patch }));
  }, []);
  const [logoOn, setLogoOn] = useState(true);
  const [logos, setLogos] = useState<LogoItem[]>([]);
  const [sel, setSel] = useState(0);
  const cur = logos[sel] as LogoItem | undefined;
  const setLogo = useCallback((i: number, patch: Partial<LogoItem>) =>
    setLogos((ls) => ls.map((l, j) => (j === i ? { ...l, ...patch } : l))), []);
  // Logo yang dipilih lewat pemilih berkas: ganti logo ke-i, atau -1 = tambah.
  const putLogo = useCallback((image: string, i: number) => {
    setLogoOn(true);
    if (i >= 0) { setLogo(i, { image }); setSel(i); return; }
    setLogos((ls) => {
      // Logo pertama ke pojok kanan atas, berikutnya ke kiri atas lalu bebas.
      const next = [...ls, { image, x: 540, y: 300, width: 25, anchor: ls.length === 0 ? 2 : ls.length === 1 ? 0 : 4 }];
      setSel(next.length - 1);
      return next;
    });
  }, [setLogo]);
  // Rasio lebar/tinggi tiap logo, dibaca dari gambarnya sendiri saat dimuat.
  const [dims, setDims] = useState<Record<string, { w: number; h: number }>>({});
  const logoURL = (image: string) => eng(`/api/image?path=${encodeURIComponent(image)}`);
  const onLogoLoad = (image: string) => (e: React.SyntheticEvent<HTMLImageElement>) => {
    const { naturalWidth: w, naturalHeight: h } = e.currentTarget;
    if (w && h && (w !== dims[image]?.w || h !== dims[image]?.h)) setDims((d) => ({ ...d, [image]: { w, h } }));
  };

  // Tata letak kotak judul dari engine (baris, tinggi). Posisi tegaknya
  // dihitung di sini dari DY — rumus yang sama dengan NewsBox — supaya
  // menyeret kotak (dan logo yang terkunci padanya) tidak menunggu jaringan.
  const [layout, setLayout] = useState<NewsLayout | null>(null);
  // Teks yang dipratinjau: judul LLM baru ada per klip, jadi contohnya yang tampil.
  const llmOn = llm && watermark.hlSource === "llm";
  const shownText = llmOn ? t("headlineSample") : watermark.hlText;
  const boxOn = !!watermark.hlBox && !!shownText.trim();
  useEffect(() => {
    if (!boxOn) { setLayout(null); return; }
    const id = setTimeout(() => {
      post<NewsLayout>("/api/watermark/layout", { font: HL_FONT, watermark: watermarkToAPI({ ...watermark, hlText: shownText, format, hlDY: 0, hlCardY: 0 }, HL_FONT) })
        .then(setLayout).catch(() => {});
    }, 120);
    return () => clearTimeout(id);
  }, [boxOn, shownText, watermark.hlSize, watermark.hlCredit, watermark.hlSharp, watermark.hlAlign, watermark.hlStyle, format]); // eslint-disable-line react-hooks/exhaustive-deps
  // Tampilan kartu (Look): tata letak dari engine dengan card_y 0; geserannya
  // dihitung di sini (cardGeom) supaya seret tidak menunggu jaringan.
  const cardStyle = !!watermark.hlBox && watermark.hlStyle === "look";
  const cardOn = boxOn && cardStyle;
  // card_y menggeser BATAS gambar–panel (config.CardPhotoPx: dijepit 20–85%
  // bingkai); judul menempel 12 di bawahnya (subtitle.CardBox: lo 88, tepi 48).
  const cardGeom = (l: NewsLayout, dy: number) => {
    const photo = Math.min(Math.max(l.photo! + dy, Math.trunc(fh / 5)), Math.trunc((fh * 85) / 100));
    return { photo, top: Math.min(Math.max(photo + 12, 88), Math.max(88, fh - l.h - 48)) };
  };
  // Bingkai pratinjau dipotong ulang engine setelah seret BERHENTI, bukan tiap gerak.
  const [frameCardY, setFrameCardY] = useState(watermark.hlCardY || 0);
  useEffect(() => {
    const id = setTimeout(() => setFrameCardY(watermark.hlCardY || 0), 300);
    return () => clearTimeout(id);
  }, [watermark.hlCardY]);
  const nb = boxOn && layout
    ? layout.photo ? { ...layout, ...cardGeom(layout, watermark.hlCardY || 0) } : { ...layout, top: Math.min(Math.max(0, Math.trunc(fh / 2) + watermark.hlDY - Math.trunc(layout.h / 2)), Math.max(0, fh - layout.h)) }
    : null;

  // Ukuran kotak satu logo (ruang bingkai), dari lebar & rasio gambarnya.
  const sizeOf = (l: LogoItem) => {
    const d = dims[l.image];
    const a = d ? d.w / d.h : 1;
    const height = Math.min(100, Math.max(5, Math.round((l.width * fw) / a / fh)));
    return { height, wPx: (fw * l.width) / 100, hPx: (fh * height) / 100 };
  };
  // Kotak & titik tengah logo yang SEBENARNYA dikirim: jangkar dihitung ulang
  // tiap render, jadi ganti format, ukuran, atau teks judul ikut memindahkannya.
  const placeLogo = (l: LogoItem) => {
    const { height, wPx, hPx } = sizeOf(l);
    let { x, y } = l;
    if (l.anchor === PIN && nb) { x = nb.left + (l.dx ?? 0) + wPx / 2; y = nb.top + (l.dy ?? 0) - hPx / 2; }
    else if (l.anchor >= 0) ({ x, y } = anchorPoint(l.anchor === PIN ? 0 : l.anchor, wPx, hPx, fw, fh));
    return { image: l.image, x: Math.round(Math.min(fw, Math.max(1, x))), y: Math.round(Math.min(fh, Math.max(1, y))), width: l.width, height };
  };
  const placed = logoOn ? logos.filter((l) => l.image).map(placeLogo) : [];
  // Sel yang menyala = tempat titik tengah logo terpilih; sel logo lain samar.
  const cellOf = (x: number, y: number) =>
    Math.min(PAD - 1, Math.floor((y / fh) * PAD)) * PAD + Math.min(PAD - 1, Math.floor((x / fw) * PAD));
  const curCell = cur?.image ? (({ x, y }) => cellOf(x, y))(placeLogo(cur)) : -1;
  const otherCells = new Set(logos.filter((l, i) => i !== sel && l.image).map((l) => { const p = placeLogo(l); return cellOf(p.x, p.y); }));
  // Klik sel = titik tengah logo di tengah sel itu, dijaga PAD_EDGE dari tepi bingkai.
  const placeAt = (c: number, r: number) => {
    if (!cur) return;
    const { wPx, hPx } = sizeOf(cur);
    const m = PAD_EDGE * fw;
    const x = Math.min(fw - m - wPx / 2, Math.max(m + wPx / 2, ((c + 0.5) * fw) / PAD));
    const y = Math.min(fh - m - hPx / 2, Math.max(m + hPx / 2, ((r + 0.5) * fh) / PAD));
    setLogo(sel, { anchor: -1, x: Math.round(x), y: Math.round(y) });
  };

  // Yang dipratinjau & dikirim: logo hanya bila "Pakai logo" menyala, judul
  // selalu bebas di bingkai, tepi dihitung otomatis.
  const eff: Watermark = {
    ...watermark,
    format,
    image: placed[0]?.image || "",
    ...(placed[0] ? { x: placed[0].x, y: placed[0].y, width: placed[0].width, height: placed[0].height } : {}),
    logos: placed.slice(1),
    hlFree: true, hlSource: llmOn ? "llm" : "text",
    at: timed ? watermark.at : 0, dur: timed ? watermark.dur : 0,
  };

  // Metrik font judul dari engine: .ass mengartikan ukuran sebagai tinggi kotak
  // font, CSS sebagai em. Tanpa ini pratinjau meleset dari hasil render.
  const [fontScale, setFontScale] = useState(1);
  useEffect(() => {
    fetch(eng("/api/fonts")).then((r) => r.json())
      .then((f: Font[]) => setFontScale(f.find((x) => x.name === HL_FONT)?.scale || 1))
      .catch(() => {});
  }, []);

  const boxRef = useRef<HTMLDivElement | null>(null);
  // Kisi yang sama menempelkan seretan DAN digambar (bila >= 20); Alt = lepas.
  const { dragAt, dragProps } = useLayerDrag(boxRef, grid, fh);
  // Logo yang sedang menempel ke tepi kotak judul (selama diseret) — tandanya.
  const [stuck, setStuck] = useState(-1);
  const logoDrag = (i: number) => {
    const p = placed[i];
    const { wPx, hPx } = sizeOf(logos[i]);
    const props = dragProps(p.x, p.y, (x, y) => {
      let left = x - wPx / 2, bottom = y + hPx / 2, hit = false;
      if (nb) {
        // Menempel: tepi bawah logo ke tepi ATAS kotak, atau tepi atas logo ke
        // tepi BAWAH kotak; tepi kiri/kanan logo ke tepi kiri/kanan kotak.
        const stickTo = (v: number, to: number) => (Math.abs(v - to) < STICK ? ((hit = true), to) : v);
        bottom = stickTo(bottom, nb.top);
        bottom = stickTo(bottom - hPx, nb.top + nb.h) + hPx;
        left = stickTo(left, nb.left);
        left = stickTo(left + wPx, nb.left + nb.w) - wPx;
      }
      setStuck(hit ? i : -1);
      if (logos[i].anchor === PIN && nb) setLogo(i, { dx: Math.round(left - nb.left), dy: Math.round(bottom - nb.top) });
      else setLogo(i, { x: Math.round(left + wPx / 2), y: Math.round(bottom - hPx / 2), anchor: -1 });
    }, fh / 2, fh);
    return { ...props, onPointerDown: (e: React.PointerEvent) => { setSel(i); props.onPointerDown(e); } };
  };
  // Kunci/lepas logo ke kotak judul DI TEMPATNYA SEKARANG: menguncinya tidak
  // memindahkan apa pun, cuma membuatnya ikut bila kotaknya digeser.
  const toggleLock = (i: number) => {
    const l = logos[i], p = placed[i];
    if (!l || !p) return;
    if (l.anchor === PIN || !nb) { setLogo(i, { anchor: -1, x: p.x, y: p.y }); return; }
    const { wPx, hPx } = sizeOf(l);
    setLogo(i, { anchor: PIN, dx: Math.round(p.x - wPx / 2 - nb.left), dy: Math.round(p.y + hPx / 2 - nb.top) });
  };
  const hlBox = headlineBox(eff);
  const hlLines = wrapHeadline(shownText, eff.hlSize, hlBox.w);
  const hlAt = headlineAnchor(hlLines, eff.hlSize, eff.hlDX, eff.hlDY, hlBox);
  // Kotak berita hanya bisa naik-turun: lebarnya selalu selebar bingkai.
  // Kartu: yang diseret batas gambar–panel (panelnya naik-turun); magnet di bawaan.
  const headlineDrag = nb?.photo && layout?.photo ? dragProps(fw / 2, nb.photo, (_x, y) => setWatermark({ hlCardY: cardGeom(layout, Math.round(y) - layout.photo!).photo - layout.photo! }), layout.photo, fh)
    : nb ? dragProps(fw / 2, nb.top + nb.h / 2, (_x, y) => setWatermark({ hlDY: Math.round(y - fh / 2) }), fh / 2, fh)
    : dragProps(hlAt.x, hlAt.y,
      (x, y) => setWatermark({ hlDX: x - hlBox.cx, hlDY: y - hlBox.cy }), hlBox.cy, fh, hlBox.cx);

  // Look kotak judul: engine menghitung warna panel, judul, & lencana (tema
  // "photo" dari rona seluruh bingkai; "auto" menyambung dasar area gambar),
  // lalu warnanya MENGISI panel Colour — tetap bisa diubah di sana.
  const applyLook = (theme: string) =>
    post<{ box: string; text: string; badge: string; on_badge: string }>("/api/watermark/look", { theme, file: first, format, card_y: watermark.hlCardY || 0 })
      .then((c) => setWatermark({ hlLook: theme, hlBoxColor: c.box, hlColor: c.text, hlBadge: c.badge, hlOnBadge: c.on_badge })).catch(() => {});
  // Warna otomatis (tongkat ajaib di panel Colour). Box = warna panel dari
  // gambar seperti kartu (Look "auto"), teks ikut supaya terbaca. Text = warna
  // paling terbaca di atas latarnya: kotak bila menyala, selain itu gambarnya.
  const autoColor = (target: "text" | "box") => {
    const theme = target === "text" && watermark.hlBox ? toHex(watermark.hlBoxColor || "white") : "auto";
    post<{ box: string; text: string }>("/api/watermark/look", { theme, file: first, format, card_y: watermark.hlCardY || 0 })
      .then((c) => setWatermark(target === "box" ? { hlBoxColor: c.box, hlColor: c.text } : { hlColor: c.text })).catch(() => {});
  };
  // Berkas pertama / format berganti → warna dari gambar dihitung ulang.
  useEffect(() => {
    const l = watermark.hlLook;
    if (watermark.hlBox && watermark.hlStyle === "look" && (l === "photo" || l === "auto") && first) applyLook(l);
  }, [first, format]); // eslint-disable-line react-hooks/exhaustive-deps

  // Penerap setelan dari simpanan atau template: logo & gaya judul. keepText =
  // teks judul yang sudah diketik TIDAK ditimpa. Bagian milik halaman (format,
  // waktu tampil, kisi) diterapkan halamannya sendiri.
  const applyMark = (v: Record<string, unknown>, keepText?: string) => {
    if (typeof v.logoOn === "boolean") setLogoOn(v.logoOn);
    if (Array.isArray(v.logos)) setLogos(v.logos as LogoItem[]);
    if (v.watermark && typeof v.watermark === "object") {
      const w = { ...DEFAULT_WATERMARK, ...(v.watermark as Partial<Watermark>), hlSource: "text" as const };
      // Simpanan sebelum ada daftar logo: satu logo di watermark.image.
      if (!Array.isArray(v.logos) && w.image) {
        setLogos([{ image: w.image, x: w.x, y: w.y, width: w.width, anchor: typeof v.anchor === "number" ? v.anchor : -1 }]);
      }
      // Ukuran judul dari versi lama dibulatkan ke tingkat terdekat.
      w.hlSize = SIZES.reduce((a, b) => (Math.abs(b - w.hlSize) < Math.abs(a - w.hlSize) ? b : a));
      if (keepText !== undefined) w.hlText = keepText;
      setWatermarkState(w);
    }
    setSel(0);
  };

  return {
    fw, fh, format, first, grid, watermark, setWatermark, logoOn, setLogoOn, logos, setLogos, setLogo, sel, setSel, cur, putLogo,
    dims, logoURL, onLogoLoad, layout, nb, cardStyle, cardOn, frameCardY, placed, curCell, otherCells, placeAt, eff,
    fontScale, boxRef, dragAt, dragProps, stuck, logoDrag, toggleLock, hlLines, hlAt, headlineDrag, applyLook, applyMark, autoColor,
  };
}
export type Mark = ReturnType<typeof useMark>;

// useTemplates = daftar template Mark di server, SATU untuk halaman Mark & News.
// kind = jenis daftar di engine: "watermark" (Mark & News) | "caption" (gaya subtitle Clips).
export function useTemplates(setError: (e: string) => void, kind: "watermark" | "caption" = "watermark") {
  const { t } = useI18n();
  const [templates, setTemplates] = useState<Template[]>([]);
  const [ask, confirmEl] = useConfirm();
  useEffect(() => {
    fetch(eng(`/api/watermark/templates?kind=${kind}`)).then((r) => r.json())
      .then((d) => setTemplates(d.templates || [])).catch(() => {});
  }, [kind]);
  const call = async (p: Promise<{ templates?: Template[] }>) => {
    try { setTemplates((await p).templates || []); return true; } catch (e) { setError(String(e)); return false; }
  };
  const save = (name: string, data: Record<string, unknown>) => call(post(`/api/watermark/templates?kind=${kind}`, { name, data }));
  const setDefault = (name: string) => call(post(`/api/watermark/templates/${encodeURIComponent(name)}/default?kind=${kind}`, {}));
  const remove = async (name: string) => {
    if (!(await ask(t("wmTplDeleteAsk", { name }), t("wmTplDelete")))) return;
    await call(fetch(eng(`/api/watermark/templates/${encodeURIComponent(name)}?kind=${kind}`), { method: "DELETE" }).then(async (r) => {
      const d = await r.json();
      if (!r.ok) throw new Error(d.error || "failed");
      return d;
    }));
  };
  return { templates, save, remove, setDefault, confirmEl, defaultTemplate: templates.find((x) => x.default) };
}
export type Templates = ReturnType<typeof useTemplates>;

// MarkLogoGroup = tab Logo halaman Mark: daftar logo (ukuran, kunci, ganti) di
// kiri, kisi penempatan di kanan. openPicker = pemilih berkas lokal; di mode
// web logo diunggah lewat input berkas.
export function MarkLogoGroup({ m, web, openPicker, setError }: {
  m: Mark; web: boolean | null; openPicker: (i: number) => void; setError: (e: string) => void;
}) {
  const { t } = useI18n();
  const input = useRef<HTMLInputElement>(null);
  const [target, setTarget] = useState(-1);
  const [upNote, setUpNote] = useState("");
  const choose = (i: number) => { setTarget(i); if (web) input.current?.click(); else openPicker(i); };
  const pick = async (f: File) => {
    try {
      const p = await upload(f, (x) => setUpNote(`${f.name}: ${t("uploadingPct", { pct: Math.round(x * 100) })}`));
      m.putLogo(p, target);
    } catch (e) { setError(`${f.name}: ${String(e)}`); }
    setUpNote("");
  };
  const { logos, sel, cur, fw, fh, nb } = m;
  return <>
    {web && (
      <input ref={input} type="file" accept="image/png,image/*" hidden
        onChange={(e) => { const f = e.target.files?.[0]; e.target.value = ""; if (f) pick(f); }} />
    )}
    {/* Sejajar, bukan atas-bawah (.mc-split, pola panel warna): kiri =
        daftar logo, kanan = penempatan logo terpilih. Panel kanan SELALU
        ada — dimatikan bila belum ada logo, tidak disembunyikan — jadi
        menambah logo tidak menggeser apa pun. */}
    <Group title={t("wmLogoImages")}>
      <Pad><Toggle checked={m.logoOn} onChange={m.setLogoOn} label={t("wmUseLogo")} /></Pad>
      <Split sideOff={!cur || !m.logoOn} side={<>
        <span className="mc-r-l">{t("wmPlaceAt")}</span>
        {/* Kisi posisi = peta kecil bingkai: 3x3 kotak (nama jangkar di
            title), tiap kotak SUB x SUB sel. Tinggi tetap, rasio mengikuti
            format. Klik = titik tengah logo di sel itu (posisi bebas,
            seperti hasil seret). */}
        <div className="wm-pad" role="group" aria-label={t("wmPlaceAt")} style={{ aspectRatio: `${fw} / ${fh}` }}>
          {Array.from({ length: 9 }, (_, b) => (
            <div key={b} className="wm-pad-box" title={t(`wmAnchor${b}` as "wmAnchor0")}>
              {Array.from({ length: SUB * SUB }, (_, k) => {
                const c = (b % 3) * SUB + (k % SUB), r = Math.floor(b / 3) * SUB + Math.floor(k / SUB), n = r * PAD + c;
                return <button key={k} type="button" className={n === m.curCell ? "on" : m.otherCells.has(n) ? "other" : ""}
                  aria-label={`${t(`wmAnchor${b}` as "wmAnchor0")} ${k + 1}`} aria-pressed={n === m.curCell} onClick={() => m.placeAt(c, r)} />;
              })}
            </div>
          ))}
        </div>
      </>}>
          <Pad>
            {logos.length > 0 && (
              <ul className="item-list">
                {logos.map((l, i) => (
                  // .logo-row: kotak (gambar, nama, hapus) lalu kendalinya
                  // DI LUAR kotak, sejajar di sebelah kanan.
                  <li key={i} className={"logo-row" + (i === sel ? " on" : "")} onClick={() => m.setSel(i)}>
                    <div className="logo-box">
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img className="wm-logo-thumb" src={m.logoURL(l.image)} alt="" onLoad={m.onLogoLoad(l.image)} />
                    <div className="grow">
                      <p className="hi-title wm-name" title={l.image}>{baseName(l.image)}</p>
                      {m.dims[l.image] && <p className="meta">{m.dims[l.image].w} × {m.dims[l.image].h}</p>}
                    </div>
                    <button type="button" className="ghost tiny icon-only logo-del" aria-label={t("wmRemoveLogo")} title={t("wmRemoveLogo")}
                      onClick={(e) => { e.stopPropagation(); m.setLogos((ls) => ls.filter((_, j) => j !== i)); m.setSel(0); }}>
                      <X className="ico" aria-hidden="true" /></button>
                    </div>
                    <div className="logo-ctl">
                      {/* Ukuran milik TIAP logo, jadi duduk di barisnya. */}
                      <Stepper value={l.width} onChange={(width) => m.setLogo(i, { width })} min={5} max={100} step={1} suffix="%" />
                      {/* Kunci ke kotak judul: milik TIAP logo. Selalu ada; mati
                          tanpa kotak judul (tidak ada yang bisa dikunci). */}
                      <button type="button" className={"ghost tiny icon-only wm-pin" + (l.anchor === PIN ? " active" : "")} aria-pressed={l.anchor === PIN}
                        disabled={!nb} aria-label={t("wmLockBox")} title={`${t("wmLockBox")} — ${t("wmLocked")}`}
                        onClick={(e) => { e.stopPropagation(); m.toggleLock(i); }}>
                        <Lock className="ico" aria-hidden="true" /></button>
                      <button type="button" className="ghost tiny" onClick={(e) => { e.stopPropagation(); choose(i); }}>{t("wmChange")}</button>
                    </div>
                  </li>
                ))}
              </ul>
            )}
            <button type="button" className={"ghost cap-add" + (logos.length ? "" : " wm-logo-empty")} onClick={() => choose(-1)}>
              {logos.length ? <Plus className="ico" aria-hidden="true" /> : <ImageIcon className="ico" aria-hidden="true" />}
              {" "}{logos.length ? t("wmAddLogo") : t("wmPickLogo")}
            </button>
            {!logos.length && <span className="mc-note">{t("wmLogoHint")}</span>}
            {upNote && <span className="mc-note">{upNote}</span>}
          </Pad>
      </Split>
    </Group>
  </>;
}

// MarkHeadline = tab Headline halaman Mark.
// llm = baris sumber judul (teks sendiri | judul LLM per klip) — hanya Clips.
export function MarkHeadline({ m, llm = false }: { m: Mark; llm?: boolean }) {
  const { t } = useI18n();
  const { watermark, setWatermark, cardStyle, fh } = m;
  const [fmt, setFmt] = useSticky<string>("mark.fmt", "size");
  return (
    // SATU grup: sakelar kotak judul, lalu baris di kiri dan panel
    // warna di kanan. Baris khusus kotak judul selalu ada — dimatikan,
    // tidak disembunyikan — jadi menyalakannya tidak menggeser apa pun.
    <Group title={t("wmTabHeadline")}>
      <Pad>
          <Toggle checked={!!watermark.hlBox} label={t("wmNewsBox")} desc={t("wmNewsBoxTip")} onChange={(on) => {
            setWatermark({
              hlBox: on,
              // Kartu berita: teks merah di kotak putih tanpa garis tepi,
              // kotaknya di bawah tengah. Judul biasa: tepi huruf bawaan.
              ...(on && toHex(watermark.hlColor) === "#ffffff" ? { hlColor: "red" } : {}),
              ...(on ? { hlDY: Math.round(fh * 0.22), hlOutline: 0 } : { hlDY: 0, hlOutline: outlineFor(watermark.hlSize) }),
            });
            // Logo TIDAK dikunci otomatis — pengguna menguncinya sendiri lewat tombol kunci.
          }} />
      </Pad>
      {/* Kanan: Format lalu Colour, keduanya pemilih sasaran + SATU kendali
          di bawahnya. Align mati (tidak hilang) tanpa kotak judul. */}
      <Split side={<>
        <span className="mc-r-l">{t("wmFormatTitle")}</span>
        {(() => {
          // Tebal garis tepi pindah ke panel Colour (sasaran Outline).
          const off = (k: string) => k === "align" && !watermark.hlBox;
          // Simpanan lama bisa berisi "outline" (sudah pindah ke panel Colour) → size.
          const cur = fmt !== "align" || off(fmt) ? "size" : "align";
          return <>
            <div className="segmented cp-targets" role="group" aria-label={t("wmFormatTitle")}>
              {([["size", t("wmFmtSize")], ["align", t("wmFmtAlign")]] as const).map(([k, name]) => (
                <button key={k} type="button" className={k === cur ? "on" : ""} aria-pressed={k === cur}
                  disabled={off(k)} onClick={() => setFmt(k)}>{name}</button>
              ))}
            </div>
            {/* Slot setinggi tetap: berganti sasaran tidak menggeser Colour di bawahnya. */}
            <div className="wm-fmt-slot">
              {cur === "size" && <Segmented label={t("wmTextSize")} value={watermark.hlSize} onChange={(v) => setWatermark({ hlSize: v })}
                options={[{ value: SIZES[0], name: t("wmSmall") }, { value: SIZES[1], name: t("wmMedium") }, { value: SIZES[2], name: t("wmLarge") }]} />}
              {cur === "align" && <AlignPicker label={t("wmAlign")} value={watermark.hlAlign || "left"} onChange={(v) => setWatermark({ hlAlign: v })} />}
            </div>
          </>;
        })()}
        {/* Kartu: warna panel datang dari Theme, dan kartu tidak bergaris tepi —
            hanya Text yang masih diatur di sini. */}
        <ColorPanel targets={watermarkColors(watermark, setWatermark, t, true, m.first ? m.autoColor : undefined).map((x) => cardStyle && x.key !== "text" ? { ...x, off: true } : x)} />
      </>}>
        {llm && (
          <Row label={t("headlineSource")} title={t("headlineSourceTip")}>
            <Segmented label={t("headlineSource")} value={watermark.hlSource} onChange={(v) => setWatermark({ hlSource: v })}
              options={[{ value: "text" as const, name: t("headlineMine") }, { value: "llm" as const, name: t("headlineLLM") }]} />
          </Row>
        )}
        <Row label={t("wmTitleText")} off={llm && watermark.hlSource === "llm"}>
          {/* SATU kolom setinggi tetap di kedua mode (dulu input berganti
              textarea dan panelnya melompat). Enter = baris baru hanya
              di kotak judul; judul biasa dibungkus otomatis. */}
          <textarea id="wm-title" rows={2} value={watermark.hlText} spellCheck={false}
            placeholder={llm && watermark.hlSource === "llm" ? t("headlineFromLLM") : t("wmTitlePlaceholder")}
            aria-label={t("wmTitleText")} onChange={(e) => setWatermark({ hlText: e.target.value })}
            onKeyDown={(e) => { if (e.key === "Enter" && !watermark.hlBox) e.preventDefault(); }} />
        </Row>
          <Row label={t("wmCredit")} off={!watermark.hlBox}>
            <input id="wm-credit" value={watermark.hlCredit || ""} spellCheck={false} placeholder={t("wmCreditPlaceholder")}
              aria-label={t("wmCredit")} onChange={(e) => setWatermark({ hlCredit: e.target.value })} />
          </Row>
          {/* Gaya: Box (sudut diatur sendiri) atau Look (tampilan jadi kartu
              berita). Keduanya selalu tampil — yang tidak berlaku dimatikan.
              */}
          <Row label={t("wmStyle")} off={!watermark.hlBox}>
            <Segmented label={t("wmStyle")} value={watermark.hlStyle || "box"} onChange={(v) => {
              setWatermark({ hlStyle: v });
              if (v === "look" && !watermark.hlLook) m.applyLook("photo");
            }} options={[{ value: "box" as const, name: t("wmStyleBox") }, { value: "look" as const, name: t("wmStyleLook") }]} />
          </Row>
          <Row label={t("wmCorner")} off={!watermark.hlBox || watermark.hlStyle === "look"}>
            <Segmented label={t("wmCorner")} value={watermark.hlSharp ? "sharp" : "round"} onChange={(v) => setWatermark({ hlSharp: v === "sharp" })}
              options={[{ value: "round", name: t("wmCornerRound") }, { value: "sharp", name: t("wmCornerSharp") }]} />
          </Row>
          <Row label={t("wmTheme")} off={!watermark.hlBox || watermark.hlStyle !== "look"}>
            {/* Auto: engine mencari warna panel yang menyambung dasar gambar. */}
            <ThemePicker label={t("wmTheme")} value={watermark.hlLook || ""} onChange={m.applyLook}
              auto={{ onClick: () => m.applyLook("auto"), disabled: !m.first }} />
          </Row>
      </Split>
    </Group>
  );
}

// MarkTemplates = template tersimpan (pakai, jadikan bawaan, hapus) di kiri,
// simpan setelan sekarang di kanan. data() = setelan yang disimpan; onApply =
// penerapnya (halaman menambah bagian miliknya sendiri, mis. format).
export function MarkTemplates({ tpl, data, onApply, active }: {
  tpl: Templates; data: () => Record<string, unknown>; onApply: (v: Record<string, unknown>, name: string) => void;
  // active = template yang terakhir dipakai (ditandai di daftarnya).
  active?: string;
}) {
  const { t } = useI18n();
  const [name, setName] = useState("");
  const save = async () => { if (name.trim() && (await tpl.save(name.trim(), data()))) setName(""); };
  return (
    // Sejajar: kiri = template tersimpan, kanan = simpan yang sekarang.
    <Group title={t("wmTplTitle")}>
      <Split side={<>
        <span className="mc-r-l">{t("wmTplNew")}</span>
        <input value={name} maxLength={60} onChange={(e) => setName(e.target.value)} aria-label={t("wmTplName")} placeholder={t("wmTplName")}
          onKeyDown={(e) => { if (e.key === "Enter") save(); }} />
        <button type="button" className="ghost" disabled={!name.trim()} onClick={save}>{t("wmTplSave")}</button>
        <span className="mc-note">{t("wmTplHint")}</span>
      </>}>
          <Pad>
            <span className="mc-r-l">{t("wmTplSaved")}</span>
            {tpl.templates.length ? (
              <ul className="item-list">
                {tpl.templates.map((tp) => (
                  <li key={tp.name} className={tp.name === active ? "on" : ""}>
                    <p className="hi-title wm-name grow" title={tp.name}>{tp.name}</p>
                    {/* Bintang = template bawaan: terpasang otomatis di setiap kartu News baru. */}
                    <button type="button" className={"ghost tiny icon-only tpl-star" + (tp.default ? " active" : "")} aria-pressed={!!tp.default}
                      aria-label={t("wmTplDefault")} title={t("wmTplDefault")} onClick={() => tpl.setDefault(tp.name)}>
                      <Star className="ico" aria-hidden="true" /></button>
                    <button type="button" className="ghost tiny" onClick={() => onApply(tp.data, tp.name)}>{tp.name === active ? t("wmTplInUse") : t("wmTplApply")}</button>
                    <button type="button" className="ghost tiny icon-only" aria-label={t("wmTplDelete")} title={t("wmTplDelete")}
                      onClick={() => tpl.remove(tp.name)}><X className="ico" aria-hidden="true" /></button>
                  </li>
                ))}
              </ul>
            ) : <span className="mc-note">{t("wmTplNone")}</span>}
          </Pad>
      </Split>
    </Group>
  );
}

// MarkLayers = lapisan Mark di atas bingkai pratinjau (kotak judul / kartu /
// judul biasa, logo, tanda kunci & magnet). Induknya wajib position:relative,
// ber-ref m.boxRef, dan menyetel --pvh = tinggi bingkai dalam piksel.
export function MarkLayers({ m, frameH }: { m: Mark; frameH: number }) {
  const { t } = useI18n();
  const { eff, nb, fw, fh, fontScale, headlineDrag, watermark, placed, sel, logos, stuck, dragAt, hlLines, hlAt } = m;
  // Ukuran huruf .ass → CSS: satuan Fontsize per tinggi bingkai, kali --pvh.
  const pv = (units: number) => `calc(${units / fh} * var(--pvh))`;
  // Garis tepi: warna pilihan, atau otomatis (hitam; putih untuk teks hitam).
  const outline = toHex(outlineColor(eff));
  const o = (eff.hlOutline * frameH) / fh;
  return <>
    {nb?.photo ? (
      // Tampilan kartu berita: bingkai sudah dipotong engine (card=1);
      // panel, peralihan pudar, garis aksen, judul, dan lencana kredit
      // digambar di sini dengan angka yang sama dengan subtitle.writeCard.
      (() => {
        const panel = toHex(eff.hlBoxColor || "white");
        const rgb = hexToRgb(panel).join(",");
        const badge = toHex(eff.hlBadge || "#FFD400");
        const at = (a: string) => eff.hlAlign === "center" ? "center" : eff.hlAlign === "right" ? "flex-end" : a;
        return <>
          <div className="wm-card-panel wm-grab" {...headlineDrag} style={{ top: `${((nb.photo - 1) / fh) * 100}%`, background: panel }} />
          <div className="wm-card-fade" style={{ height: `${(nb.photo / fh) * 100}%`,
            background: `linear-gradient(180deg,rgba(0,0,0,.35) 0%,rgba(0,0,0,0) 26%,rgba(${rgb},0) 60%,rgba(${rgb},.92) 88%,${panel} 100%)` }} />
          <div className="wm-card-text wm-grab" {...headlineDrag} style={{ left: `${(nb.left / fw) * 100}%`, top: `${((nb.top - 40) / fh) * 100}%`, width: `${(nb.w / fw) * 100}%`,
            fontFamily: `"${HL_FONT}", sans-serif`, alignItems: at("flex-start") }}>
            <span style={{ width: pv(64), height: pv(6), background: badge, marginBottom: pv(28 + 12 - 6) }} />
            <div style={{ alignSelf: "stretch", fontSize: pv(eff.hlSize * fontScale), lineHeight: 1 / fontScale, color: toHex(eff.hlColor), textAlign: eff.hlAlign || "left" }}>
              {nb.lines.map((line, i) => <div key={i} style={eff.hlAlign === "justify" ? { textAlignLast: nb.stretch?.[i] ? "justify" : "left" } : undefined}>{line}</div>)}
            </div>
            {nb.credit > 0 && (
              <span className="wm-card-chip" style={{ marginTop: pv(Math.floor(eff.hlSize / 2)), padding: `${pv(8)} ${pv(14)}`, background: badge,
                color: toHex(eff.hlOnBadge || "#15181D"), fontSize: pv(nb.credit * fontScale), lineHeight: 1 / fontScale }}>{watermark.hlCredit}</span>
            )}
          </div>
        </>;
      })()
    ) : nb && (
      <div className={"wm-newsbox wm-grab" + (stuck >= 0 && dragAt ? " stick" : "")} {...headlineDrag}
        style={{
          left: `${(nb.left / fw) * 100}%`, top: `${(nb.top / fh) * 100}%`,
          width: `${(nb.w / fw) * 100}%`, height: `${(nb.h / fh) * 100}%`,
          borderRadius: pv(nb.radius), padding: pv(nb.pad),
          fontFamily: `"${HL_FONT}", sans-serif`,
          background: toHex(eff.hlBoxColor || "white"),
          // \bord libass pada gambar vektor = garis DI LUAR bentuknya.
          boxShadow: eff.hlOutline > 0 ? `0 0 0 ${o}px ${outline}` : undefined,
        }}>
        <div style={{ fontSize: pv(eff.hlSize * fontScale), lineHeight: 1 / fontScale, color: toHex(eff.hlColor), textAlign: eff.hlAlign || "left" }}>
          {/* Justify: hanya baris yang engine rentangkan (stretch) — sama dengan hasil render. */}
          {nb.lines.map((line, i) => <div key={i} style={eff.hlAlign === "justify" ? { textAlignLast: nb.stretch?.[i] ? "justify" : "left" } : undefined}>{line}</div>)}
        </div>
        {nb.credit > 0 && (
          <div className="wm-credit" style={{ textAlign: eff.hlAlign === "justify" ? "left" : eff.hlAlign || "left", color: isDark(eff.hlBoxColor) ? "#eee" : "#222", marginTop: pv(Math.floor(eff.hlSize / 4)), fontSize: pv(nb.credit * fontScale), lineHeight: 1 / fontScale }}>
            {watermark.hlCredit}
          </div>
        )}
      </div>
    )}
    {placed.map((l, i) => (
      /* eslint-disable-next-line @next/next/no-img-element */
      <img key={i} className={"wmoverlay wm-grab" + (i === sel ? " on" : "")} alt={t("wmLogoTitle")} draggable={false}
        src={m.logoURL(l.image)} onLoad={m.onLogoLoad(l.image)}
        style={{
          left: `${(l.x / fw) * 100}%`, top: `${(l.y / fh) * 100}%`,
          width: `${l.width}%`, height: `${l.height}%`,
        }}
        {...m.logoDrag(i)} />
    ))}
    {/* Tanda di logo: gembok = terkunci ke kotak judul (ikut bergeser
        bersamanya); magnet = sedang menempel ke tepi kotak saat diseret. */}
    {placed.map((l, i) => {
      const locked = logos[i]?.anchor === PIN && nb;
      const sticking = stuck === i && dragAt;
      if (!locked && !sticking) return null;
      const Icon = sticking ? Magnet : Lock;
      return (
        <div key={"m" + i} className={"wm-mark" + (sticking ? " stick" : "")} title={sticking ? t("wmSnapped") : t("wmLocked")}
          style={{ left: `${((l.x + (fw * l.width) / 200) / fw) * 100}%`, top: `${((l.y - (fh * l.height) / 200) / fh) * 100}%` }}>
          <Icon className="ico" aria-hidden="true" />
        </div>
      );
    })}
    {!nb && hlLines.length > 0 && (
      <div className="suboverlay headlineoverlay wm-grab"
        style={{
          left: `${(hlAt.x / fw) * 100}%`, top: `${(hlAt.y / fh) * 100}%`,
          fontFamily: `"${HL_FONT}", sans-serif`,
          fontSize: pv(eff.hlSize * fontScale),
          lineHeight: 1 / fontScale,
          color: toHex(eff.hlColor),
          textShadow: `-${o}px -${o}px 0 ${outline},${o}px -${o}px 0 ${outline},-${o}px ${o}px 0 ${outline},${o}px ${o}px 0 ${outline}`,
        }}
        {...headlineDrag}>
        {hlLines.map((line, i) => <div key={i}>{line}</div>)}
      </div>
    )}
  </>;
}

// MarkFont = font asli dimuat supaya pratinjau memakai huruf yang SAMA dengan
// yang dibakar libass. DUA aturan per font, tegak dan tebal: kalau hanya satu,
// browser menebalkan sendiri face tegaknya, dan itu tidak sama dengan face
// tebal sungguhan libass. names = font lain yang ikut dimuat (subtitle Clips).
export function MarkFont({ names = [HL_FONT] }: { names?: string[] }) {
  return <style dangerouslySetInnerHTML={{ __html: [...new Set(names)].flatMap((n) => [400, 700].map((w) =>
    `@font-face{font-family:"${n}";font-weight:${w};src:url("${eng(`/api/font-file?name=${encodeURIComponent(n)}&weight=${w}`)}");font-display:swap;}`
  )).join("") }} />;
}

// MarkTiming = baris "kapan tampil" (seluruh video | atur waktu + detiknya).
// Mark & Clips. Detiknya selalu ada, mati selagi "seluruh video" — barisnya
// tidak berubah tinggi.
export function MarkTiming({ timed, setTimed, m }: { timed: boolean; setTimed: (v: boolean) => void; m: Mark }) {
  const { t } = useI18n();
  const { watermark, setWatermark } = m;
  return (
    <Row label={t("wmWhenTitle")}>
      <Segmented label={t("wmWhenTitle")} value={timed ? "timed" : "whole"} onChange={(v) => {
        const on = v === "timed";
        setTimed(on);
        if (on && !watermark.dur) setWatermark({ dur: 5 });
      }} options={[{ value: "whole", name: t("wmWhenWhole") }, { value: "timed", name: t("wmWhenTimed") }]} />
      <fieldset className="wm-timed" disabled={!timed}>
          {t("wmFromA")}
          <input type="number" min={0} value={watermark.at} aria-label={t("wmAt")}
            onChange={(e) => setWatermark({ at: Math.max(0, Number(e.target.value) || 0) })} />
          {t("wmFromB")}
          <input type="number" min={1} value={watermark.dur || 5} aria-label={t("wmFor")}
            onChange={(e) => setWatermark({ dur: Math.max(1, Number(e.target.value) || 1) })} />
          {t("wmFromC")}
      </fieldset>
    </Row>
  );
}
