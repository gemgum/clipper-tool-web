"use client";

// Kartu "More controls" (rancangan Video clips, 14 Oktober 2026): deretan ikon;
// satu ketukan membuka SATU panel di bawahnya, ketukan kedua menutupnya.
//
// Sejak laci "Advanced settings" dibuang, SEMUA setelan lanjutan tinggal di
// sini — tidak ada tempat kedua. Ikon "Text" dan "Position" ditambahkan untuk
// isi laci yang tidak punya padanan di rancangan. Dua ikon rancangan disesuaikan
// dengan yang benar-benar ada di engine:
//   "Auto zoom"  zoom yang mengikuti ucapan tidak ada — ikonnya memegang
//                pembingkaian sungguhan (Fit, Zoom, Background);
//   AI engine    hanya ditampilkan: mesin dipilih sekali di Settings (notes/39).

import {
  AudioLines, Cpu, Crosshair, Download, Droplet, Film, Monitor, Move, PenLine, Type, TypeOutline, ZoomIn,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useI18n } from "./i18n";
import type { MessageKey } from "./i18n";
import { eng } from "./engine";
import Select from "./select";
import Stepper from "./stepper";
import Warn from "./warn";
import { Segmented, Swatches, Toggle } from "./clip-steps";
import { useEngines } from "./engine-picker";
import { sameModel, useOllama } from "./ollama";
import { GRIDS } from "./guides";
import { PLATFORMS } from "./preview-panel";
import type { Font, FontCheck } from "./preview-panel";
import WatermarkPanel from "./watermark-panel";
import type { Watermark } from "./watermark-model";

export type WhisperModel = { name: string; size: string; downloaded: boolean };

// Zoom dibaca RELATIF terhadap titik awal modenya. Harus sama dengan engine/config.
//   fit    : mulai 0 (seluruh video masuk) → hanya bisa NAIK
//   center : mulai 100 (memenuhi bingkai)   → hanya bisa TURUN
export const ZOOM_MAX = 100, ZOOM_STEP = 5;
const ZOOM_BOUNDS: Record<string, { min: number; natural: number }> = {
  fit: { min: 0, natural: 0 },
  center: { min: 5, natural: 100 },
};
export const zoomBounds = (mode: string) => ZOOM_BOUNDS[mode] ?? ZOOM_BOUNDS.center;

type Key = "transcription" | "ai" | "resolution" | "fps" | "font" | "text" | "outline" | "position" | "zoom" | "watermark";
const ITEMS: { key: Key; icon: LucideIcon; label: MessageKey; desc: MessageKey }[] = [
  { key: "transcription", icon: AudioLines, label: "mcTranscription", desc: "mcTranscriptionDesc" },
  { key: "ai", icon: Cpu, label: "mcAI", desc: "mcAIDesc" },
  { key: "resolution", icon: Monitor, label: "resolution", desc: "mcResolutionDesc" },
  { key: "fps", icon: Film, label: "fps", desc: "mcFpsDesc" },
  { key: "font", icon: Type, label: "font", desc: "mcFontDesc" },
  { key: "text", icon: TypeOutline, label: "mcText", desc: "mcTextDesc" },
  { key: "outline", icon: PenLine, label: "outline", desc: "mcOutlineDesc" },
  { key: "position", icon: Move, label: "position", desc: "mcPositionDesc" },
  { key: "zoom", icon: ZoomIn, label: "mcFraming", desc: "mcZoomDesc" },
  { key: "watermark", icon: Droplet, label: "groupWatermark", desc: "mcWatermarkDesc" },
];
// Tebal garis tepi sebagai tiga pilihan; angka lain = Custom (angka persisnya di panel Text).
const OUTLINES = [{ v: 3, k: "mcThin" }, { v: 5, k: "mcMedium" }, { v: 8, k: "mcThick" }] as const;

