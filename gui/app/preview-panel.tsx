"use client";

// Ikon: lucide-react (ISC) — alasannya di gui/app/page.tsx.
import { Eye, RotateCw, X } from "lucide-react";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useI18n } from "./i18n";
import { eng } from "./engine";
import { PLAY_H, PLAY_W, CENTER_X, CENTER_Y } from "./drag";
import Guides from "./guides";
import { MarkLayers, type Mark } from "./mark";
import Warn from "./warn";
import Filmstrip from "./filmstrip";
import { StageFrame, useStageWidth } from "./stage";
import { toHex } from "./color-panel";
import { CaptionWords, NO_FX, captionShadow } from "./caption-presets";
import type { SubFx } from "./caption-presets";

// Area yang tertutup UI tiap aplikasi (fraksi tinggi/lebar frame 9:16).
// Angka awal hasil pengukuran kasar — sesuaikan bila UI aplikasi berubah.
//
// Tinggal di sini bersama pratinjau: pembatas ini
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


// Berapa baris yang mungkin muncul sekaligus — harus sama dengan Pacing() di
// engine (lambat/normal 2, padat 3; mode word selalu 1 kata per layar).
export const linesFor = (subMode: string, subSpeed: string, stack = false) =>
  subMode === "word" ? 1 : stack ? 3 : subSpeed === "dense" ? 3 : 2;

