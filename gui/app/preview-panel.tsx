"use client";

// Ikon: lucide-react (ISC) — alasannya di gui/app/page.tsx.
import { Eye, RotateCw, X } from "lucide-react";
import Tip from "./tip";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useI18n } from "./i18n";
import { eng } from "./engine";
import Stepper from "./stepper";
import { PLAY_H, PLAY_W, CENTER_X, CENTER_Y, snap, useLayerDrag } from "./drag";
import Guides, { GridPicker, PositionField } from "./guides";
import { headlineAnchor, headlineBox, wrapHeadline } from "./watermark-model";
import type { Watermark } from "./watermark-model";
import Select from "./select";
import Warn from "./warn";

// Ruang koordinat, titik tengah, dan mesin seret tinggal di ./drag — dipakai
// bersama halaman watermark. Diekspor ulang dari sini supaya pemanggil lama
// (page.tsx) tidak perlu tahu ia pindah.
export { PLAY_W, PLAY_H, CENTER_X, CENTER_Y } from "./drag";

// Area yang tertutup UI tiap aplikasi (fraksi tinggi/lebar frame 9:16).
// Angka awal hasil pengukuran kasar — sesuaikan bila UI aplikasi berubah.
//
// Tinggal di sini bersama pratinjau, bukan di <FramePanel>: pembatas ini
// menggambar overlay DI ATAS bingkai dan mengatur ke mana subtitle boleh
// ditaruh — bukan bagaimana video dipasang ke bingkai.
export type Zone = { top: number; bottom: number; right: number };
export const PLATFORMS: Record<string, { label: string; zone: Zone }> = {
  tiktok: { label: "TikTok", zone: { top: 0.08, bottom: 0.20, right: 0.16 } },
  reels: { label: "Instagram Reels", zone: { top: 0.07, bottom: 0.17, right: 0.15 } },
  shorts: { label: "YouTube Shorts", zone: { top: 0.06, bottom: 0.13, right: 0.14 } },
  generic: { label: "", zone: { top: 0.08, bottom: 0.20, right: 0.16 } },
};

// scale = piksel CSS per satuan "size" subtitle. WAJIB dipakai pratinjau: .ass
// mengartikan ukuran sebagai tinggi kotak font, CSS mengartikannya sebagai em,
// dan selisihnya berbeda tiap font (Montserrat 0,725 · Anton 0,577). Engine yang
// menghitungnya dari berkas fontnya — lihat engine/internal/api/fontmetrics.go.
export type Font = { name: string; scale: number };
// Hasil /api/font-check: valid = font benar-benar ada (bawaan atau sistem).
export type FontCheck = { valid: boolean; name: string; family: string; source: string; error: string; scale: number };

// Pilihan kerapatan grid di ruang 1080×1920. 0 = mati.
//
// 20 jadi bawaan: 54×96 kotak — cukup kasar untuk membuat penempatan bisa
// diulang, cukup halus untuk tidak terasa seperti pagar. Yang lain disediakan
// karena "rasa" grid hanya bisa dinilai dengan mencoba, bukan dari angka.
// Baris contoh di preview — panjangnya sengaja mendekati batas nyata satu baris
// (~22 karakter pada ukuran font bawaan).
const SAMPLE_LINES = ["sampleLine1", "sampleLine2", "sampleLine3"] as const;

// Nama warna lama ATAU hex (#rrggbb, swatch DESIGN.md §7.1) — engine menerima keduanya.
const hex = (c: string) => (c.startsWith("#") ? c : c === "yellow" ? "#ffdd00" : c === "green" ? "#4ade80" : c === "cyan" ? "#38bdf8" : "#ffffff");

// Berapa baris yang mungkin muncul sekaligus — harus sama dengan Pacing() di
// engine (lambat/normal 2, padat 3; mode word selalu 1 kata per layar).
export const linesFor = (subMode: string, subSpeed: string) =>
  subMode === "word" ? 1 : subSpeed === "dense" ? 3 : 2;

