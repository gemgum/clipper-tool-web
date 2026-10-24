"use client";

// Tab watermark: logo + judul untuk video yang SUDAH jadi (DESIGN-Clipper-Lanjutan §4).
//
// Halaman klip memotong lalu membakar identitas; halaman ini hanya membakar.
// Koordinat mentah diganti tarikan langsung: logo dan judul diseret di
// pratinjau, sembilan titik jangkar jadi jalan cepat, dan ukuran logo satu
// angka yang menjaga proporsinya.
//
// notes/44: format feed 4:5 & 1:1 (foto juga), beberapa logo sekaligus, kisi
// yang selalu terlihat, dan kotak judul gaya kartu berita yang ukurannya
// mengikuti teks — logo pertama bisa ditempel otomatis di tepi atasnya.
// Logo bisa DIKUNCI ke kotak itu (ikut bergeser bersamanya) dan menempel ke
// tepinya saat diseret. Setelan bisa disimpan sebagai template di server.
//
// Ikon: lucide-react (ISC). Tanpa satu emoji pun — alasannya di gui/app/page.tsx.

import { useCallback, useEffect, useRef, useState } from "react";
import PageHeader from "../page-header";
import { Bookmark, Eye, Film, Plus, RectangleVertical, Type, X, ImageIcon, Lock, Magnet } from "lucide-react";

import { eng, isWeb, upload, useWeb, post } from "../engine";
import { useI18n } from "../i18n";
import Alerts from "../alerts";
import { DEFAULT_WATERMARK, watermarkToAPI, watermarkOn, headlineAnchor, headlineBox, wrapHeadline,
  frameSize } from "../watermark-model";
import type { Format, NewsLayout, Watermark } from "../watermark-model";
import { useConfirm } from "../confirm";
import { CENTER_X, useLayerDrag } from "../drag";
import type { Font } from "../preview-panel";
import Guides, { GridPicker } from "../guides";
import { Segmented, Toggle } from "../clip-steps";
import TabCard, { Group, Pad, Row, Split } from "../tab-card";
import { watermarkColors } from "../watermark-panel";
import Stage, { StageFrame, useStageWidth } from "../stage";
import ColorPanel, { isDark, toHex } from "../color-panel";
import { TechNotes } from "../log-panel";
import { useJob } from "../use-job";
import Picker from "../picker";
import FileCard, { baseName, useFiles } from "../file-list";
import Select from "../select";
import Stepper from "../stepper";
import { useKeep, useRestore } from "../persist";

type FileResult = { video: string; name: string; output?: string; seconds?: number; error?: string };
type Probe = { w: number; h: number; dur: number } | "error";

// Foto dikenali dari akhirannya, sama dengan watermark.IsImage di engine.
const isImg = (p: string) => /\.(jpe?g|png|webp)$/i.test(p);
const ok = (p: Probe | undefined) => !!p && p !== "error";
const mmss = (s: number) => `${Math.floor(s / 60)}:${String(Math.floor(s % 60)).padStart(2, "0")}`;

// Font judul. Satu untuk seluruh aplikasi — pemilihnya ada di halaman klip,
// dan pemilih kedua berarti pengukuran kedua (notes/29).
const HL_FONT = "Montserrat";
// Kecil / Sedang / Besar dalam satuan bingkai 1080x1920.
const SIZES = [48, 64, 88];
// Garis tepi bawaan judul biasa: makin besar huruf, makin tebal tepinya. Kotak
// judul bawaannya tanpa garis tepi. Keduanya bisa diubah pengguna.
const outlineFor = (size: number) => Math.max(2, Math.round(size / 20));
// Jarak (ruang bingkai) tempat logo yang diseret menempel ke tepi kotak judul.
const STICK = 24;
// Sama dengan aspectTolerance di engine/internal/watermark. Rasio lain TIDAK
// ditolak, cuma disesuaikan (notes/44) — ini menentukan tandanya saja.
const fits = (p: Probe | undefined, f: Format) => {
  const { w, h } = frameSize(f);
  return !!p && p !== "error" && Math.abs(p.w / p.h - w / h) <= 0.02;
};

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
const PIN = 9;
// Satu logo di halaman ini. Tingginya tidak disimpan: diturunkan dari lebar dan
// rasio gambarnya, jadi kotaknya selalu pas dengan gambarnya. anchor -1 = posisi
// bebas hasil seretan.
type LogoItem = { image: string; x: number; y: number; width: number; anchor: number; dx?: number; dy?: number };
// Template = setelan halaman TANPA berkas dan teks judul (lihat applySettings).
type Template = { name: string; data: Record<string, unknown> };