export default function MoreControls(p: {
  disabled?: boolean;
  addLog: (text: string) => void;
  web: boolean | null;
  // transkripsi & mesin
  model: string; setModel: (v: string) => void; models: WhisperModel[]; transcriber: string;
  engine: string; setEngine: (v: string) => void;
  llmModel: string; setLlmModel: React.Dispatch<React.SetStateAction<string>>;
  transcriptFix: boolean; setTranscriptFix: (v: boolean) => void;
  terms: string; setTerms: (v: string) => void;
  // keluaran
  resolution: string; setResolution: (v: string) => void;
  quality: string; setQuality: (v: string) => void;
  fps: number; setFps: (v: number) => void;
  outputDir: string; setOutputDir: (v: string) => void; onPickOut: () => void;
  // huruf
  fonts: Font[]; subFont: string; setSubFont: (v: string) => void;
  fontManual: boolean; setFontManual: (v: boolean) => void; fontCheck: FontCheck | null; fontChecking: boolean;
  subColor: string; setSubColor: (v: string) => void;
  subSize: number; setSubSize: (v: number) => void;
  subOutline: number; setSubOutline: (v: number) => void;
  subSpeed: string; setSubSpeed: (v: string) => void;
  subBox: boolean; setSubBox: (v: boolean) => void;
  subMode: string; setSubMode: (v: string) => void;
  // posisi
  subX: number; subY: number; centerAnchorY: number; onCentre: () => void;
  platform: string; setPlatform: (v: string) => void;
  inUnsafe: boolean; hasZone: boolean; onPlaceSafe: () => void;
  grid: number; setGrid: (v: number) => void;
  alwaysGuides: boolean; setAlwaysGuides: (v: boolean) => void;
  // bingkai
  reframe: string; setReframe: (v: string) => void;
  background: string; setBackground: (v: string) => void;
  zoom: number; setZoom: (v: number) => void;
  // watermark
  watermark: Watermark; setWatermark: (patch: Partial<Watermark>) => void; onPickImage: () => void;
}) {
  const { t } = useI18n();
  const { engines } = useEngines();
  const [open, setOpen] = useState<Key | null>(null);
  const item = ITEMS.find((i) => i.key === open);
  const bounds = zoomBounds(p.reframe);
  const engineName = p.engine === "heuristic" ? t("offlineHeuristic") : (engines.find((e) => e.id === p.engine)?.name || p.engine);

  // --- Ollama: dipindah utuh dari panel laci lama. Hidup di badan komponen,
  // bukan di dalam panel, sebab efeknya harus jalan walau panelnya tertutup. ---
  const { engine, llmModel, setLlmModel, setEngine, setTranscriptFix, addLog } = p;
  const [pulling, setPulling] = useState(false);
  const [pullPct, setPullPct] = useState(-1);
  const ollamaActive = engine === "ollama";
  const { status: ollamaStatus, installed: ollamaInstalled, check: checkOllama } = useOllama(ollamaActive);
  const selectedOllama = useMemo(() => ollamaInstalled.find((m) => sameModel(m.name, llmModel)), [ollamaInstalled, llmModel]);
  // Nama disamakan dengan yang TERPASANG ("qwen2.5" → "qwen2.5:latest"); bila
  // pilihan belum terpasang, ambil yang siap lebih dulu.
  useEffect(() => {
    if (!ollamaStatus?.running || !ollamaInstalled.length) return;
    const match = ollamaInstalled.find((m) => sameModel(m.name, llmModel));
    if (match) { if (match.name !== llmModel) setLlmModel(match.name); return; }
    setLlmModel((ollamaInstalled.find((m) => m.ready) || ollamaInstalled[0]).name);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ollamaInstalled, ollamaStatus?.running]);
  // Tanpa Ollama, SETELAN AWAL jadi heuristik — sekali, saat status pertama
  // datang. Bukan penggantian mesin diam-diam saat job berjalan (notes/12).
  const autoPicked = useRef(false);
  useEffect(() => {
    if (autoPicked.current || ollamaStatus === null || !ollamaActive) return;
    autoPicked.current = true;
    if (!ollamaStatus.running) { setEngine("heuristic"); setTranscriptFix(false); addLog(t("logNoOllama")); }
  }, [ollamaStatus, ollamaActive, setEngine, setTranscriptFix, addLog, t]);
  // Unduhan model berjalan di LATAR engine; di sini cuma berlangganan kabarnya.
  const pullModel = useCallback(async () => {
    setPulling(true); setPullPct(-1);
    addLog(t("logPullStart", { model: llmModel }));
    try {
      const res = await fetch(eng(`/api/ollama/pull`), {
        method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ model: llmModel }),
      });
      if (!res.ok && res.status !== 409) throw new Error((await res.json()).error || "failed");
    } catch (e: any) { setPulling(false); addLog(t("logPullFailed", { error: e.message })); }
  }, [llmModel, addLog, t]);
  useEffect(() => {
    if (!ollamaActive) return;
    const es = new EventSource(eng(`/api/requirements/events`));
    es.addEventListener("install", (ev) => {
      const st = JSON.parse((ev as MessageEvent).data);
      if (!String(st.id || "").startsWith("llm-model:")) return;
      if (!st.done) { setPulling(true); setPullPct(st.value); return; }
      setPulling(false);
      if (st.error) addLog(t("logPullFailed", { error: st.error }));
      else { addLog(t("logPullDone", { model: String(st.id).slice(10) })); checkOllama(); }
    });
    return () => es.close();
  }, [ollamaActive, addLog, checkOllama, t]);
  const pullLabel = pullPct >= 0 ? `${t("downloading")} ${Math.round(pullPct * 100)}%` : t("downloading");
  const ollamaWarn = !ollamaActive ? null
    : ollamaStatus !== null && !ollamaStatus.running ? (
      <Warn>{t("ollamaNotDetected")} <code>ollama serve</code>
        <button type="button" className="ghost tiny" onClick={() => checkOllama()}>{t("recheck")}</button></Warn>
    ) : ollamaStatus?.running && !selectedOllama ? (
      <Warn>{t("modelNeedsDownload")}
        <button type="button" className="ghost tiny" onClick={pullModel} disabled={pulling}>
          {pulling ? pullLabel : <><Download className="ico" aria-hidden="true" /> {t("downloadModel")}</>}
        </button></Warn>
    ) : selectedOllama && !selectedOllama.ready ? (
      <Warn>{selectedOllama.note}
        <button type="button" className="ghost tiny" onClick={pullModel} disabled={pulling}>
          {pulling ? pullLabel : <><Download className="ico" aria-hidden="true" /> {t("downloadSelected")}</>}
        </button></Warn>
    ) : pulling ? <span className="meta">{pullLabel}</span> : null;

  const sub = (label: MessageKey, control: React.ReactNode, note?: React.ReactNode) => (
    <div className="mc-field">
      <span className="mc-sub">{t(label)}</span>
      {control}
      {note && <span className="mc-note">{note}</span>}
    </div>
  );

  const body = () => {
    switch (open) {
      case "transcription":
        return <>
          {sub("model", p.transcriber === "whisper" ? (
            <Select value={p.model} onChange={p.setModel} ariaLabel={t("mcTranscription")} options={p.models.map((m) => ({
              value: m.name, label: `Whisper ${m.name}`, note: m.downloaded ? m.size : t("modelNotDownloaded"),
            }))} />
          ) : <span className="mc-note">{t("mcTranscriptionElsewhere")}</span>)}
          <div className="mc-field">
            <Toggle checked={p.transcriptFix} onChange={p.setTranscriptFix} label={t("transcriptFix")} desc={t("transcriptFixTip")} />
            {p.transcriptFix && engine === "heuristic" && <span className="mc-note">{t("transcriptFixNeedsLLM")}</span>}
          </div>
          {/* Istilah hanya dipakai tahap koreksi; tanpa koreksi isian ini tak berpengaruh. */}
          {p.transcriptFix && sub("terms",
            <input id="terms" value={p.terms} placeholder={t("termsPlaceholder")} onChange={(e) => p.setTerms(e.target.value)} />)}
        </>;
      case "ai":
        return <>
          <p className="mc-note">
            <b>{engineName}</b>{engine !== "heuristic" && llmModel && <> · <code>{llmModel}</code></>}
            {" "}<a href="/requirements">{t("aiChangeInSettings")} →</a>
          </p>
          {ollamaWarn}
        </>;
      case "resolution":
        return <>
          {sub("resolution", <Segmented label={t("resolution")} value={p.resolution} onChange={p.setResolution} options={[
            { value: "720p", name: "720×1280" }, { value: "1080p", name: "1080×1920" }, { value: "1440p", name: "1440×2560" },
          ]} />)}
          {sub("quality", <Segmented label={t("quality")} value={p.quality} onChange={p.setQuality} options={[
            { value: "draft", name: t("qualityDraft") }, { value: "hd", name: t("qualityHd") }, { value: "max", name: t("qualityMax") },
          ]} />)}
          {/* Mode web: hasil selalu di folder server, jadi pilihan folder tidak bermakna. */}
          {p.web === false && sub("outputDir",
            <div className="path-row">
              <input id="out-dir" value={p.outputDir} onChange={(e) => p.setOutputDir(e.target.value)} placeholder={t("outputDirPlaceholder")} />
              <button type="button" className="ghost" onClick={p.onPickOut}>{t("pickerGo")}…</button>
            </div>)}
        </>;
      case "fps":
        return <Segmented label={t("fps")} value={p.fps} onChange={p.setFps} options={[
          { value: 0, name: t("fpsSource") }, { value: 24, name: "24" }, { value: 30, name: "30" }, { value: 60, name: "60" },
        ]} />;
      case "font":
        return <>
          <Select value={p.fontManual ? "__manual__" : p.subFont} ariaLabel={t("font")}
            onChange={(v) => { if (v === "__manual__") { p.setFontManual(true); return; } p.setFontManual(false); p.setSubFont(v); }}
            options={[...p.fonts.map((f) => ({ value: f.name, label: f.name })), { value: "__manual__", label: t("fontOther") }]} />
          {/* Font di luar daftar dicek engine dulu; job menolaknya sampai lolos. */}
          {p.fontManual && <>
            <input className="mc-gap" value={p.subFont} spellCheck={false} aria-label={t("font")}
              placeholder={t("fontPlaceholder")} onChange={(e) => p.setSubFont(e.target.value)} />
            <span className="mc-note">
              {p.fontChecking ? t("fontChecking") : !p.fontCheck ? t("fontHint")
                : p.fontCheck.valid ? t("fontFound", { family: p.fontCheck.family, source: p.fontCheck.source })
                : p.fontCheck.error}
            </span>
          </>}
        </>;
      case "text":
        return <>
          {sub("color", <Swatches label={t("color")} value={p.subColor} onChange={p.setSubColor} options={[
            { value: "white", name: t("colorWhite"), swatch: "#ffffff" }, { value: "yellow", name: t("colorYellow"), swatch: "#ffdd00" },
          ]} />)}
          {sub("size", <Stepper value={p.subSize} onChange={p.setSubSize} min={40} max={140} step={2} />, t("mcSizeNote"))}
          {sub("subSpeed", <Segmented label={t("subSpeed")} value={p.subSpeed} onChange={p.setSubSpeed} options={[
            { value: "slow", name: t("speedSlow") }, { value: "normal", name: t("speedNormal") }, { value: "dense", name: t("speedDense") },
          ]} />, t("subSpeedTip"))}
          <div className="mc-field">
            <Toggle checked={p.subBox} onChange={p.setSubBox} label={t("mcBox")} desc={t("mcBoxDesc")} />
          </div>
          <div className="mc-field">
            {/* Gaya keempat; tiga lainnya di kartu Subtitle style. */}
            <Toggle checked={p.subMode === "word"} onChange={(v) => p.setSubMode(v ? "word" : "highlight")}
              label={t("subWord")} desc={t("mcWordDesc")} />
          </div>
        </>;
      case "outline":
        return <Segmented label={t("outline")} value={p.subOutline} onChange={p.setSubOutline} options={[
          ...OUTLINES.map((o) => ({ value: o.v, name: t(o.k) })),
          ...(OUTLINES.some((o) => o.v === p.subOutline) ? [] : [{ value: p.subOutline, name: t("mcCustom", { n: p.subOutline }) }]),
        ]} />;
      case "position":
        return <>
          {sub("position",
            <div className="mc-row">
              <span className="mc-xy">{p.subX} · {p.subY}</span>
              <button type="button" className="ghost" onClick={p.onCentre}><Crosshair className="ico" aria-hidden="true" /> {t("mcCentre")}</button>
              <button type="button" className="ghost" disabled={!p.hasZone} onClick={p.onPlaceSafe}>{t("mcSafe")}</button>
            </div>,
            p.inUnsafe ? <span className="bad">{t("unsafeWarning")}</span> : t("mcPositionNote"))}
          {sub("platformGuide", <Segmented label={t("platformGuide")} value={p.platform} onChange={p.setPlatform} options={[
            ...Object.keys(PLATFORMS).map((k) => ({ value: k, name: k === "generic" ? t("platformGeneric") : PLATFORMS[k].label.replace(/^(Instagram|YouTube) /, "") })),
            { value: "off", name: t("platformNone") },
          ]} />, t("platformGuideTip"))}
          {sub("grid", <Segmented label={t("grid")} value={p.grid} onChange={p.setGrid}
            options={GRIDS.map((g) => ({ value: g, name: g === 0 ? t("gridOff") : String(g) }))} />)}
          <div className="mc-field">
            <Toggle checked={p.alwaysGuides} onChange={p.setAlwaysGuides} label={t("guidesAlways")} />
          </div>
        </>;
      case "zoom":
        return <>
          {sub("fitMode", <Segmented label={t("fitMode")} value={p.reframe} onChange={p.setReframe} options={[
            { value: "center", name: t("fitCenter") }, { value: "fit", name: t("fitWhole") },
          ]} />, t("fitModeTip"))}
          {sub("zoom", <Stepper value={p.zoom} onChange={p.setZoom} min={bounds.min} max={ZOOM_MAX} step={ZOOM_STEP} suffix="%" />,
            p.reframe === "fit" ? t("mcZoomFit") : t("mcZoomCenter"))}
          {/* Latar hanya terlihat bila gambar tidak memenuhi bingkai (zoom < 100). */}
          {sub("background", p.zoom < 100 ? <Segmented label={t("background")} value={p.background} onChange={p.setBackground} options={[
            { value: "blur", name: t("backgroundBlur") }, { value: "black", name: t("backgroundBlack") },
          ]} /> : null, p.zoom < 100 ? t("backgroundTip") : t("mcBackgroundHidden"))}
        </>;
      case "watermark":
        return <WatermarkPanel bare watermark={p.watermark} setWatermark={p.setWatermark}
          onPickImage={p.onPickImage} open setOpen={() => {}} />;
    }
    return null;
  };

  return (
    <section className={"step-card mc" + (p.disabled ? " off" : "")} aria-disabled={p.disabled || undefined}>
      <div className="mc-head">
        <h2>{t("mcTitle")}</h2>
        <span className="mc-hint">{t("mcHint")}</span>
      </div>
      <fieldset className="step-body" disabled={p.disabled}>
        <div className="mc-icons" role="group" aria-label={t("mcTitle")}>
          {ITEMS.map(({ key, icon: Icon, label }) => (
            <button key={key} type="button" className={"mc-btn" + (open === key ? " on" : "")}
              aria-expanded={open === key} aria-controls="mc-panel"
              onClick={() => setOpen(open === key ? null : key)}>
              <span className={`mc-ico mc-${key}`} aria-hidden="true"><Icon className="ico" /></span>
              <span className="mc-label">{t(label)}</span>
              {/* Peringatan Ollama terlihat walau panel AI tertutup. */}
              {key === "ai" && ollamaWarn && open !== "ai" && <span className="mc-alert" aria-label={t("mcAIAlert")} />}
            </button>
          ))}
        </div>
        {item && (
          <div className="mc-panel" id="mc-panel">
            <span className={`mc-ico mc-${item.key}`} aria-hidden="true"><item.icon className="ico" /></span>
            <div className="mc-panel-body">
              <p className="mc-panel-title">{t(item.label)}</p>
              <p className="mc-panel-desc">{t(item.desc)}</p>
              <div className="mc-control">{body()}</div>
            </div>
          </div>
        )}
      </fieldset>
    </section>
  );
}
