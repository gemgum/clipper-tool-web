"use client";

// Tab watermark: logo + judul untuk video yang SUDAH jadi (DESIGN-Clipper-Lanjutan §4).
//
// Halaman klip memotong lalu membakar identitas; halaman ini hanya membakar.
// Koordinat mentah diganti tarikan langsung: logo dan judul diseret di
// pratinjau, sembilan titik jangkar jadi jalan cepat, dan ukuran logo satu
// angka yang menjaga proporsinya.
//
// Ikon: lucide-react (ISC). Tanpa satu emoji pun — alasannya di gui/app/page.tsx.

import { useCallback, useEffect, useRef, useState } from "react";
import PageHeader from "../page-header";
import { Film, Folder, Plus, X, ImageIcon } from "lucide-react";

import { eng, isWeb, upload, useWeb } from "../engine";
import { useI18n } from "../i18n";
import Alerts from "../alerts";
import { DEFAULT_WATERMARK, watermarkToAPI, watermarkOn, headlineAnchor, headlineBox, wrapHeadline } from "../watermark-model";
import type { Watermark } from "../watermark-model";
import { CENTER_X, CENTER_Y, PLAY_H, PLAY_W, useLayerDrag } from "../drag";
import type { Font } from "../preview-panel";
import Guides from "../guides";
import { Segmented } from "../clip-steps";
import LogPanel from "../log-panel";
import Picker from "../picker";
import Select from "../select";
import Stepper from "../stepper";
import { useKeep, useRestore } from "../persist";

type FileResult = { video: string; name: string; output?: string; seconds?: number; error?: string };
type WatermarkJob = {
  id: string;
  status: string;
  stage: string;
  progress: number;
  log?: string[];
  error?: string;
  result?: { files: FileResult[] };
};
type Probe = { w: number; h: number; dur: number } | "error";

const baseName = (p: string) => p.split(/[\\/]/).pop() || p;
const mmss = (s: number) => `${Math.floor(s / 60)}:${String(Math.floor(s % 60)).padStart(2, "0")}`;

// Font judul. Satu untuk seluruh aplikasi — pemilihnya ada di halaman klip,
// dan pemilih kedua berarti pengukuran kedua (notes/29).
const HL_FONT = "Montserrat";
const COLORS: Record<string, string> = { white: "#ffffff", yellow: "#ffd400", black: "#15181d" };
// Kecil / Sedang / Besar dalam satuan bingkai 1080x1920.
const SIZES = [48, 64, 88];
// Garis tepi dihitung dari ukuran, bukan disetel: makin besar huruf, makin tebal tepinya.
const outlineFor = (size: number) => Math.max(2, Math.round(size / 20));
// Sama dengan aspectTolerance di engine/internal/watermark.
const is916 = (p: Probe | undefined) => !!p && p !== "error" && Math.abs(p.w / p.h - 9 / 16) <= 0.02;

// Titik tengah logo untuk jangkar 0..8 (baris lalu kolom), menurut tabel §4:
// tepi kiri/kanan 5%, atas 4%, tengah mulai 46%, bawah 16% dari dasar.
function anchorPoint(i: number, wPx: number, hPx: number) {
  const col = i % 3, row = Math.floor(i / 3);
  const x = col === 0 ? 0.05 * PLAY_W + wPx / 2 : col === 1 ? CENTER_X : PLAY_W - 0.05 * PLAY_W - wPx / 2;
  const y = row === 0 ? 0.04 * PLAY_H + hPx / 2 : row === 1 ? 0.46 * PLAY_H + hPx / 2 : PLAY_H - 0.16 * PLAY_H - hPx / 2;
  return { x: Math.round(x), y: Math.round(y) };
}

