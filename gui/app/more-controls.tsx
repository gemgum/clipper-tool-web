"use client";

// Kartu "More controls" (rancangan Video clips, 14 Oktober 2026): deretan ikon
// untuk setelan yang paling sering disentuh; satu ketukan membuka SATU panel di
// bawahnya, ketukan kedua menutupnya.
//
// Bukan tempat kedua untuk menyimpan nilai: semua kendali di sini memegang state
// halaman yang SAMA dengan laci Advanced settings, jadi keduanya tidak bisa
// berbeda. Dua ikon rancangan disesuaikan dengan yang benar-benar ada:
//   "Auto zoom"  zoom yang mengikuti ucapan tidak ada di engine — ikonnya
//                memegang Zoom bingkai yang sungguhan;
//   Watermark    watermark klip diatur di laci Advanced halaman ini, bukan di
//                halaman Watermark (alat lain, untuk video yang sudah 9:16).
// AI engine hanya ditampilkan: mesin dipilih sekali di Settings (notes/39).

import { AudioLines, Cpu, Droplet, Film, Monitor, PenLine, Type, ZoomIn } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useState } from "react";
import { useI18n } from "./i18n";
import type { MessageKey } from "./i18n";
import Select from "./select";
import Stepper from "./stepper";
import { Segmented } from "./clip-steps";
import { useEngines } from "./engine-picker";
import type { WhisperModel } from "./setup-panel";
import type { Font } from "./preview-panel";
import { zoomBounds, ZOOM_MAX, ZOOM_STEP } from "./frame-panel";

type Key = "transcription" | "ai" | "resolution" | "fps" | "font" | "outline" | "zoom" | "watermark";
const ITEMS: { key: Key; icon: LucideIcon; label: MessageKey; desc: MessageKey }[] = [
  { key: "transcription", icon: AudioLines, label: "mcTranscription", desc: "mcTranscriptionDesc" },
  { key: "ai", icon: Cpu, label: "mcAI", desc: "mcAIDesc" },
  { key: "resolution", icon: Monitor, label: "resolution", desc: "mcResolutionDesc" },
  { key: "fps", icon: Film, label: "fps", desc: "mcFpsDesc" },
  { key: "font", icon: Type, label: "font", desc: "mcFontDesc" },
  { key: "outline", icon: PenLine, label: "outline", desc: "mcOutlineDesc" },
  { key: "zoom", icon: ZoomIn, label: "zoom", desc: "mcZoomDesc" },
  { key: "watermark", icon: Droplet, label: "groupWatermark", desc: "mcWatermarkDesc" },
];
// Tebal garis tepi sebagai tiga pilihan; angka lain (dari Advanced) = Custom.
const OUTLINES = [{ v: 3, k: "mcThin" }, { v: 5, k: "mcMedium" }, { v: 8, k: "mcThick" }] as const;