// Bingkai 9:16 + seretan subtitle, watermark, dan headline. Setelannya ada di
// kartu More controls (more-controls.tsx).
export default function PreviewPanel({
  path, reframe, background, zoom, zone,
  fonts, subFont, setSubFont, fontManual, setFontManual, fontCheck, fontChecking,
  subSize, setSubSize, subColor, setSubColor, subOutline, setSubOutline, subBox, setSubBox, subUpper = false, subFx = NO_FX,
  subMode, setSubMode, subHighlight, setSubHighlight, subSpeed, setSubSpeed,
  subX, setSubX, subY, setSubY, blockH, centerAnchorY,
  m, addLog, alwaysGuides, showZones = true, previewTime, setPreviewTime,
}: {
  path: string; reframe: string; background: string; zoom: number; zone?: Zone;
  fonts: Font[];
  subFont: string; setSubFont: (v: string) => void;
  fontManual: boolean; setFontManual: (v: boolean) => void;
  fontCheck: FontCheck | null; fontChecking: boolean;
  subSize: number; setSubSize: (v: number) => void;
  subColor: string; setSubColor: (v: string) => void;
  subOutline: number; setSubOutline: (v: number) => void;
  subBox: boolean; setSubBox: (v: boolean) => void;
  subUpper?: boolean; subFx?: SubFx;
  subMode: string; setSubMode: (v: string) => void;
  subHighlight: string; setSubHighlight: (v: string) => void;
  subSpeed: string; setSubSpeed: (v: string) => void;
  subX: number; setSubX: (v: number | ((p: number) => number)) => void;
  subY: number; setSubY: (v: number | ((p: number) => number)) => void;
  blockH: number; centerAnchorY: number;
  // Mark (logo + judul) yang SAMA dengan halaman Mark & News: lapisannya,
  // bingkai (boxRef), mesin seret & kisinya dipakai juga untuk subtitle.
  m: Mark;
  addLog: (text: string) => void;
  alwaysGuides: boolean;
  /** Kotak centang "Tampilkan area yang tertutup UI aplikasi". */
  showZones?: boolean;
  // Detik pratinjau tinggal di halaman: kartu Source memilihnya juga (filmstrip).
  previewTime: number; setPreviewTime: (t: number) => void;
}) {
  const { t } = useI18n();

  const [previewOn, setPreviewOn] = useState(false);
  const [previewBusy, setPreviewBusy] = useState(false);
  const [duration, setDuration] = useState(0);
  // null = belum diperiksa. false = video tanpa trek suara.
  const [hasAudio, setHasAudio] = useState<boolean | null>(null);
  // Dinaikkan tiap muat ulang manual supaya URL frame berubah — kalau URL-nya
  // sama persis, browser memakai gambar lama meski videonya sudah ditimpa.
  const [previewNonce, setPreviewNonce] = useState(0);

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
      // Kartu berita (Look): bingkai dipotong engine seperti hasil render — sama dengan Mark.
      + (m.cardOn ? `&card=1&card_y=${m.frameCardY}` : "")
      + `&n=${previewNonce}`,
    [path, previewTime, reframe, background, zoom, previewNonce, m.cardOn, m.frameCardY]
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
      setPreviewTime(Math.min(5, (data.duration || 10) / 2));
      setPreviewNonce((n) => n + 1);
      setPreviewOn(true);
    } catch (e: any) {
      if (!silent) addLog(t("logPreviewFailed", { error: e.message }));
    } finally { setPreviewBusy(false); }
  }, [path, addLog, t, setPreviewTime]);

  // Kembali ke keadaan awal: frame dilepas, durasi & waktu direset. Dipakai
  // tombol "reset" dan otomatis saat videonya berganti.
  const resetPreview = useCallback(() => {
    setPreviewOn(false);
    setHasAudio(null);
    setPreviewBusy(false);
    setDuration(0);
    setPreviewTime(5);
  }, [setPreviewTime]);

  // Video baru masuk (unggah, seret-lepas, atau path diketik) → preview lama
  // dibuang lalu dimuat ulang sendiri. Ditunda 500 ms supaya path yang sedang
  // diketik tidak memicu satu permintaan per huruf.
  useEffect(() => {
    resetPreview();
    if (!path) return;
    const timer = setTimeout(() => loadPreview(true), 500);
    return () => clearTimeout(timer);
  }, [path, resetPreview, loadPreview]);

  // Satu mesin seret (useMark → useLayerDrag) untuk subtitle, logo, dan judul.
  const { dragAt, dragProps, boxRef } = m;
  const frameH = (useStageWidth() * 16) / 9;

  const subDrag = dragProps(
    subX, subY,
    // Batas bawah dikurangi tinggi blok: yang harus tetap di dalam bingkai
    // adalah seluruh blok, bukan cuma titik jangkarnya.
    (x, y) => { setSubX(x); setSubY(y); },
    centerAnchorY, PLAY_H - blockH,
  );

  const atCenterX = (dragAt?.x ?? subX) === CENTER_X;
  const atCenterY = dragAt ? dragAt.y === CENTER_Y || dragAt.y === centerAnchorY : subY === centerAnchorY;
  const guidesVisible = dragAt !== null || alwaysGuides;

  const maxLines = linesFor(subMode, subSpeed, subFx.stack);
  // Piksel CSS per satuan ukuran subtitle, dari engine. 0 = belum tahu (daftar
  // font belum sampai, atau font manual belum lolos cek) — dalam keadaan itu
  // dipakai 1, yang berarti pratinjau menggambar seperti sebelumnya.
  const fontScale =
    (fontManual ? fontCheck?.scale : fonts.find((f) => f.name === subFont)?.scale) || 1;
  const colorHex = toHex(subColor);

  return (
      <div className="sub-layout pv-only">
      <div className="sub-preview">
        <StageFrame>
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
          <Guides grid={m.grid} visible={guidesVisible} atX={atCenterX} atY={atCenterY} dragAt={dragAt} />
          {/* Video tanpa suara: seluruh pipeline berdiri di atas ucapan, jadi
              ini harus terlihat SEBELUM tombol Mulai ditekan. Lencana melayang
              di atas gambar — tidak menambah baris, tidak menggeser apa pun. */}
          {hasAudio === false && (
            <div className="no-audio"><Warn width={280}>{t("noAudioWarning")}</Warn>{t("noAudio")}</div>
          )}
          {/* Logo & judul Mark (lapisan yang sama dengan halaman Mark & News),
              digambar SEBELUM subtitle — urutan tumpukannya sama dengan render. */}
          <MarkLayers m={m} frameH={frameH} />
          <div className="suboverlay"
            style={{
              left: `${(subX / PLAY_W) * 100}%`, top: `${(subY / PLAY_H) * 100}%`,
              fontFamily: `"${subFont}", sans-serif`,
              fontSize: `calc(${(subSize * fontScale) / PLAY_H} * var(--pvh))`,
              // Kotak baris harus setinggi ukuran .ass, sedangkan font-size di
              // atas sudah dikecilkan jadi em-nya. 1/scale mengembalikannya.
              lineHeight: 1 / fontScale,
              color: colorHex,
              textTransform: subUpper ? "uppercase" : "none",
              background: subBox ? "rgba(0,0,0,0.6)" : "transparent",
              borderRadius: subBox ? 8 : 0,
              padding: subBox ? "4px 14px" : "2px 8px",
              textShadow: captionShadow({ size: subSize, outline: subOutline, box: subBox, fx: subFx }),
            }}
            {...subDrag}>
            {/* Contoh ditampilkan sebanyak baris terbanyak yang mungkin
                muncul, bukan satu baris pendek. Dulu preview selalu 1 baris
                sehingga pengguna menaruhnya pas, lalu terkejut waktu hasilnya
                2 baris. */}
            {/* Aturan mode, gerak & bayangan = kotak contoh preset (CaptionWords). */}
            <CaptionWords lines={Array.from({ length: maxLines }, (_, i) => t(SAMPLE_LINES[i]).split(" "))}
              s={{ font: subFont, size: subSize, color: subColor, highlight: subHighlight, outline: subOutline, box: subBox, mode: subMode, upper: subUpper, fx: subFx }} />
          </div>
        </div>
        </StageFrame>

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
        {path && <Filmstrip className="pv-film" path={path} duration={previewOn ? duration : 0}
          time={previewTime} onPick={setPreviewTime} />}
        </div>
      </div>
    );
  }

