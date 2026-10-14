"use client";

// Kartu setelan klip (rancangan "Redesain Video Clips", 14 Oktober 2026): empat
// tab sama lebar; panel tab terpilih selalu terbuka di bawahnya, dibagi bagian.
//
// Sejak laci "Advanced settings" dibuang, SEMUA setelan lanjutan tinggal di
// sini — tidak ada tempat kedua. AI engine hanya ditampilkan: mesin dipilih
// sekali di Settings (notes/39).

import { Ban, Camera, CaseSensitive, Cpu, Download, Highlighter, Mic, Monitor, Move, Music, Smartphone, SquarePlay, Type } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useI18n } from "./i18n";
import type { MessageKey } from "./i18n";
import { eng } from "./engine";
import Select from "./select";
import Stepper from "./stepper";
import Warn from "./warn";
import { Segmented, Toggle } from "./clip-steps";
import { useEngines } from "./engine-picker";
import { sameModel, useOllama } from "./ollama";
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

type Key = "transcription" | "ai" | "clips" | "quality" | "style" | "type"
  | "platform" | "zoom" | "watermark";
// Judul tiap bagian di dalam panel tab. Posisi, kisi, dan panduan tinggal di kartu Preview.
const PARTS: Record<Key, MessageKey> = {
  transcription: "mcSecTranscript", ai: "mcAI",
  clips: "mcSecClips", quality: "mcSecVideo",
  style: "mcSecStyle", type: "mcSecType",
  platform: "mcSecPlatform", zoom: "mcFraming", watermark: "groupWatermark",
};
// Semua setelan halaman: empat tab, satu panel selalu terbuka.
export type Group = "ai" | "output" | "text" | "layout";
const GROUPS: { key: Group; icon: LucideIcon; label: MessageKey; parts: Key[] }[] = [
  { key: "ai", icon: Cpu, label: "mcGroupAI", parts: ["transcription", "ai"] },
  { key: "output", icon: Monitor, label: "mcGroupOutput", parts: ["clips", "quality"] },
  { key: "text", icon: Type, label: "mcGroupText", parts: ["style", "type"] },
  { key: "layout", icon: Move, label: "mcGroupLayout", parts: ["platform", "zoom", "watermark"] },
];
// Tebal garis tepi sebagai tiga pilihan; angka lain = Custom (angka persisnya di panel Text).
const OUTLINES = [{ v: 3, k: "mcThin" }, { v: 5, k: "mcMedium" }, { v: 8, k: "mcThick" }] as const;
// Palet warna teks & sorotan (engine menerima #RRGGBB untuk keduanya).
const PALETTE = [
  { hex: "#FFFFFF", k: "colorWhite" }, { hex: "#FFD400", k: "colorYellow" }, { hex: "#F2874B", k: "colorOrange" },
  { hex: "#E5484D", k: "colorRed" }, { hex: "#FF7AB6", k: "colorPink" }, { hex: "#9B7BFF", k: "colorPurple" },
  { hex: "#4F8BFF", k: "colorBlue" }, { hex: "#3AD1D9", k: "colorCyan" }, { hex: "#38C976", k: "colorGreen" },
  { hex: "#111111", k: "colorBlack" },
] as const;
// Nilai lama "white"/"yellow" (preset tersimpan) dibaca sebagai hex palet.
const colHex = (c: string) => (c === "white" ? "#ffffff" : c === "yellow" ? "#ffd400" : c.toLowerCase());
const SIZES = [{ v: 72, k: "sizeS" }, { v: 94, k: "sizeM" }, { v: 120, k: "sizeL" }] as const;

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
  platform: string; setPlatform: (v: string) => void;
  // bingkai
  reframe: string; setReframe: (v: string) => void;
  background: string; setBackground: (v: string) => void;
  zoom: number; setZoom: (v: number) => void;
  // watermark
  watermark: Watermark; setWatermark: (patch: Partial<Watermark>) => void; onPickImage: () => void;
  // gaya, ukuran, sorotan, dan jumlah/panjang klip
  subHighlight: string; setSubHighlight: (v: string) => void; pickSize: (v: number) => void;
  maxClips: number; setMaxClips: (v: number) => void;
  durationPreset: string; setDurationPreset: (v: string) => void;
  saveMode: string; setSaveMode: (v: string) => void;
}) {
  const { t } = useI18n();
  const { engines } = useEngines();
  const [open, setOpen] = useState<Group>("ai");
  const [colMode, setColMode] = useState<"text" | "hl">("text");
  const item = GROUPS.find((g) => g.key === open)!;
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

  // Satu baris: label di kiri, kendali di kanan (rancangan "tidy settings").
  const row = (label: MessageKey, control: React.ReactNode, note?: React.ReactNode) => (
    <div className="mc-r">
      <span className="mc-r-l">{t(label)}</span>
      <div className="mc-r-c">{control}{note && <span className="mc-note">{note}</span>}</div>
    </div>
  );
  const sw = (toggle: React.ReactNode, note?: React.ReactNode) => (
    <div className="mc-r mc-r-sw">{toggle}{note && <span className="mc-note">{note}</span>}</div>
  );
  const isHl = colMode === "hl";
  const colValue = isHl ? p.subHighlight : p.subColor;

  const body = (part: Key) => {
    switch (part) {
      case "transcription":
        return <>
          {p.transcriber === "whisper" && row("model", (
            <Select value={p.model} onChange={p.setModel} ariaLabel={t("mcTranscription")} options={p.models.map((m) => ({
              value: m.name, label: `Whisper ${m.name}`, note: m.downloaded ? m.size : t("modelNotDownloaded"),
            }))} />
          ))}
          {sw(<Toggle checked={p.transcriptFix} onChange={p.setTranscriptFix} label={t("transcriptFix")} desc={t("transcriptFixTip")} />,
            p.transcriptFix && engine === "heuristic" && t("transcriptFixNeedsLLM"))}
          {/* Istilah hanya dipakai tahap koreksi; tanpa koreksi isian ini tak berpengaruh. */}
          {p.transcriptFix && row("terms",
            <input id="terms" value={p.terms} placeholder={t("termsPlaceholder")} onChange={(e) => p.setTerms(e.target.value)} />)}
        </>;
      case "ai":
        return <>
          {row("mcEngineRow", <>
            <span className="mc-val">{engineName}{engine !== "heuristic" && llmModel && <> · {llmModel}</>}</span>
            <a className="mc-link" href="/requirements">{t("aiChangeInSettings")} →</a>
          </>)}
          {ollamaWarn && <div className="mc-r mc-r-sw">{ollamaWarn}</div>}
        </>;
      case "clips":
        return <>
          {row("clipCountLabel", <Stepper value={p.maxClips} onChange={p.setMaxClips} min={1} max={40} />)}
          {row("clipLength", <Segmented label={t("clipLength")}
            value={["auto", "30", "60"].includes(p.durationPreset) ? p.durationPreset : "auto"}
            onChange={p.setDurationPreset} options={[
              { value: "auto", name: t("lenAutoName") }, { value: "30", name: "15–30 s" }, { value: "60", name: "30–60 s" },
            ]} />)}
          {sw(<Toggle checked={p.saveMode !== "burn"} onChange={(v) => p.setSaveMode(v ? "both" : "burn")} label={t("keepClean")} />)}
        </>;
      case "quality":
        return <>
          {row("resolution", <Segmented label={t("resolution")} value={p.resolution} onChange={p.setResolution} options={[
            { value: "720p", name: "720p" }, { value: "1080p", name: "1080p" }, { value: "1440p", name: "1440p" },
          ]} />)}
          {row("quality", <Segmented label={t("quality")} value={p.quality} onChange={p.setQuality} options={[
            { value: "draft", name: t("qualityDraft") }, { value: "hd", name: t("qualityHd") }, { value: "max", name: t("qualityMax") },
          ]} />)}
          {row("fps", <Segmented label={t("fps")} value={p.fps} onChange={p.setFps} options={[
            { value: 0, name: t("fpsSource") }, { value: 24, name: "24" }, { value: 30, name: "30" }, { value: 60, name: "60" },
          ]} />)}
          {/* Mode web: hasil selalu di folder server, jadi pilihan folder tidak bermakna. */}
          {p.web === false && row("outputDir",
            <div className="path-row">
              <input id="out-dir" value={p.outputDir} onChange={(e) => p.setOutputDir(e.target.value)} placeholder={t("outputDirPlaceholder")} />
              <button type="button" className="ghost" onClick={p.onPickOut}>{t("pickerGo")}…</button>
            </div>)}
        </>;
      case "style":
        return <>
          {row("mcSecStyle", <Segmented label={t("step2Title")} value={p.subMode === "word" ? "normal" : p.subMode} onChange={p.setSubMode} options={[
            { value: "normal", name: t("styleNormalName"), icon: CaseSensitive },
            { value: "highlight", name: t("styleHighlightName"), icon: Highlighter },
            { value: "karaoke", name: t("styleKaraokeName"), icon: Mic },
          ]} />)}
          {row("subSpeed", <Segmented label={t("subSpeed")} value={p.subSpeed} onChange={p.setSubSpeed} options={[
            { value: "slow", name: t("speedSlow") }, { value: "normal", name: t("speedNormal") }, { value: "dense", name: t("speedDense") },
          ]} />)}
          {/* Gaya keempat; tiga lainnya di baris Style. */}
          {sw(<Toggle checked={p.subMode === "word"} onChange={(v) => p.setSubMode(v ? "word" : "highlight")} label={t("subWord")} />)}
        </>;
      case "type":
        return <div className="mc-split">
          <div className="mc-split-l">
            {row("font", <>
              <Select value={p.fontManual ? "__manual__" : p.subFont} ariaLabel={t("font")}
                onChange={(v) => { if (v === "__manual__") { p.setFontManual(true); return; } p.setFontManual(false); p.setSubFont(v); }}
                options={[...p.fonts.map((f) => ({ value: f.name, label: f.name })), { value: "__manual__", label: t("fontOther") }]} />
              {/* Font di luar daftar dicek engine dulu; job menolaknya sampai lolos. */}
              {p.fontManual && <>
                <input value={p.subFont} spellCheck={false} aria-label={t("font")}
                  placeholder={t("fontPlaceholder")} onChange={(e) => p.setSubFont(e.target.value)} />
                <span className="mc-note">
                  {p.fontChecking ? t("fontChecking") : !p.fontCheck ? t("fontHint")
                    : p.fontCheck.valid ? t("fontFound", { family: p.fontCheck.family, source: p.fontCheck.source })
                    : p.fontCheck.error}
                </span>
              </>}
            </>)}
            {row("textSize", <Segmented label={t("textSize")} value={p.subSize} onChange={p.pickSize} options={[
              ...SIZES.map((o) => ({ value: o.v, name: t(o.k) })),
              ...(SIZES.some((o) => o.v === p.subSize) ? [] : [{ value: p.subSize, name: t("mcCustom", { n: p.subSize }) }]),
            ]} />)}
            {row("outline", <Segmented label={t("outline")} value={p.subOutline} onChange={p.setSubOutline} options={[
              ...OUTLINES.map((o) => ({ value: o.v, name: t(o.k) })),
              ...(OUTLINES.some((o) => o.v === p.subOutline) ? [] : [{ value: p.subOutline, name: t("mcCustom", { n: p.subOutline }) }]),
            ]} />)}
            {sw(<Toggle checked={p.subBox} onChange={p.setSubBox} label={t("mcBox")} />)}
          </div>
          {/* Satu palet untuk dua sasaran: warna teks atau warna sorotan. */}
          <div className="mc-split-r">
            <span className="mc-r-l">{t("color")}</span>
            <div className="segmented" role="group" aria-label={t("color")}>
              {(["text", "hl"] as const).map((m) => (
                <button key={m} type="button" className={colMode === m ? "on" : ""} aria-pressed={colMode === m} onClick={() => setColMode(m)}>
                  <span className="mc-cdot" style={{ background: colHex(m === "hl" ? p.subHighlight : p.subColor) }} />
                  {t(m === "hl" ? "mcColHl" : "mcColText")}
                </button>
              ))}
            </div>
            <div className="mc-cdots" role="group" aria-label={t(isHl ? "highlightColour" : "color")}>
              {PALETTE.map((c) => {
                const on = colHex(colValue) === c.hex.toLowerCase();
                return <button key={c.hex} type="button" className={"mc-cswatch" + (on ? " on" : "")} aria-pressed={on}
                  aria-label={t(c.k)} title={t(c.k)} onClick={() => (isHl ? p.setSubHighlight : p.setSubColor)(c.hex)}>
                  <span style={{ background: c.hex }} />
                </button>;
              })}
            </div>
          </div>
        </div>;
      case "platform":
        return row("postTo", <Segmented label={t("mcSecPlatform")} value={p.platform} onChange={p.setPlatform} options={[
          { value: "tiktok", name: "TikTok", icon: Music },
          { value: "reels", name: "Reels", icon: Camera },
          { value: "shorts", name: "Shorts", icon: SquarePlay },
          { value: "generic", name: t("platformGeneric"), icon: Smartphone },
          { value: "off", name: t("platformNone"), icon: Ban },
        ]} />);
      case "zoom":
        return <>
          {row("fitMode", <Segmented label={t("fitMode")} value={p.reframe} onChange={p.setReframe} options={[
            { value: "center", name: t("fitCenter") }, { value: "fit", name: t("fitWhole") },
          ]} />)}
          {row("zoom", <Stepper value={p.zoom} onChange={p.setZoom} min={bounds.min} max={ZOOM_MAX} step={ZOOM_STEP} suffix="%" />)}
          {/* Latar hanya ada artinya bila gambar tidak memenuhi bingkai (zoom < 100). */}
          {p.zoom < 100 && row("background", <Segmented label={t("background")} value={p.background} onChange={p.setBackground} options={[
            { value: "blur", name: t("backgroundBlur") }, { value: "black", name: t("backgroundBlack") },
          ]} />)}
        </>;
      case "watermark":
        return <div className="mc-r mc-r-pad">
          <WatermarkPanel bare watermark={p.watermark} setWatermark={p.setWatermark}
            onPickImage={p.onPickImage} open setOpen={() => {}} />
        </div>;
    }
    return null;
  };

  return (
    <section className={"step-card mc" + (p.disabled ? " off" : "")} aria-disabled={p.disabled || undefined}>
      <fieldset className="step-body" disabled={p.disabled}>
        <div className="mc-tabs" role="tablist" aria-label={t("mcTitle")}>
          {GROUPS.map(({ key, icon: Icon, label }) => (
            <button key={key} type="button" role="tab" className={"mc-tab" + (open === key ? " on" : "")}
              aria-selected={open === key} aria-controls="mc-panel" onClick={() => setOpen(key)}>
              <span className="mc-tab-ic" aria-hidden="true"><Icon className="ico" /></span>
              <span className="mc-tab-lb">{t(label)}</span>
              {/* Peringatan Ollama terlihat walau tab AI tidak dipilih. */}
              {key === "ai" && ollamaWarn && open !== "ai" && <span className="mc-alert" aria-label={t("mcAIAlert")} />}
            </button>
          ))}
        </div>
        <div className="mc-panel" id="mc-panel" role="tabpanel">
          {item.parts.map((part) => (
            <section key={part} className="mc-group">
              <h3 className="mc-group-t">{t(PARTS[part])}</h3>
              <div className="mc-rows">{body(part)}</div>
            </section>
          ))}
        </div>
      </fieldset>
    </section>
  );
}