export default function WatermarkPage() {
  const { t } = useI18n();

  const [videos, setVideos] = useState<string[]>([]);
  const [probes, setProbes] = useState<Record<string, Probe>>({});
  // Mode web (notes/42): video & logo DIUNGGAH lewat input berkas.
  const web = useWeb();
  const fileInput = useRef<HTMLInputElement>(null);
  const logoInput = useRef<HTMLInputElement>(null);
  const [upNote, setUpNote] = useState("");
  const [paste, setPaste] = useState("");
  const [picking, setPicking] = useState<"" | "file" | "folder" | "out" | "logo">("");
  const [outDir, setOutDir] = useState("");
  const [quality, setQuality] = useState("hd");
  const [dragOver, setDragOver] = useState(false);

  const [watermark, setWatermarkState] = useState<Watermark>(DEFAULT_WATERMARK);
  const setWatermark = useCallback((patch: Partial<Watermark>) => {
    setWatermarkState((b) => ({ ...b, ...patch }));
  }, []);
  const [logoOn, setLogoOn] = useState(true);
  // Jangkar terpilih; -1 = posisi bebas hasil seretan.
  const [anchor, setAnchor] = useState(2);
  const [timed, setTimed] = useState(false);
  // Rasio lebar/tinggi logo, dibaca dari gambarnya sendiri saat dimuat.
  const [logoDims, setLogoDims] = useState<{ w: number; h: number } | null>(null);
  const aspect = logoDims ? logoDims.w / logoDims.h : 1;

  // Satu angka ukuran (persen lebar bingkai); tinggi kotaknya diturunkan dari
  // rasio logo supaya kotaknya pas dengan gambarnya.
  const heightFor = (width: number, a: number) =>
    Math.min(100, Math.max(5, Math.round((width * PLAY_W) / a / PLAY_H)));
  const place = useCallback((i: number, width: number, a: number) => {
    const h = heightFor(width, a);
    const p = anchorPoint(i, (PLAY_W * width) / 100, (PLAY_H * h) / 100);
    setWatermarkState((b) => ({ ...b, width, height: h, x: p.x, y: p.y }));
  }, []);
  const setSize = (width: number) => {
    if (anchor >= 0) place(anchor, width, aspect);
    else setWatermark({ width, height: heightFor(width, aspect) });
  };
  // Rasio baru (logo diganti) = kotak & jangkar dihitung ulang.
  useEffect(() => {
    if (!logoDims) return;
    if (anchor >= 0) place(anchor, watermark.width, aspect);
    else setWatermark({ height: heightFor(watermark.width, aspect) });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [logoDims]);

  // Yang dipratinjau & dikirim: logo hanya bila "Pakai logo" menyala, judul
  // selalu bebas di bingkai, tepi dihitung otomatis.
  const eff: Watermark = {
    ...watermark,
    image: logoOn ? watermark.image : "",
    hlFree: true, hlSource: "text",
    hlOutline: outlineFor(watermark.hlSize),
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

  const [jobId, setJobId] = useState("");
  const [job, setJob] = useState<WatermarkJob | null>(null);
  const [logs, setLogs] = useState<string[]>([]);
  const [error, setError] = useState("");
  const busy = job?.status === "running" || (!!jobId && !job);

  const boxRef = useRef<HTMLDivElement | null>(null);
  // Kisi 10 tetap menempelkan seretan, tapi tidak digambar (Guides: < 20).
  const { dragAt, dragProps } = useLayerDrag(boxRef, 10);
  const wmDrag = dragProps(eff.x, eff.y, (x, y) => { setAnchor(-1); setWatermark({ x, y }); }, CENTER_Y, PLAY_H);
  const hlBox = headlineBox(eff);
  const hlLines = wrapHeadline(eff.hlText, eff.hlSize, hlBox.w);
  const hlAt = headlineAnchor(hlLines, eff.hlSize, eff.hlDX, eff.hlDY, hlBox);
  const headlineDrag = dragProps(hlAt.x, hlAt.y,
    (x, y) => setWatermark({ hlDX: x - hlBox.cx, hlDY: y - hlBox.cy }), hlBox.cy, PLAY_H, hlBox.cx);

  // Pratinjau memakai video 9:16 PERTAMA: watermark-nya sama untuk semuanya.
  const first = videos.find((v) => is916(probes[v])) || "";
  const frame = first ? eng(`/api/frame?path=${encodeURIComponent(first)}&t=1&reframe=center&background=black&zoom=100`) : "";

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
  useKeep("watermark", { videos, outDir, quality, watermark, logoOn, anchor, timed });
  useRestore<Record<string, unknown>>("watermark", (v) => {
    if (Array.isArray(v.videos)) setVideos(v.videos as string[]);
    if (typeof v.outDir === "string") setOutDir(v.outDir);
    if (typeof v.quality === "string") setQuality(v.quality);
    if (typeof v.logoOn === "boolean") setLogoOn(v.logoOn);
    if (typeof v.anchor === "number") setAnchor(v.anchor);
    if (typeof v.timed === "boolean") setTimed(v.timed);
    if (v.watermark && typeof v.watermark === "object") {
      const w = { ...DEFAULT_WATERMARK, ...(v.watermark as Partial<Watermark>), hlSource: "text" as const };
      // Ukuran judul dari versi lama dibulatkan ke tingkat terdekat.
      w.hlSize = SIZES.reduce((a, b) => (Math.abs(b - w.hlSize) < Math.abs(a - w.hlSize) ? b : a));
      if (!(w.hlColor in COLORS)) w.hlColor = "white";
      setWatermarkState(w);
    }
  });

  // Satu langganan SSE untuk seluruh halaman — membakar video panjang itu
  // hitungan menit per berkas.
  useEffect(() => {
    const es = new EventSource(eng("/api/watermark/events"));
    es.addEventListener("watermark", (ev) => {
      const j: WatermarkJob = JSON.parse((ev as MessageEvent).data);
      setJobId((cur) => {
        if (cur && j.id !== cur) return cur;
        setJob(j);
        if (j.log) setLogs(j.log);
        if (j.error) setError(j.error);
        // Mode web: engine menghapus video unggahan begitu job sukses.
        if (j.status === "done") {
          const done = (j.result?.files ?? []).filter((f) => !f.error).map((f) => f.video);
          isWeb().then((w) => w && setVideos((v) => v.filter((x) => !done.includes(x))));
        }
        return cur || j.id;
      });
    });
    return () => es.close();
  }, []);

  const add = useCallback((paths: string[]) => {
    setVideos((cur) => {
      const out = [...cur];
      for (const p of paths) {
        // Kutip pembungkus dari "Copy as path" di Explorer dibuang.
        const path = p.trim().replace(/^["']|["']$/g, "").trim();
        if (path && !out.includes(path)) out.push(path);
      }
      return out;
    });
  }, []);

  const addFolder = useCallback(async (dir: string) => {
    try {
      const r = await fetch(eng(`/api/browse?dir=${encodeURIComponent(dir)}`));
      const data = await r.json();
      if (!r.ok) throw new Error(data.error || "browse failed");
      const found = (data.entries || []).filter((e: { video?: boolean }) => e.video)
        .map((e: { path: string }) => e.path);
      if (!found.length) { setError(t("capFolderEmpty")); return; }
      add(found);
    } catch (e) {
      setError(String(e));
    }
  }, [add, t]);

  // Berkas yang dilepas TIDAK diunggah di desktop: engine ditanya di mana
  // berkasnya (notes/24). Mode web mengunggahnya.
  const dropFiles = useCallback(async (files: FileList | File[]) => {
    const web = await isWeb();
    for (const f of Array.from(files)) {
      if (web) {
        try {
          add([await upload(f, (x) => setUpNote(`${f.name}: ${t("uploadingPct", { pct: Math.round(x * 100) })}`))]);
        } catch (e) { setError(`${f.name}: ${String(e)}`); }
        setUpNote("");
        continue;
      }
      try {
        const r = await fetch(eng("/api/locate"), {
          method: "POST", headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ name: f.name, size: f.size }),
        });
        const data = await r.json();
        if (r.ok && data.path) { add([data.path]); continue; }
      } catch { /* engine mati: dilaporkan di bawah */ }
      setError(`${f.name}: ${t("capFailed")}`);
    }
  }, [add, t]);

  const pickLogo = async (f: File) => {
    try {
      const p = await upload(f, (x) => setUpNote(`${f.name}: ${t("uploadingPct", { pct: Math.round(x * 100) })}`));
      setWatermark({ image: p }); setLogoOn(true);
    } catch (e) { setError(`${f.name}: ${String(e)}`); }
    setUpNote("");
  };

  // Yang dikirim hanya video 9:16; sisanya ditandai di daftar dan dilewati.
  const ready = videos.filter((v) => is916(probes[v]));
  const nothing = !watermarkOn(eff);

  const start = async () => {
    setError(""); setLogs([]); setJob(null); setJobId("");
    try {
      const r = await fetch(eng("/api/watermark"), {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          videos: ready, quality, out_dir: outDir,
          font: HL_FONT,
          watermark: watermarkToAPI(eff, HL_FONT),
        }),
      });
      const data = await r.json();
      if (!r.ok) throw new Error(data?.error || data?.message || "failed to start");
      setJobId(data.id);
    } catch (e) {
      setError(String(e));
    }
  };

  const cancel = async () => {
    if (!jobId) return;
    try {
      const r = await fetch(eng(`/api/watermark/${jobId}/cancel`), {
        method: "POST", headers: { "Content-Type": "application/json" }, body: "{}",
      });
      if (!r.ok) throw new Error(await r.text());
    } catch (e) {
      setError(String(e));
    }
  };

  const files = job?.result?.files ?? [];
  const resultOf = (v: string) => files.findIndex((f) => f.video === v);
  const pct = Math.round((job?.progress ?? 0) * 100);
  const logoName = watermark.image ? baseName(watermark.image) : "";
  const logoURL = watermark.image ? eng(`/api/image?path=${encodeURIComponent(watermark.image)}`) : "";
  const onLogoLoad = (e: React.SyntheticEvent<HTMLImageElement>) => {
    const { naturalWidth: w, naturalHeight: h } = e.currentTarget;
    if (w && h && (w !== logoDims?.w || h !== logoDims?.h)) setLogoDims({ w, h });
  };
  const outline = COLORS[eff.hlColor === "black" ? "white" : "black"];
  const o = 2;

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

      <div className="cap-body">
        {/* KIRI: pratinjau yang bisa dipegang. */}
        <section className="wm-stage">
          <div className="preview9x16 wm-frame" ref={boxRef}>
            {frame ? (
              /* eslint-disable-next-line @next/next/no-img-element */
              <img src={frame} alt="" draggable={false} />
            ) : (
              <div className="preview-empty">
                <div className="pe-icon" aria-hidden="true" />
                <div className="pe-title">{t("wmEmptyFrame")}</div>
              </div>
            )}
            <Guides grid={10} visible={dragAt !== null}
              atX={(dragAt?.x ?? eff.x) === CENTER_X} atY={(dragAt?.y ?? eff.y) === CENTER_Y} dragAt={dragAt} />
            {eff.image && (
              /* eslint-disable-next-line @next/next/no-img-element */
              <img className="wmoverlay wm-grab" alt={t("wmLogoTitle")} draggable={false} src={logoURL} onLoad={onLogoLoad}
                style={{
                  left: `${(eff.x / PLAY_W) * 100}%`, top: `${(eff.y / PLAY_H) * 100}%`,
                  width: `${eff.width}%`, height: `${eff.height}%`,
                }}
                {...wmDrag} />
            )}
            {hlLines.length > 0 && (
              <div className="suboverlay headlineoverlay wm-grab"
                style={{
                  left: `${(hlAt.x / PLAY_W) * 100}%`, top: `${(hlAt.y / PLAY_H) * 100}%`,
                  fontFamily: `"${HL_FONT}", sans-serif`,
                  fontSize: `calc(${(eff.hlSize * fontScale) / PLAY_H} * var(--pvh))`,
                  lineHeight: 1 / fontScale,
                  color: COLORS[eff.hlColor],
                  textShadow: `-${o}px -${o}px 0 ${outline},${o}px -${o}px 0 ${outline},-${o}px ${o}px 0 ${outline},${o}px ${o}px 0 ${outline}`,
                }}
                {...headlineDrag}>
                {hlLines.map((line, i) => <div key={i}>{line}</div>)}
              </div>
            )}
            <div className="wm-drag-hint" aria-hidden="true">{t("wmDragHint")}</div>
          </div>
          <p className="meta">{t("wmFrameMeta")}</p>
        </section>

        {/* KANAN: logo, judul, waktu, lalu daftar video. */}
        <section className="cap-results">
          <div className="card">
            <div className="card-head">
              <h2>{t("wmLogoTitle")}</h2>
              <label className="wm-check">
                <input type="checkbox" checked={logoOn} onChange={(e) => setLogoOn(e.target.checked)} />
                {t("wmUseLogo")}
              </label>
            </div>
            {web && (
              <input ref={logoInput} type="file" accept="image/png,image/*" hidden
                onChange={(e) => { const f = e.target.files?.[0]; e.target.value = ""; if (f) pickLogo(f); }} />
            )}
            {watermark.image ? (
              <div className="wm-logo-row">
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img className="wm-logo-thumb" src={logoURL} alt="" onLoad={onLogoLoad} />
                <div className="grow">
                  <p className="hi-title">{logoName}</p>
                  {logoDims && <p className="meta">{logoDims.w} × {logoDims.h}</p>}
                </div>
                <button type="button" className="ghost" onClick={() => (web ? logoInput.current?.click() : setPicking("logo"))}>{t("wmChange")}</button>
              </div>
            ) : (
              <button type="button" className="ghost cap-add wm-logo-empty" onClick={() => (web ? logoInput.current?.click() : setPicking("logo"))}>
                <ImageIcon className="ico" aria-hidden="true" /> {t("wmPickLogo")}
              </button>
            )}
            {!watermark.image && <p className="meta cap-hint">{t("wmLogoHint")}</p>}
            {upNote && <p className="meta">{upNote}</p>}

            {watermark.image && logoOn && (
              <div className="wm-place">
                <div>
                  <p className="step-label">{t("wmPlaceAt")}</p>
                  <div className="wm-anchors" role="group" aria-label={t("wmPlaceAt")}>
                    {Array.from({ length: 9 }, (_, i) => (
                      <button key={i} type="button" className={anchor === i ? "on" : ""} aria-pressed={anchor === i}
                        aria-label={t(`wmAnchor${i}` as "wmAnchor0")} title={t(`wmAnchor${i}` as "wmAnchor0")}
                        onClick={() => { setAnchor(i); place(i, watermark.width, aspect); }} />
                    ))}
                  </div>
                </div>
                <div className="grow">
                  <p className="step-label">{t("wmLogoSize")}</p>
                  <Stepper value={watermark.width} onChange={setSize} min={5} max={60} step={1} suffix="%" />
                  <p className="meta cap-hint">{t("wmLogoSizeHint")}</p>
                </div>
              </div>
            )}
          </div>

          <div className="card">
            <h2>{t("wmTitleTitle")}</h2>
            <label className="step-label wr-gap" htmlFor="wm-title">{t("wmTitleText")}</label>
            <input id="wm-title" value={watermark.hlText} spellCheck={false} placeholder={t("wmTitlePlaceholder")}
              onChange={(e) => setWatermark({ hlText: e.target.value })} />
            <div className="wm-row">
              <div className="grow">
                <p className="step-label">{t("wmTextSize")}</p>
                <Segmented label={t("wmTextSize")} value={watermark.hlSize} onChange={(v) => setWatermark({ hlSize: v })}
                  options={[{ value: SIZES[0], name: t("wmSmall") }, { value: SIZES[1], name: t("wmMedium") }, { value: SIZES[2], name: t("wmLarge") }]} />
              </div>
              <div className="grow">
                <p className="step-label">{t("wmTextColor")}</p>
                <div className="wm-swatches" role="group" aria-label={t("wmTextColor")}>
                  {(["white", "yellow", "black"] as const).map((c) => {
                    const name = t(c === "white" ? "colorWhite" : c === "yellow" ? "colorYellow" : "colorBlack");
                    return <button key={c} type="button" className={watermark.hlColor === c ? "on" : ""} aria-pressed={watermark.hlColor === c}
                      aria-label={name} title={name} style={{ background: COLORS[c] }} onClick={() => setWatermark({ hlColor: c })} />;
                  })}
                </div>
              </div>
            </div>
          </div>

          <div className="card">
            <h2>{t("wmWhenTitle")}</h2>
            <label className="wm-radio">
              <input type="radio" name="wm-when" checked={!timed} onChange={() => setTimed(false)} />
              <span>{t("wmWhole")}</span>
            </label>
            <label className="wm-radio">
              <input type="radio" name="wm-when" checked={timed} onChange={() => { setTimed(true); if (!watermark.dur) setWatermark({ dur: 5 }); }} />
              <span>
                {t("wmFromA")}
                <input type="number" min={0} value={watermark.at} aria-label={t("wmFromA")} onFocus={() => setTimed(true)}
                  onChange={(e) => setWatermark({ at: Math.max(0, Number(e.target.value) || 0) })} />
                {t("wmFromB")}
                <input type="number" min={1} value={watermark.dur || 5} aria-label={t("wmFromC")} onFocus={() => setTimed(true)}
                  onChange={(e) => setWatermark({ dur: Math.max(1, Number(e.target.value) || 1) })} />
                {t("wmFromC")}
              </span>
            </label>
          </div>

          <div className={"card cap-drop" + (dragOver ? " over" : "")}
            onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
            onDragLeave={() => setDragOver(false)}
            onDrop={(e) => { e.preventDefault(); setDragOver(false); if (e.dataTransfer.files?.length) dropFiles(e.dataTransfer.files); }}>
            <div className="card-head">
              <h2>{t("wmVideosTitle")}</h2>
              <span className="meta">{videos.length === 1 ? t("capFilesOne") : t("capFiles", { n: videos.length })}</span>
            </div>
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
                const info = p && p !== "error" ? `${mmss(p.dur)} · ${p.w}×${p.h}` : "";
                const [tag, cls] = r ? (r.error ? [t("wmFailedTag"), "bad"] : [t("wmDone"), "ok"])
                  : !p ? [t("wmChecking"), ""] : is916(p) ? [t("wmReady"), "ok"] : [t("wmNot916"), "bad"];
                return (
                  <li key={v}>
                    <Film className="ico" aria-hidden="true" />
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
            {web ? (
              <>
                <input ref={fileInput} type="file" accept="video/*" multiple hidden
                  onChange={(e) => { const fs = Array.from(e.target.files ?? []); e.target.value = ""; if (fs.length) dropFiles(fs); }} />
                <button type="button" className="ghost cap-add wr-gap" disabled={!!upNote} onClick={() => fileInput.current?.click()}>
                  <Plus className="ico" aria-hidden="true" /> {t("capAddVideo")}
                </button>
              </>
            ) : (
              <>
                <div className="cap-add-row">
                  <button type="button" className="ghost cap-add" onClick={() => setPicking("file")}><Plus className="ico" aria-hidden="true" /> {t("capAddVideo")}</button>
                  <button type="button" className="ghost" onClick={() => setPicking("folder")}><Folder className="ico" aria-hidden="true" /> {t("capPickFolder")}</button>
                </div>
                <div className="path-row wr-gap">
                  <input value={paste} onChange={(e) => setPaste(e.target.value)} aria-label={t("capPastePlaceholder")}
                    onKeyDown={(e) => { if (e.key === "Enter") { add(paste.split("\n")); setPaste(""); } }} placeholder={t("capPastePlaceholder")} />
                  <button type="button" className="ghost" disabled={!paste.trim()} onClick={() => { add(paste.split("\n")); setPaste(""); }}>{t("capAdd")}</button>
                </div>
              </>
            )}
            <p className="meta cap-hint">{t("wmVideosNote")}</p>
          </div>

          <details className="card adv">
            <summary>{t("advancedTitle")}</summary>
            <label className="step-label wr-gap" title={t("wmQualityTip")}>{t("quality")}</label>
            <Select value={quality} onChange={setQuality} disabled={busy} options={[
              { value: "draft", label: t("qualityDraft") },
              { value: "hd", label: t("qualityHd") },
              { value: "max", label: t("qualityMax") },
            ]} />
            {!web && (
              <>
                <label className="step-label wr-gap" htmlFor="wm-out">{t("outputDir")}</label>
                <div className="path-row">
                  <input id="wm-out" value={outDir} onChange={(e) => setOutDir(e.target.value)} placeholder={t("wmOutPlaceholder")} disabled={busy} />
                  <button type="button" className="ghost" onClick={() => setPicking("out")}>{t("pickerGo")}…</button>
                </div>
              </>
            )}
          </details>
        </section>
      </div>

      <div className="wr-tech">
        <details className="card tech">
          <summary>{t("techNotes")}</summary>
          <LogPanel logs={logs} />
        </details>
      </div>

      {picking && (
        <Picker
          mode={picking === "folder" || picking === "out" ? "folder" : "file"}
          start={picking === "logo" ? watermark.image : undefined}
          onPick={(p) => {
            if (picking === "folder") addFolder(p);
            else if (picking === "out") setOutDir(p);
            else if (picking === "logo") { setWatermark({ image: p }); setLogoOn(true); }
            else add([p]);
            setPicking("");
          }}
          onClose={() => setPicking("")}
        />
      )}
    </div>
  );
}