// Isi folder yang ditambahkan: video DAN foto.
const KEEP = (e: { video?: boolean; path: string }) => !!e.video || isImg(e.path);

export default function WatermarkPage() {
  const { t } = useI18n();

  const [probes, setProbes] = useState<Record<string, Probe>>({});
  // Mode web (notes/42): video & logo DIUNGGAH lewat input berkas.
  const web = useWeb();
  const logoInput = useRef<HTMLInputElement>(null);
  const [picking, setPicking] = useState<"" | "file" | "folder" | "out" | "logo">("");
  const [outDir, setOutDir] = useState("");
  const [quality, setQuality] = useState("hd");

  const [watermark, setWatermarkState] = useState<Watermark>(DEFAULT_WATERMARK);
  const setWatermark = useCallback((patch: Partial<Watermark>) => {
    setWatermarkState((b) => ({ ...b, ...patch }));
  }, []);
  const [format, setFormat] = useState<Format>("short");
  // Berkas berasio lain: dipotong memenuhi bingkai, atau utuh di atas latar blur.
  const [fit, setFit] = useState<"crop" | "whole">("crop");
  const { w: fw, h: fh } = frameSize(format);
  const [logoOn, setLogoOn] = useState(true);
  const [logos, setLogos] = useState<LogoItem[]>([]);
  const [sel, setSel] = useState(0);
  const cur = logos[sel] as LogoItem | undefined;
  const setLogo = useCallback((i: number, patch: Partial<LogoItem>) =>
    setLogos((ls) => ls.map((l, j) => (j === i ? { ...l, ...patch } : l))), []);
  // Logo yang dipilih lewat pemilih berkas: ganti logo ke-i, atau -1 = tambah.
  const [logoTarget, setLogoTarget] = useState(-1);
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
  const [timed, setTimed] = useState(false);
  const [tab, setTab] = useState<"format" | "logo" | "headline" | "template">("format");
  // Lebar bingkai = kolom pratinjau baku (stage.tsx), tidak pernah ikut isi.
  const stageW = useStageWidth();
  // Rasio lebar/tinggi tiap logo, dibaca dari gambarnya sendiri saat dimuat.
  const [dims, setDims] = useState<Record<string, { w: number; h: number }>>({});
  const [grid, setGrid] = useState(40);
  const [gridAlways, setGridAlways] = useState(true);

  // Tata letak kotak judul dari engine (baris, tinggi). Posisi tegaknya
  // dihitung di sini dari DY — rumus yang sama dengan NewsBox — supaya
  // menyeret kotak (dan logo yang terkunci padanya) tidak menunggu jaringan.
  const [layout, setLayout] = useState<NewsLayout | null>(null);
  const boxOn = !!watermark.hlBox && !!watermark.hlText.trim();
  useEffect(() => {
    if (!boxOn) { setLayout(null); return; }
    const id = setTimeout(() => {
      post<NewsLayout>("/api/watermark/layout", { font: HL_FONT, watermark: watermarkToAPI({ ...watermark, format, hlDY: 0 }, HL_FONT) })
        .then(setLayout).catch(() => {});
    }, 120);
    return () => clearTimeout(id);
  }, [boxOn, watermark.hlText, watermark.hlSize, watermark.hlCredit, watermark.hlSharp, format]); // eslint-disable-line react-hooks/exhaustive-deps
  const nb = boxOn && layout
    ? { ...layout, top: Math.min(Math.max(0, Math.trunc(fh / 2) + watermark.hlDY - Math.trunc(layout.h / 2)), Math.max(0, fh - layout.h)) }
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

  // Yang dipratinjau & dikirim: logo hanya bila "Pakai logo" menyala, judul
  // selalu bebas di bingkai, tepi dihitung otomatis.
  const eff: Watermark = {
    ...watermark,
    format,
    image: placed[0]?.image || "",
    ...(placed[0] ? { x: placed[0].x, y: placed[0].y, width: placed[0].width, height: placed[0].height } : {}),
    logos: placed.slice(1),
    hlFree: true, hlSource: "text",
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

  const [error, setError] = useState("");
  // Daftar berkas + cara menambahnya: satu untuk semua halaman (file-list.tsx).
  const fl = useFiles(KEEP, setError);
  const { files: videos, setFiles: setVideos, add, addFolder, upNote, setUpNote } = fl;
  // Siklus job latar (use-job.ts): langganan SSE, start, cancel.
  const { jobId, job, logs, busy, start: startJob, cancel } = useJob<{ files: FileResult[] }>("/api/watermark", "watermark", setError, (j) => {
    // Mode web: engine menghapus video unggahan begitu job sukses, jadi daftarnya
    // ikut dibersihkan — menjalankannya lagi butuh unggah ulang.
    const gone = (j.result?.files ?? []).filter((f) => !f.error).map((f) => f.video);
    isWeb().then((w) => w && setVideos((v) => v.filter((x) => !gone.includes(x))));
  });

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
  const hlLines = wrapHeadline(eff.hlText, eff.hlSize, hlBox.w);
  const hlAt = headlineAnchor(hlLines, eff.hlSize, eff.hlDX, eff.hlDY, hlBox);
  // Kotak berita hanya bisa naik-turun: lebarnya selalu selebar bingkai.
  const headlineDrag = nb
    ? dragProps(fw / 2, nb.top + nb.h / 2, (_x, y) => setWatermark({ hlDY: Math.round(y - fh / 2) }), fh / 2, fh)
    : dragProps(hlAt.x, hlAt.y,
      (x, y) => setWatermark({ hlDX: x - hlBox.cx, hlDY: y - hlBox.cy }), hlBox.cy, fh, hlBox.cx);

  // Pratinjau memakai berkas PERTAMA yang cocok formatnya: watermark-nya sama
  // untuk semuanya. Foto tidak punya detik ke-1.
  const first = videos.find((v) => ok(probes[v])) || "";
  const reframe = fit === "whole" ? "reframe=fit&background=blur&zoom=0" : "reframe=center&background=black&zoom=100";
  const frame = first ? eng(`/api/frame?path=${encodeURIComponent(first)}&t=${isImg(first) ? 0 : 1}&${reframe}&format=${format}`) : "";

  // Rasio dibaca dari berkasnya (bukan namanya) begitu video masuk daftar.
  useEffect(() => {
    for (const v of videos) {
      if (probes[v]) continue;
      fetch(eng(`/api/probe?path=${encodeURIComponent(v)}`)).then((r) => r.json().then((d) => ({ ok: r.ok, d })))
        .then(({ ok, d }) => setProbes((p) => ({ ...p, [v]: ok && d.width ? { w: d.width, h: d.height, dur: d.duration } : "error" })))
        .catch(() => setProbes((p) => ({ ...p, [v]: "error" })));
    }
  }, [videos, probes]);

  // Setelan watermark LENGKET: identitas akun dipilih sekali, bukan disusun
  // ulang tiap kali hendak memposting.
  useKeep("watermark", { videos, outDir, quality, watermark, logoOn, logos, timed, format, fit, grid, gridAlways });
  // Satu penerap untuk dua sumber: simpanan browser (useRestore) dan template.
  // keepText = teks judul yang sudah diketik TIDAK ditimpa template.
  const applySettings = (v: Record<string, unknown>, keepText?: string) => {
    if (typeof v.logoOn === "boolean") setLogoOn(v.logoOn);
    if (typeof v.timed === "boolean") setTimed(v.timed);
    if (v.format === "short" || v.format === "feed45" || v.format === "feed11") setFormat(v.format);
    if (v.fit === "crop" || v.fit === "whole") setFit(v.fit);
    if (typeof v.grid === "number") setGrid(v.grid);
    if (typeof v.gridAlways === "boolean") setGridAlways(v.gridAlways);
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
  };
  useRestore<Record<string, unknown>>("watermark", (v) => {
    if (Array.isArray(v.videos)) setVideos(v.videos as string[]);
    if (typeof v.outDir === "string") setOutDir(v.outDir);
    if (typeof v.quality === "string") setQuality(v.quality);
    applySettings(v);
  });

  // Template di server: satu daftar untuk semua akun (notes/44).
  const [templates, setTemplates] = useState<Template[]>([]);
  const [tplName, setTplName] = useState("");
  const [ask, confirmEl] = useConfirm();
  useEffect(() => {
    fetch(eng("/api/watermark/templates")).then((r) => r.json())
      .then((d) => setTemplates(d.templates || [])).catch(() => {});
  }, []);
  const tplCall = async (path: string, init: RequestInit) => {
    try {
      const r = await fetch(eng(path), init);
      const d = await r.json();
      if (!r.ok) throw new Error(d.error || "failed");
      setTemplates(d.templates || []);
      return true;
    } catch (e) { setError(String(e)); return false; }
  };
  const saveTemplate = async () => {
    const name = tplName.trim();
    if (!name) return;
    const data = { format, fit, logoOn, logos, timed, grid, gridAlways, watermark: { ...watermark, hlText: "" } };
    if (await tplCall("/api/watermark/templates", {
      method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name, data }),
    })) setTplName("");
  };
  const deleteTemplate = async (name: string) => {
    if (!(await ask(t("wmTplDeleteAsk", { name }), t("wmTplDelete")))) return;
    await tplCall(`/api/watermark/templates/${encodeURIComponent(name)}`, { method: "DELETE" });
  };



  const pickLogo = async (f: File) => {
    try {
      const p = await upload(f, (x) => setUpNote(`${f.name}: ${t("uploadingPct", { pct: Math.round(x * 100) })}`));
      putLogo(p, logoTarget);
    } catch (e) { setError(`${f.name}: ${String(e)}`); }
    setUpNote("");
  };
  const chooseLogo = (i: number) => {
    setLogoTarget(i);
    if (web) logoInput.current?.click(); else setPicking("logo");
  };

  // Semua berkas yang terbaca ikut; rasio lain disesuaikan engine.
  const ready = videos.filter((v) => ok(probes[v]));
  const nothing = !watermarkOn(eff);

  // Pratinjau hasil: engine membakar satu frame berkas pertama persis seperti
  // hasil akhirnya (watermark.Preview).
  const [shot, setShot] = useState("");
  const [shooting, setShooting] = useState(false);
  const previewResult = async () => {
    if (!first) return;
    setShooting(true); setError("");
    try {
      const r = await fetch(eng("/api/watermark/preview"), {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ file: first, fit, font: HL_FONT, watermark: watermarkToAPI(eff, HL_FONT) }),
      });
      if (!r.ok) throw new Error((await r.json().catch(() => null))?.error || "preview failed");
      const url = URL.createObjectURL(await r.blob());
      setShot((old) => { if (old) URL.revokeObjectURL(old); return url; });
    } catch (e) { setError(String(e)); }
    setShooting(false);
  };
  useEffect(() => {
    if (!shot) return;
    const esc = (e: KeyboardEvent) => { if (e.key === "Escape") setShot(""); };
    window.addEventListener("keydown", esc);
    return () => window.removeEventListener("keydown", esc);
  }, [shot]);

  const start = () => startJob({ videos: ready, quality, out_dir: outDir, fit, font: HL_FONT, watermark: watermarkToAPI(eff, HL_FONT) });

  const files = job?.result?.files ?? [];
  const resultOf = (v: string) => files.findIndex((f) => f.video === v);
  const pct = Math.round((job?.progress ?? 0) * 100);
  const logoURL = (image: string) => eng(`/api/image?path=${encodeURIComponent(image)}`);
  const onLogoLoad = (image: string) => (e: React.SyntheticEvent<HTMLImageElement>) => {
    const { naturalWidth: w, naturalHeight: h } = e.currentTarget;
    if (w && h && (w !== dims[image]?.w || h !== dims[image]?.h)) setDims((d) => ({ ...d, [image]: { w, h } }));
  };
  // Ukuran huruf .ass → CSS: satuan Fontsize per tinggi bingkai, kali --pvh.
  const pv = (units: number) => `calc(${units / fh} * var(--pvh))`;
  // Bingkai pratinjau: lebar paling 340 px, tinggi paling 604 px.
  const frameH = Math.round((stageW * fh) / fw);
  // Garis tepi: warna pilihan, atau otomatis (hitam; putih untuk teks hitam).
  const outline = toHex(eff.hlOutlineColor || (eff.hlColor === "black" ? "white" : "black"));
  const o = (eff.hlOutline * frameH) / fh;
  return (
    <div className="screen scroll clips-v2 wm-v2">
      <PageHeader title={t("tabWatermark")} subtitle={t("subWatermark")}>
        {busy ? (
          <button type="button" className="danger" onClick={cancel}>{t("cancelRun")}</button>
        ) : (
          <button type="button" className="primary big" onClick={start} disabled={ready.length === 0 || nothing}
            title={nothing ? t("wmNothing") : undefined}>
            {files.length ? t("wmRedo") : ready.length === 1 ? t("wmStartOne") : t("wmStart", { n: ready.length })}
          </button>
        )}
      </PageHeader>
      {/* Font asli dimuat supaya pratinjau memakai huruf yang SAMA dengan yang
          dibakar libass. */}
      <style dangerouslySetInnerHTML={{ __html: [400, 700].map((w) =>
        `@font-face{font-family:"${HL_FONT}";font-weight:${w};src:url("${eng(`/api/font-file?name=${encodeURIComponent(HL_FONT)}&weight=${w}`)}");font-display:swap;}`
      ).join("") }} />

      <Alerts items={[error && { kind: "error" as const, text: error }]} />

      {/* Tata letak baku (halaman klip): kiri = berkas + kartu setelan bertab,
          kanan = kolom pratinjau yang menempel (stage.tsx). */}
      <div className="setup-grid">
        <div className="setup-main">
          <FileCard title={t("wmVideosTitle")} fl={fl} web={web} accept="video/*,image/*" addLabel={t("wmAddFile")} note={t("wmVideosNote")}
            onPick={(m) => setPicking(m)}>
            {busy && (
              <div aria-live="polite">
                <p className="meta">{t("wmRunning")} {pct}%</p>
                <div className="bar wr-gap"><div style={{ width: `${pct}%` }} /></div>
              </div>
            )}
            <ul className="cap-videos wm-videos">
              {videos.map((v) => {
                const p = probes[v];
                const ri = resultOf(v);
                const r = ri >= 0 ? files[ri] : undefined;
                const info = p && p !== "error" ? (isImg(v) ? `${p.w}×${p.h}` : `${mmss(p.dur)} · ${p.w}×${p.h}`) : "";
                const [tag, cls] = r ? (r.error ? [t("wmFailedTag"), "bad"] : [t("wmDone"), "ok"])
                  : !p ? [t("wmChecking"), ""] : p === "error" ? [t("wmFailedTag"), "bad"]
                  : fits(p, format) ? [t("wmReady"), "ok"] : [t("wmFitTag"), "ok"];
                return (
                  <li key={v}>
                    {isImg(v) ? <ImageIcon className="ico" aria-hidden="true" /> : <Film className="ico" aria-hidden="true" />}
                    <div className="grow">
                      <p className="wm-name" title={v}>{baseName(v)}</p>
                      <p className="meta">{r?.error || (r?.output ? t("wmSaved", { name: baseName(r.output) }) : info)}</p>
                    </div>
                    {web && r?.output && <a className="btn-ghost" href={eng(`/api/watermark/${jobId}/file?i=${ri}`)} download>{t("download")}</a>}
                    <span className={"wm-tag " + cls}>{tag}</span>
                    <button type="button" className="ghost tiny icon-only" aria-label={t("capRemove")} title={t("capRemove")} disabled={busy}
                      onClick={() => setVideos((cur) => cur.filter((x) => x !== v))}><X className="ico" aria-hidden="true" /></button>
                  </li>
                );
              })}
            </ul>
          </FileCard>

          <TabCard label={t("tabWatermark")} open={tab} setOpen={setTab} tabs={[
            { key: "format", icon: RectangleVertical, label: t("wmFormatTitle") },
            { key: "logo", icon: ImageIcon, label: t("wmLogoTitle") },
            { key: "headline", icon: Type, label: t("wmTabHeadline") },
            { key: "template", icon: Bookmark, label: t("wmTplTitle") },
          ]}>
            {tab === "format" && (
              // Sejajar: kiri = format, kanan = keluaran.
              <Group title={t("wmFormatTitle")}>
                <Split side={<>
                  <span className="mc-r-l" title={t("wmQualityTip")}>{t("quality")}</span>
                  <Select value={quality} onChange={setQuality} disabled={busy} ariaLabel={t("quality")} options={[
                    { value: "draft", label: t("qualityDraft") },
                    { value: "hd", label: t("qualityHd") },
                    { value: "max", label: t("qualityMax") },
                  ]} />
                  {!web && <>
                    <span className="mc-r-l">{t("outputDir")}</span>
                    <div className="path-row">
                      <input id="wm-out" value={outDir} onChange={(e) => setOutDir(e.target.value)} placeholder={t("wmOutPlaceholder")} disabled={busy} aria-label={t("outputDir")} />
                      <button type="button" className="ghost" onClick={() => setPicking("out")}>{t("pickerGo")}…</button>
                    </div>
                  </>}
                </>}>
                    <Row label={t("wmFormatTitle")} note={t("wmFormatTip")}>
                      <Segmented label={t("wmFormatTitle")} value={format} onChange={setFormat}
                        options={[{ value: "short" as Format, name: t("wmFormatShort") }, { value: "feed45" as Format, name: t("wmFormatFeed45") },
                          { value: "feed11" as Format, name: t("wmFormatFeed11") }]} />
                    </Row>
                    <Row label={t("wmFitTitle")}>
                      <Segmented label={t("wmFitTitle")} value={fit} onChange={setFit}
                        options={[{ value: "crop" as const, name: t("wmFitCrop") }, { value: "whole" as const, name: t("wmFitWhole") }]} />
                    </Row>
                    {/* Waktu tampil = satu baris seperti yang lain; detiknya muncul hanya bila diatur. */}
                    <Row label={t("wmWhenTitle")}>
                      <Segmented label={t("wmWhenTitle")} value={timed ? "timed" : "whole"} onChange={(v) => {
                        const on = v === "timed";
                        setTimed(on);
                        if (on && !watermark.dur) setWatermark({ dur: 5 });
                      }} options={[{ value: "whole", name: t("wmWhenWhole") }, { value: "timed", name: t("wmWhenTimed") }]} />
                      {timed && (
                        <span className="wm-timed">
                          {t("wmFromA")}
                          <input type="number" min={0} value={watermark.at} aria-label={t("wmFromA")}
                            onChange={(e) => setWatermark({ at: Math.max(0, Number(e.target.value) || 0) })} />
                          {t("wmFromB")}
                          <input type="number" min={1} value={watermark.dur || 5} aria-label={t("wmFromC")}
                            onChange={(e) => setWatermark({ dur: Math.max(1, Number(e.target.value) || 1) })} />
                          {t("wmFromC")}
                        </span>
                      )}
                    </Row>
                </Split>
              </Group>
            )}

            {tab === "logo" && <>
              {web && (
                <input ref={logoInput} type="file" accept="image/png,image/*" hidden
                  onChange={(e) => { const f = e.target.files?.[0]; e.target.value = ""; if (f) pickLogo(f); }} />
              )}
              {/* Sejajar, bukan atas-bawah (.mc-split, pola panel warna): kiri =
                  daftar logo, kanan = penempatan logo terpilih. Panel kanan SELALU
                  ada — dimatikan bila belum ada logo, tidak disembunyikan — jadi
                  menambah logo tidak menggeser apa pun. */}
              <Group title={t("wmLogoImages")}>
                <Pad><Toggle checked={logoOn} onChange={setLogoOn} label={t("wmUseLogo")} /></Pad>
                <Split sideOff={!cur || !logoOn} side={<>
                  <span className="mc-r-l">{t("wmPlaceAt")}</span>
                  <div className="wm-anchors" role="group" aria-label={t("wmPlaceAt")}>
                    {Array.from({ length: 9 }, (_, i) => (
                      <button key={i} type="button" className={cur?.anchor === i ? "on" : ""} aria-pressed={cur?.anchor === i}
                        aria-label={t(`wmAnchor${i}` as "wmAnchor0")} title={t(`wmAnchor${i}` as "wmAnchor0")}
                        onClick={() => setLogo(sel, { anchor: i })} />
                    ))}
                  </div>
                  {/* Selalu ada; mati tanpa kotak judul (tidak ada yang bisa dikunci). */}
                  <button type="button" className={"ghost tiny wm-pin" + (cur?.anchor === PIN ? " active" : "")} aria-pressed={cur?.anchor === PIN}
                    disabled={!nb} title={t("wmLocked")} onClick={() => toggleLock(sel)}>
                    <Lock className="ico" aria-hidden="true" /> {t("wmLockBox")}</button>
                  <span className="mc-r-l">{t("wmLogoSize")}</span>
                  <Stepper value={cur?.width ?? 10} onChange={(width) => setLogo(sel, { width })} min={5} max={100} step={1} suffix="%" />
                  <span className="mc-note">{t("wmLogoSizeHint")}</span>
                </>}>
                    <Pad>
                      {logos.length > 0 && (
                        <ul className="wm-logos">
                          {logos.map((l, i) => (
                            <li key={i} className={i === sel ? "on" : ""} onClick={() => setSel(i)}>
                              {/* eslint-disable-next-line @next/next/no-img-element */}
                              <img className="wm-logo-thumb" src={logoURL(l.image)} alt="" onLoad={onLogoLoad(l.image)} />
                              <div className="grow">
                                <p className="hi-title wm-name" title={l.image}>{baseName(l.image)}</p>
                                {dims[l.image] && <p className="meta">{dims[l.image].w} × {dims[l.image].h}</p>}
                              </div>
                              <button type="button" className="ghost" onClick={(e) => { e.stopPropagation(); chooseLogo(i); }}>{t("wmChange")}</button>
                              <button type="button" className="ghost tiny icon-only" aria-label={t("wmRemoveLogo")} title={t("wmRemoveLogo")}
                                onClick={(e) => { e.stopPropagation(); setLogos((ls) => ls.filter((_, j) => j !== i)); setSel(0); }}>
                                <X className="ico" aria-hidden="true" /></button>
                            </li>
                          ))}
                        </ul>
                      )}
                      <button type="button" className={"ghost cap-add" + (logos.length ? "" : " wm-logo-empty")} onClick={() => chooseLogo(-1)}>
                        {logos.length ? <Plus className="ico" aria-hidden="true" /> : <ImageIcon className="ico" aria-hidden="true" />}
                        {" "}{logos.length ? t("wmAddLogo") : t("wmPickLogo")}
                      </button>
                      {!logos.length && <span className="mc-note">{t("wmLogoHint")}</span>}
                      {upNote && <span className="mc-note">{upNote}</span>}
                    </Pad>
                </Split>
              </Group>
            </>}

            {tab === "headline" && (
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
                    // Logo pertama langsung terkunci di tepi atas kotaknya — itu
                    // yang otomatis.
                    if (on && logos.length) setLogo(0, { anchor: PIN, dx: 0, dy: 0 });
                  }} />
                </Pad>
                <Split side={<ColorPanel targets={watermarkColors(watermark, setWatermark, t)} />}>
                  <Row label={t("wmTitleText")}>
                    {watermark.hlBox ? (
                      <textarea id="wm-title" rows={3} value={watermark.hlText} spellCheck={false} placeholder={t("wmTitlePlaceholder")}
                        aria-label={t("wmTitleText")} onChange={(e) => setWatermark({ hlText: e.target.value })} />
                    ) : (
                      <input id="wm-title" value={watermark.hlText} spellCheck={false} placeholder={t("wmTitlePlaceholder")}
                        aria-label={t("wmTitleText")} onChange={(e) => setWatermark({ hlText: e.target.value })} />
                    )}
                  </Row>
                  <Row label={t("wmTextSize")}>
                    <Segmented label={t("wmTextSize")} value={watermark.hlSize} onChange={(v) => setWatermark({ hlSize: v })}
                      options={[{ value: SIZES[0], name: t("wmSmall") }, { value: SIZES[1], name: t("wmMedium") }, { value: SIZES[2], name: t("wmLarge") }]} />
                  </Row>
                  {/* Outline: tepi HURUF pada judul biasa, tepi KOTAK pada kotak judul. */}
                  <Row label={t("wmOutline")}>
                    <Stepper value={watermark.hlOutline} onChange={(v) => setWatermark({ hlOutline: v })} min={0} max={12} step={1} suffix="px" />
                  </Row>
                  <fieldset className="cp-plain" disabled={!watermark.hlBox} title={watermark.hlBox ? undefined : t("wmNewsBoxTip")}>
                    <Row label={t("wmCredit")}>
                      <input id="wm-credit" value={watermark.hlCredit || ""} spellCheck={false} placeholder={t("wmCreditPlaceholder")}
                        aria-label={t("wmCredit")} onChange={(e) => setWatermark({ hlCredit: e.target.value })} />
                    </Row>
                    <Row label={t("wmAlign")}>
                      <Segmented label={t("wmAlign")} value={watermark.hlAlign || "left"} onChange={(v) => setWatermark({ hlAlign: v })}
                        options={[{ value: "left" as const, name: t("wmAlignLeft") },
                          { value: "center" as const, name: t("wmAlignCenter") },
                          { value: "right" as const, name: t("wmAlignRight") }]} />
                    </Row>
                    <Row label={t("wmCorner")}>
                      <Segmented label={t("wmCorner")} value={watermark.hlSharp ? "sharp" : "round"} onChange={(v) => setWatermark({ hlSharp: v === "sharp" })}
                        options={[{ value: "round", name: t("wmCornerRound") }, { value: "sharp", name: t("wmCornerSharp") }]} />
                    </Row>
                  </fieldset>
                </Split>
              </Group>
            )}

            {tab === "template" && <>
              {/* Sejajar: kiri = template tersimpan, kanan = simpan yang sekarang. */}
              <Group title={t("wmTplTitle")}>
                <Split side={<>
                  <span className="mc-r-l">{t("wmTplNew")}</span>
                  <input value={tplName} maxLength={60} onChange={(e) => setTplName(e.target.value)} aria-label={t("wmTplName")} placeholder={t("wmTplName")}
                    onKeyDown={(e) => { if (e.key === "Enter") saveTemplate(); }} />
                  <button type="button" className="ghost" disabled={!tplName.trim()} onClick={saveTemplate}>{t("wmTplSave")}</button>
                  <span className="mc-note">{t("wmTplHint")}</span>
                </>}>
                    <Pad>
                      <span className="mc-r-l">{t("wmTplSaved")}</span>
                      {templates.length ? (
                        <ul className="wm-tpls">
                          {templates.map((tp) => (
                            <li key={tp.name}>
                              <span className="wm-name grow" title={tp.name}>{tp.name}</span>
                              <button type="button" className="ghost" onClick={() => { applySettings(tp.data, watermark.hlText); setSel(0); }}>{t("wmTplApply")}</button>
                              <button type="button" className="ghost tiny icon-only" aria-label={t("wmTplDelete")} title={t("wmTplDelete")}
                                onClick={() => deleteTemplate(tp.name)}><X className="ico" aria-hidden="true" /></button>
                            </li>
                          ))}
                        </ul>
                      ) : <span className="mc-note">{t("wmTplNone")}</span>}
                    </Pad>
                </Split>
              </Group>
            </>}
          </TabCard>
          {/* Log di dasar kolom kiri: halaman ini tidak bergulir, kolomnya yang bergulir. */}
          <TechNotes logs={logs} />
        </div>

        <Stage title={t("previewTitle")} badge={`${fw} × ${fh}`} help={t("wmDragHint")}
          tools={<>
            <button type="button" className="wm-preview-btn" disabled={!first || shooting || nothing} title={t("wmPreviewTip")} onClick={previewResult}>
              <Eye className="ico" aria-hidden="true" /> {shooting ? t("wmPreviewing") : t("wmPreviewResult")}
            </button>
            <GridPicker grid={grid} setGrid={setGrid} always={gridAlways} setAlways={setGridAlways} />
          </>}>
          <StageFrame>
            <div className="preview9x16 wm-frame" ref={boxRef}
              style={{ width: stageW, height: frameH, "--pvh": `${frameH}px` } as React.CSSProperties}>
              {frame ? (
                /* eslint-disable-next-line @next/next/no-img-element */
                <img src={frame} alt="" draggable={false} />
              ) : (
                <div className="preview-empty">
                  <div className="pe-icon" aria-hidden="true" />
                  <div className="pe-title">{t("wmEmptyFrame")}</div>
                </div>
              )}
              <Guides grid={grid} visible={gridAlways || dragAt !== null} playH={fh}
                atX={(dragAt?.x ?? placed[sel]?.x) === CENTER_X} atY={(dragAt?.y ?? placed[sel]?.y) === fh / 2} dragAt={dragAt} />
              {nb && (
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
                    {nb.lines.map((line, i) => <div key={i}>{line}</div>)}
                  </div>
                  {nb.credit > 0 && (
                    <div className="wm-credit" style={{ textAlign: eff.hlAlign || "left", color: isDark(eff.hlBoxColor) ? "#eee" : "#222", marginTop: pv(Math.floor(eff.hlSize / 4)), fontSize: pv(nb.credit * fontScale), lineHeight: 1 / fontScale }}>
                      {watermark.hlCredit}
                    </div>
                  )}
                </div>
              )}
              {placed.map((l, i) => (
                /* eslint-disable-next-line @next/next/no-img-element */
                <img key={i} className={"wmoverlay wm-grab" + (i === sel ? " on" : "")} alt={t("wmLogoTitle")} draggable={false}
                  src={logoURL(l.image)} onLoad={onLogoLoad(l.image)}
                  style={{
                    left: `${(l.x / fw) * 100}%`, top: `${(l.y / fh) * 100}%`,
                    width: `${l.width}%`, height: `${l.height}%`,
                  }}
                  {...logoDrag(i)} />
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
            </div>
          </StageFrame>
        </Stage>
      </div>


      {confirmEl}
      {shot && (
        <div className="wm-shot" role="dialog" aria-label={t("wmPreviewResult")} onClick={() => setShot("")}>
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={shot} alt={t("wmPreviewResult")} />
          <button type="button" className="ghost wm-shot-close" aria-label={t("close")} title={t("close")}><X className="ico" aria-hidden="true" /></button>
        </div>
      )}
      {picking && (
        <Picker
          mode={picking === "folder" || picking === "out" ? "folder" : "file"}
          start={picking === "logo" ? logos[logoTarget]?.image || logos[0]?.image : undefined}
          onPick={(p) => {
            if (picking === "folder") addFolder(p);
            else if (picking === "out") setOutDir(p);
            else if (picking === "logo") putLogo(p, logoTarget);
            else add([p]);
            setPicking("");
          }}
          onClose={() => setPicking("")}
        />
      )}
    </div>
  );
}