export default function MoreControls({
  model, setModel, models, transcriber, engine, llmModel,
  resolution, setResolution, fps, setFps,
  fonts, subFont, setSubFont, fontManual, setFontManual,
  subOutline, setSubOutline, reframe, zoom, setZoom,
  watermarkOn, onOpenAdvanced, disabled,
}: {
  model: string; setModel: (v: string) => void; models: WhisperModel[]; transcriber: string;
  engine: string; llmModel: string;
  resolution: string; setResolution: (v: string) => void;
  fps: number; setFps: (v: number) => void;
  fonts: Font[]; subFont: string; setSubFont: (v: string) => void;
  fontManual: boolean; setFontManual: (v: boolean) => void;
  subOutline: number; setSubOutline: (v: number) => void;
  reframe: string; zoom: number; setZoom: (v: number) => void;
  watermarkOn: boolean; onOpenAdvanced: () => void;
  disabled?: boolean;
}) {
  const { t } = useI18n();
  const { engines } = useEngines();
  const [open, setOpen] = useState<Key | null>(null);
  const item = ITEMS.find((i) => i.key === open);
  const bounds = zoomBounds(reframe);

  const body = () => {
    switch (open) {
      case "transcription":
        return transcriber === "whisper" ? (
          <Select value={model} onChange={setModel} ariaLabel={t("mcTranscription")} options={models.map((m) => ({
            value: m.name, label: `Whisper ${m.name}`, note: m.downloaded ? m.size : t("modelNotDownloaded"),
          }))} />
        ) : <p className="mc-note">{t("mcTranscriptionElsewhere")}</p>;
      case "ai":
        return (
          <p className="mc-note">
            <b>{engine === "heuristic" ? t("offlineHeuristic") : (engines.find((e) => e.id === engine)?.name || engine)}</b>
            {engine !== "heuristic" && llmModel && <> · <code>{llmModel}</code></>}
            {" "}<a href="/requirements">{t("aiChangeInSettings")} →</a>
          </p>
        );
      case "resolution":
        return <Segmented label={t("resolution")} value={resolution} onChange={setResolution} options={[
          { value: "720p", name: "720×1280" }, { value: "1080p", name: "1080×1920" }, { value: "1440p", name: "1440×2560" },
        ]} />;
      case "fps":
        return <Segmented label={t("fps")} value={fps} onChange={setFps} options={[
          { value: 0, name: t("fpsSource") }, { value: 24, name: "24" }, { value: 30, name: "30" }, { value: 60, name: "60" },
        ]} />;
      case "font":
        return (
          <Select value={fontManual ? "__manual__" : subFont} ariaLabel={t("font")}
            onChange={(v) => { if (v === "__manual__") return; setFontManual(false); setSubFont(v); }}
            options={[
              ...fonts.map((f) => ({ value: f.name, label: f.name })),
              // Font manual diketik & dicek di Advanced; di sini cukup terlihat.
              ...(fontManual ? [{ value: "__manual__", label: subFont, note: t("mcFontCustom") }] : []),
            ]} />
        );
      case "outline":
        return <Segmented label={t("outline")} value={subOutline} onChange={setSubOutline} options={[
          ...OUTLINES.map((o) => ({ value: o.v, name: t(o.k) })),
          ...(OUTLINES.some((o) => o.v === subOutline) ? [] : [{ value: subOutline, name: t("mcCustom", { n: subOutline }) }]),
        ]} />;
      case "zoom":
        return (
          <div className="mc-row">
            <Stepper value={zoom} onChange={setZoom} min={bounds.min} max={ZOOM_MAX} step={ZOOM_STEP} suffix="%" />
            <span className="mc-note">{reframe === "fit" ? t("mcZoomFit") : t("mcZoomCenter")}</span>
          </div>
        );
      case "watermark":
        return (
          <p className="mc-note">
            {watermarkOn ? t("mcWatermarkOn") : t("mcWatermarkOff")}{" "}
            <button type="button" className="mc-link" onClick={onOpenAdvanced}>{t("mcWatermarkOpen")} →</button>
          </p>
        );
    }
    return null;
  };

  return (
    <section className={"step-card mc" + (disabled ? " off" : "")} aria-disabled={disabled || undefined}>
      <div className="mc-head">
        <h2>{t("mcTitle")}</h2>
        <span className="mc-hint">{t("mcHint")}</span>
      </div>
      <fieldset className="step-body" disabled={disabled}>
        <div className="mc-icons" role="group" aria-label={t("mcTitle")}>
          {ITEMS.map(({ key, icon: Icon, label }) => (
            <button key={key} type="button" className={"mc-btn" + (open === key ? " on" : "")}
              aria-expanded={open === key} aria-controls="mc-panel"
              onClick={() => setOpen(open === key ? null : key)}>
              <span className={`mc-ico mc-${key}`} aria-hidden="true"><Icon className="ico" /></span>
              <span className="mc-label">{t(label)}</span>
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