// Bingkai 9:16 + seretan subtitle + kendali huruf dan posisi.
//
// children diletakkan di ujung kolom setelan: di situlah <FramePanel> duduk,
// sebab kendali bingkai memang mengubah gambar di sebelah kiri.
export default function PreviewPanel({
  path, reframe, background, zoom, zone,
  fonts, subFont, setSubFont, fontManual, setFontManual, fontCheck, fontChecking,
  subSize, setSubSize, subColor, setSubColor, subOutline, setSubOutline, subBox, setSubBox,
  subMode, setSubMode, subHighlight, setSubHighlight, subSpeed, setSubSpeed,
  subX, setSubX, subY, setSubY, blockH, centerAnchorY,
  watermark, moveWatermark, moveHeadline, wmOpen, watermarkPanel,
  platform, setPlatform, inUnsafe, onPlaceSafe, addLog, children,
  part, grid, setGrid, alwaysGuides, setAlwaysGuides, showZones = true,
}: {
  path: string; reframe: string; background: string; zoom: number; zone?: Zone;
  platform: string; setPlatform: (v: string) => void;
  inUnsafe: boolean; onPlaceSafe: () => void;
  fonts: Font[];
  subFont: string; setSubFont: (v: string) => void;
  fontManual: boolean; setFontManual: (v: boolean) => void;
  fontCheck: FontCheck | null; fontChecking: boolean;
  subSize: number; setSubSize: (v: number) => void;
  subColor: string; setSubColor: (v: string) => void;
  subOutline: number; setSubOutline: (v: number) => void;
  subBox: boolean; setSubBox: (v: boolean) => void;
  subMode: string; setSubMode: (v: string) => void;
  subHighlight: string; setSubHighlight: (v: string) => void;
  subSpeed: string; setSubSpeed: (v: string) => void;
  subX: number; setSubX: (v: number | ((p: number) => number)) => void;
  subY: number; setSubY: (v: number | ((p: number) => number)) => void;
  blockH: number; centerAnchorY: number;
  watermark: Watermark;
  moveWatermark: (x: number, y: number) => void;
  moveHeadline: (x: number, y: number) => void;
  // wmOpen menukar isi kolom setelan: selagi menyala, hanya kelompok
  // watermark yang tampil. Alasannya tinggi, dan angkanya di watermark-panel.tsx.
  wmOpen: boolean;
  watermarkPanel: React.ReactNode;
  addLog: (text: string) => void;
  children?: React.ReactNode;
  // Halaman klip (DESIGN.md §4): pratinjau berdiri sendiri di kolom kanan,
  // setelannya di laci "Pengaturan lanjutan". Dua instans, satu sumber state —
  // karena itu kisi tinggal di halaman, bukan di sini.
  part: "preview" | "settings";
  grid: number; setGrid: (v: number) => void;
  alwaysGuides: boolean; setAlwaysGuides: (v: boolean) => void;
  /** Kotak centang "Tampilkan area yang tertutup UI aplikasi". */
  showZones?: boolean;
}) {
  const { t } = useI18n();

  const [previewOn, setPreviewOn] = useState(false);
  const [previewBusy, setPreviewBusy] = useState(false);
  const [duration, setDuration] = useState(0);
  // null = belum diperiksa. false = video tanpa trek suara.
  const [hasAudio, setHasAudio] = useState<boolean | null>(null);
  const [previewTime, setPreviewTime] = useState(5);
  // Dinaikkan tiap muat ulang manual supaya URL frame berubah — kalau URL-nya
  // sama persis, browser memakai gambar lama meski videonya sudah ditimpa.
  const [previewNonce, setPreviewNonce] = useState(0);
  const boxRef = useRef<HTMLDivElement | null>(null);

  // Tinggi & lebar bingkai datang dari CSS dan HANYA dari tinggi jendela
  // (--pv-w di globals.css). Pengukuran kolom setelan yang dulu ada di sini
  // dibuang: lebar kolom itu sendiri diturunkan dari angka yang diukur, jadi
  // setiap kendali baru membuat bingkai tumbuh, kolom menyempit, isinya melipat,
  // dan bingkai tumbuh lagi — pratinjau yang "membesar sendiri".

  // reframe ikut dikirim: preview memakai mode yang sama dengan render, jadi
  // koordinat subtitle diatur di atas geometri yang benar (di mode "muat utuh"
  // videonya cuma mengisi pita tengah, bukan seluruh kanvas).
  const frameUrl = useMemo(
    () => eng(`/api/frame?path=${encodeURIComponent(path)}&t=${previewTime.toFixed(2)}`)
      + `&reframe=${encodeURIComponent(reframe)}&background=${encodeURIComponent(background)}`
      + `&zoom=${zoom}`
      + `&n=${previewNonce}`,
    [path, previewTime, reframe, background, zoom, previewNonce]
  );

  // silent = dipicu otomatis oleh video baru; kegagalannya tidak perlu
  // diteriakkan ke log karena path bisa saja masih setengah diketik.
  const loadPreview = useCallback(async (silent = false) => {
    if (!path) return;
    setPreviewBusy(true);
    try {
      const res = await fetch(eng(`/api/probe?path=${encodeURIComponent(path)}`));
      const text = await res.text();
      let data: any;
      try { data = JSON.parse(text); }
      catch { throw new Error(t("errOldEngine")); }
      if (!res.ok) throw new Error(data.error || t("errReadVideo"));
      setDuration(data.duration || 0);
      setHasAudio(data.has_audio !== false);
      setHasAudio(data.has_audio !== false);
      setPreviewTime(Math.min(5, (data.duration || 10) / 2));
      setPreviewNonce((n) => n + 1);
      setPreviewOn(true);
    } catch (e: any) {
      if (!silent) addLog(t("logPreviewFailed", { error: e.message }));
    } finally { setPreviewBusy(false); }
  }, [path, addLog, t]);

  // Kembali ke keadaan awal: frame dilepas, durasi & waktu direset. Dipakai
  // tombol "reset" dan otomatis saat videonya berganti.
  const resetPreview = useCallback(() => {
    setPreviewOn(false);
    setHasAudio(null);
    setHasAudio(null);
    setPreviewBusy(false);
    setDuration(0);
    setPreviewTime(5);
  }, []);

  // Video baru masuk (unggah, seret-lepas, atau path diketik) → preview lama
  // dibuang lalu dimuat ulang sendiri. Ditunda 500 ms supaya path yang sedang
  // diketik tidak memicu satu permintaan per huruf.
  useEffect(() => {
    if (part !== "preview") return; // instans setelan tidak memuat gambar
    resetPreview();
    if (!path) return;
    const timer = setTimeout(() => loadPreview(true), 500);
    return () => clearTimeout(timer);
  }, [path, resetPreview, loadPreview, part]);

  // Mesin seret ada di ./drag — satu untuk ketiga lapis, dan halaman watermark
  // memakai yang sama.
  const { dragAt, dragProps } = useLayerDrag(boxRef, grid);

  const subDrag = dragProps(
    subX, subY,
    // Batas bawah dikurangi tinggi blok: yang harus tetap di dalam bingkai
    // adalah seluruh blok, bukan cuma titik jangkarnya.
    (x, y) => { setSubX(x); setSubY(y); },
    centerAnchorY, PLAY_H - blockH,
  );
  const wmDrag = dragProps(watermark.x, watermark.y, moveWatermark, CENTER_Y, PLAY_H);
  // Headline: koordinat di layar dihitung dari kotaknya, yang disimpan geseran.
  const hlBox = headlineBox(watermark);
  const hlLines = wrapHeadline(
    watermark.hlSource === "llm" ? t("headlineSample") : watermark.hlText,
    watermark.hlSize, hlBox.w);
  const hlAt = headlineAnchor(hlLines, watermark.hlSize, watermark.hlDX, watermark.hlDY, hlBox);
  const headlineDrag = dragProps(hlAt.x, hlAt.y,
    (x, y) => moveHeadline(x - hlBox.cx, y - hlBox.cy), hlBox.cy, PLAY_H, hlBox.cx);

  const atCenterX = (dragAt?.x ?? subX) === CENTER_X;
  const atCenterY = dragAt ? dragAt.y === CENTER_Y || dragAt.y === centerAnchorY : subY === centerAnchorY;
  const guidesVisible = dragAt !== null || alwaysGuides;

  const maxLines = linesFor(subMode, subSpeed);
  // Piksel CSS per satuan ukuran subtitle, dari engine. 0 = belum tahu (daftar
  // font belum sampai, atau font manual belum lolos cek) — dalam keadaan itu
  // dipakai 1, yang berarti pratinjau menggambar seperti sebelumnya.
  const fontScale =
    (fontManual ? fontCheck?.scale : fonts.find((f) => f.name === subFont)?.scale) || 1;
  const colorHex = hex(subColor);
  const highlightHex = hex(subHighlight);

  if (part === "preview") {
    return (
      <div className="sub-layout pv-only">
      <div className="sub-preview">
        <div className="preview9x16" ref={boxRef}>
          {previewOn ? (
            /* eslint-disable-next-line @next/next/no-img-element */
            <img src={frameUrl} alt="preview" draggable={false} />
          ) : (
            <div className="preview-empty">
              <div className="pe-icon" aria-hidden="true" />
              <div className="pe-title">{t("emptyFrame")}</div>
            </div>
          )}
          {zone && showZones && (
            <>
              <div className="safezone" style={{ top: 0, left: 0, right: 0, height: `${zone.top * 100}%` }}>
                <span>{t("zoneTop")}</span>
              </div>
              <div className="safezone" style={{ bottom: 0, left: 0, right: 0, height: `${zone.bottom * 100}%` }}>
                <span>{t("zoneBottom")}</span>
              </div>
              <div className="safezone" style={{
                top: `${zone.top * 100}%`, bottom: `${zone.bottom * 100}%`,
                right: 0, width: `${zone.right * 100}%`,
              }}>
                <span className="vert">{t("zoneRight")}</span>
              </div>
            </>
          )}
          {/* Garis tengah: muncul saat digeser (atau dikunci lewat centang).
              Kelas "on" = subtitle sedang menempel di garis itu. */}
          <Guides grid={grid} visible={guidesVisible} atX={atCenterX} atY={atCenterY} dragAt={dragAt} />
          {/* Video tanpa suara: seluruh pipeline berdiri di atas ucapan, jadi
              ini harus terlihat SEBELUM tombol Mulai ditekan. Lencana melayang
              di atas gambar — tidak menambah baris, tidak menggeser apa pun. */}
          {hasAudio === false && (
            <div className="no-audio"><Warn width={280}>{t("noAudioWarning")}</Warn>{t("noAudio")}</div>
          )}
          {/* Banner watermark. Digambar SEBELUM subtitle & headline supaya
              urutan tumpukannya sama dengan hasil render: overlay ffmpeg lebih
              dulu, .ass (subtitle + headline) dibakar sesudahnya.

              Sumbernya /api/image, bukan path lokal langsung: <img> di browser
              tidak bisa membuka berkas di disk, dan tanpa gambarnya yang bisa
              digeser cuma kotak kosong. */}
          {watermark.image && (
            /* eslint-disable-next-line @next/next/no-img-element */
            <img className="wmoverlay" alt="watermark" draggable={false}
              src={eng(`/api/image?path=${encodeURIComponent(watermark.image)}`)}
              style={{
                left: `${(watermark.x / PLAY_W) * 100}%`, top: `${(watermark.y / PLAY_H) * 100}%`,
                // KOTAK-nya yang diberi ukuran; object-fit: contain (globals.css)
                // yang memuat gambarnya ke dalam — sama dengan sisi ffmpeg.
                width: `${watermark.width}%`, height: `${watermark.height}%`,
              }}
              {...wmDrag} />
          )}
          {/* Headline duduk DI DALAM kotak watermark: jangkarnya tengah kotak,
              digeser sebanyak hlDX/hlDY, dan dijepit supaya bloknya tidak
              pernah keluar. Batas kotaknya digambar saat dipegang — janji
              "tidak melewati watermark" harus TERLIHAT, bukan cuma berlaku. */}
          {(hlLines.length > 0) && (
            <>
              {dragAt && <div className="wmbox" style={{
                left: `${((hlBox.cx - hlBox.w / 2) / PLAY_W) * 100}%`,
                top: `${((hlBox.cy - hlBox.h / 2) / PLAY_H) * 100}%`,
                width: `${(hlBox.w / PLAY_W) * 100}%`,
                height: `${(hlBox.h / PLAY_H) * 100}%`,
              }} />}
              <div className="suboverlay headlineoverlay"
                style={{
                  left: `${(hlAt.x / PLAY_W) * 100}%`, top: `${(hlAt.y / PLAY_H) * 100}%`,
                  fontFamily: `"${subFont}", sans-serif`,
                  fontSize: `calc(${(watermark.hlSize * fontScale) / PLAY_H} * var(--pvh))`,
                  lineHeight: 1 / fontScale,
                  color: hex(watermark.hlColor),
                  textShadow: watermark.hlOutline > 0
                    ? "-2px -2px 0 #000,2px -2px 0 #000,-2px 2px 0 #000,2px 2px 0 #000,0 0 4px #000"
                    : "none",
                }}
                {...headlineDrag}>
                {hlLines.map((line, i) => <div key={i}>{line}</div>)}
              </div>
            </>
          )}
          <div className="suboverlay"
            style={{
              left: `${(subX / PLAY_W) * 100}%`, top: `${(subY / PLAY_H) * 100}%`,
              fontFamily: `"${subFont}", sans-serif`,
              fontSize: `calc(${(subSize * fontScale) / PLAY_H} * var(--pvh))`,
              // Kotak baris harus setinggi ukuran .ass, sedangkan font-size di
              // atas sudah dikecilkan jadi em-nya. 1/scale mengembalikannya.
              lineHeight: 1 / fontScale,
              color: colorHex,
              background: subBox ? "rgba(0,0,0,0.6)" : "transparent",
              borderRadius: subBox ? 8 : 0,
              padding: subBox ? "4px 14px" : "2px 8px",
              textShadow: subBox ? "none" : (subOutline > 0
                ? "-2px -2px 0 #000,2px -2px 0 #000,-2px 2px 0 #000,2px 2px 0 #000,0 0 4px #000"
                : "none"),
            }}
            {...subDrag}>
            {/* Contoh ditampilkan sebanyak baris terbanyak yang mungkin
                muncul, bukan satu baris pendek. Dulu preview selalu 1 baris
                sehingga pengguna menaruhnya pas, lalu terkejut waktu hasilnya
                2 baris. */}
            {subMode === "word" ? (
              <div style={{ color: highlightHex }}>{t("sampleWord")}</div>
            ) : (
              Array.from({ length: maxLines }, (_, i) => {
                const line = t(SAMPLE_LINES[i]);
                const isLast = i === maxLines - 1;
                if ((subMode !== "karaoke" && subMode !== "highlight") || !isLast) return <div key={i}>{line}</div>;
                // Kata terakhir yang sedang diucapkan: karaoke berganti warna,
                // "sorot kata" berdiri di atas blok warna (sama dengan blockTag
                // di engine/internal/subtitle).
                const words = line.split(" ");
                const tail = words.pop();
                const on = subMode === "highlight"
                  ? { background: highlightHex, color: "#15181D", textShadow: "none", padding: "0 0.18em", borderRadius: "0.12em" }
                  : { color: highlightHex };
                return (
                  <div key={i}>
                    {words.join(" ")} <span style={on}>{tail}</span>
                  </div>
                );
              })
            )}
          </div>
        </div>

      </div>
        <div className="pv-controls">
        {/* Kendali pratinjau tinggal DI SINI, bukan di bawah bingkainya.
            Alasannya tinggi, bukan kerapian: kolom pratinjau dan kotak log
            berbagi satu jatah tinggi, jadi tiap piksel yang dipakai bilah ini
            di bawah bingkai diambil langsung dari kotak log. Kolom setelan
            punya ruang sisa di dasarnya; bilah ini mengisinya. */}
        {/* Kendali preview ditaruh DI BAWAH gambar: tombolnya mengubah gambar
            itu, jadi urutan bacanya lebih masuk akal daripada di atas. */}
        <div className="preview-actions">
          {!previewOn ? (
            <button className="ghost" disabled={!path || previewBusy} onClick={() => loadPreview()}>
              {previewBusy ? t("loadingPreview") : <><Eye className="ico" aria-hidden="true" /> {t("loadPreview")}</>}
            </button>
          ) : (
            <>
              <button className="ghost tiny icon-only" disabled={previewBusy}
                title={t("reloadPreview")} aria-label={t("reloadPreview")} onClick={() => loadPreview()}>
                <RotateCw className="ico" aria-hidden="true" />
              </button>
              <button className="ghost tiny icon-only" onClick={resetPreview}
                title={t("resetPreview")} aria-label={t("resetPreview")}>
                <X className="ico" aria-hidden="true" />
              </button>
            </>
          )}
        </div>

        {/* Penggeser waktu SATU baris: dulu label di atas + penggeser di bawah
            memakan 67 px, dan tiap piksel di kolom ini diambil dari bingkai
            pratinjaunya sendiri. Angkanya pindah ke ujung kanan barisnya. */}
        {/* SELALU dirender, dimatikan sebelum pratinjau termuat: baris ini dulu
            muncul bersama gambar pertama, dan kemunculannya menggeser seluruh
            kolom 22 px tepat saat pengguna sedang menatap gambar itu. */}
        <div className="frame-time" title={t("previewTime", { t: previewTime.toFixed(1) })}>
          <input type="range" min={0} max={Math.max(1, Math.floor(duration))} step={1}
            disabled={!previewOn}
            aria-label={t("previewTime", { t: previewTime.toFixed(1) })}
            value={previewTime} onChange={(e) => setPreviewTime(Number(e.target.value))} />
          <span className="meta">{previewOn ? `${previewTime.toFixed(1)}s` : "–"}</span>
        </div>
        </div>
      </div>
    );
  }
  return (
    <div className="sub-settings adv-settings">
        {/* Kelompok subtitle, penempatan, dan bingkai MENGHILANG selagi watermark
            terbuka. Bukan disembunyikan supaya rapi — kolom ini tidak punya sisa
            tinggi sama sekali, jadi dua kelompok besar tidak bisa berdiri
            bersama tanpa membuatnya bergulir. */}
        {!wmOpen && <>
        <div className="group">
        <div className="group-title" role="heading" aria-level={3}>{t("groupSubtitle")}</div>
        <div className="grid3">
          <div className="field"><label>{t("font")}
            {/* Font manual yang ditolak: lambang di label, bukan kalimat merah
                di bawah kotaknya — kalimat itu panjangnya berubah tiap ketikan
                dan seluruh kisi ikut naik-turun. */}
            {fontManual && fontCheck && !fontCheck.valid && <Warn>{fontCheck.error}</Warn>}</label>
            <Select
              value={fontManual ? "__manual__" : subFont}
              onChange={(v) => {
                if (v === "__manual__") { setFontManual(true); return; }
                setFontManual(false); setSubFont(v);
              }}
              options={[
                ...fonts.map((f) => ({ value: f.name, label: f.name })),
                { value: "__manual__", label: t("fontOther") },
              ]} />
            {fontManual && (
              <>
                <input style={{ marginTop: 6 }} value={subFont} spellCheck={false}
                  placeholder={t("fontPlaceholder")} onChange={(e) => setSubFont(e.target.value)} />
                {/* Satu baris, selalu: apa pun keadaannya tingginya sama. */}
                <div className="meta req-line">
                  {fontChecking ? t("fontChecking")
                    : !fontCheck ? t("fontHint")
                    : fontCheck.valid ? <span className="ok">{t("fontFound", { family: fontCheck.family, source: fontCheck.source })}</span>
                    : fontCheck.error}
                </div>
              </>
            )}</div>
          <div className="field"><label>{t("color")}</label>
            <Select value={subColor} onChange={setSubColor} options={[
              { value: "white", label: t("colorWhite") },
              { value: "yellow", label: t("colorYellow") },
            ]} /></div>
          <div className="field"><label>{t("size")}</label>
            <Stepper value={subSize} onChange={setSubSize} min={40} max={140} step={2} /></div>

          <div className="field"><label>{t("subStyle")} <Tip text={t("subStyleTip")} /></label>
            <Select value={subMode} onChange={setSubMode} options={[
              { value: "normal", label: t("subNormal") },
              { value: "highlight", label: t("subHighlight") },
              { value: "karaoke", label: t("subKaraoke") },
              { value: "word", label: t("subWord") },
            ]} /></div>
          <div className="field"><label>{t("subSpeed")} <Tip text={t("subSpeedTip")} /></label>
            <Select value={subSpeed} onChange={setSubSpeed} options={[
              { value: "slow", label: t("speedSlow") },
              { value: "normal", label: t("speedNormal") },
              { value: "dense", label: t("speedDense") },
            ]} /></div>
          <div className="field"><label>{t("outline")}</label>
            <Stepper value={subOutline} onChange={setSubOutline} min={0} max={12} /></div>
          {/* Warna sorot hanya berarti pada gaya yang menyorot; alasannya
              tertulis di ⓘ, bukan cuma diredupkan. */}
          <div className="field"><label>{t("highlightColor")} {subMode === "normal" && <Tip text={t("highlightWhy")} />}</label>
            <Select value={subHighlight} onChange={setSubHighlight} disabled={subMode === "normal"} options={[
              { value: "#FFD400", label: t("colorYellow") }, { value: "#FFFFFF", label: t("colorWhite") },
              { value: "#55E08A", label: t("colorGreen") }, { value: "#FF8A3D", label: t("colorOrange") },
            ]} /></div>
          <div className="field field-check">
            <label className="chk"><input type="checkbox" checked={subBox}
              onChange={(e) => setSubBox(e.target.checked)} /> {t("boxBackground")}</label>
          </div>
          <GridPicker grid={grid} setGrid={setGrid} always={alwaysGuides} setAlways={setAlwaysGuides} />

        </div>
        </div>

        {/* Penempatan: di mana subtitle berdiri, dan batas mana yang tidak boleh
            ditabrak. Pembatas platform ada DI SINI, bukan di kelompok bingkai —
            ia mengatur ke mana subtitle boleh ditaruh, bukan bagaimana video
            dipasang. */}
        <div className="group">
          <div className="group-title" role="heading" aria-level={3}>{t("groupPlacement")}</div>
          <div className="grid3">
            {/* Peringatan "menabrak zona" jadi LAMBANG di label, bukan baris
                teks di bawah kisi: baris itu muncul dan hilang tiap kali
                subtitle digeser, dan tiap kali itu terjadi kelompok Frame di
                bawahnya melompat. */}
            {/* Peringatan "menabrak zona" jadi LAMBANG di label, bukan baris
                teks di bawah kisi: baris itu muncul dan hilang tiap kali
                subtitle digeser, dan tiap kali itu terjadi kelompok Frame di
                bawahnya melompat. */}
            <PositionField
              label={<>{t("position")}{inUnsafe && <Warn>{t("unsafeWarning")}</Warn>}</>}
              x={subX} y={subY}
              onReset={() => { setSubX(CENTER_X); setSubY(CENTER_Y); }} />
            <div className="field"><label>{t("platformGuide")} <Tip text={t("platformGuideTip")} /></label>
              <Select value={platform} onChange={setPlatform} options={[
                ...Object.keys(PLATFORMS).map((k) => ({
                  value: k, label: k === "generic" ? t("platformGeneric") : PLATFORMS[k].label,
                })),
                { value: "off", label: t("platformNone") },
              ]} /></div>
            <div className="field field-check">
              <button className="ghost" disabled={!zone} onClick={onPlaceSafe}>{t("placeSafe")}</button>
            </div>
          </div>
        </div>

        {children}
        </>}

        {watermarkPanel}
    </div>
  );
}
