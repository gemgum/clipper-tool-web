"use client";

// Watermark halaman klip (tab Layout): banner PNG milik pengguna + headline di
// atasnya, dibakar ke tiap klip. Bentuknya resep kartu setelan: baris di kiri,
// SATU panel warna di kanan (color-panel.tsx).

import { FolderOpen, X } from "lucide-react";
import { useI18n } from "./i18n";
import { Position } from "./guides";
import Stepper from "./stepper";
import { Segmented } from "./clip-steps";
import { Row } from "./tab-card";
import type { ColorTarget } from "./color-panel";
import type { MessageKey } from "./i18n";
import type { Watermark } from "./watermark-model";
import { headlineAnchor, headlineBox, wrapHeadline } from "./watermark-model";

export default function WatermarkPanel({ watermark, setWatermark, onPickImage }: {
  watermark: Watermark;
  setWatermark: (patch: Partial<Watermark>) => void;
  onPickImage: () => void;
}) {
  const { t } = useI18n();
  const name = watermark.image ? watermark.image.split(/[\\/]/).pop() : "";
  const llm = watermark.hlSource === "llm";
  // Titik SEBENARNYA headline di bidang, walau yang disimpan geseran: "540·520"
  // langsung terbaca sebagai tempat, "0·-40" menuntut pembacanya menghitung.
  const hlBox = headlineBox(watermark);
  const hlAt = headlineAnchor(
    wrapHeadline(watermark.hlText, watermark.hlSize, hlBox.w),
    watermark.hlSize, watermark.hlDX, watermark.hlDY, hlBox);

  return (
    <>
        <Row label={t("wmImage")} title={t("wmImageTip")}>
          <button type="button" className="ghost" title={name || t("wmImagePick")} onClick={onPickImage}>
            <FolderOpen className="ico" aria-hidden="true" /> {name || t("wmImagePick")}
          </button>
          {watermark.image && (
            <button type="button" className="ghost tiny icon-only" title={t("wmImageClear")} aria-label={t("wmImageClear")}
              onClick={() => setWatermark({ image: "" })}><X className="ico" aria-hidden="true" /></button>
          )}
        </Row>
        {/* Lebar DAN tinggi = kotak tempat gambar dimuat utuh (persen bingkai). */}
        <Row label={t("wmWidth")} title={t("wmSizeTip")}>
          <Stepper value={watermark.width} onChange={(v) => setWatermark({ width: v })} min={5} max={100} step={1} suffix="%" />
        </Row>
        <Row label={t("wmHeight")} title={t("wmSizeTip")}>
          <Stepper value={watermark.height} onChange={(v) => setWatermark({ height: v })} min={5} max={100} step={1} suffix="%" />
        </Row>
        <Row label={t("position")}>
          <Position x={watermark.x} y={watermark.y} onReset={() => setWatermark({ x: 540, y: 960 })} />
        </Row>
        {/* 0 detik durasi = sampai klip habis, dan itu bawaannya. */}
        <Row label={t("wmAt")} title={t("wmAtTip")}>
          <Stepper value={watermark.at} onChange={(v) => setWatermark({ at: v })} min={0} max={60} step={1} suffix="s" />
        </Row>
        <Row label={t("wmFor")} title={t("wmForTip")}>
          <Stepper value={watermark.dur} onChange={(v) => setWatermark({ dur: v })} min={0} max={180} step={1} suffix="s" />
        </Row>
        <Row label={t("headlineSource")} title={t("headlineSourceTip")}>
          <Segmented label={t("headlineSource")} value={watermark.hlSource} onChange={(v) => setWatermark({ hlSource: v })}
            options={[{ value: "text" as Watermark["hlSource"], name: t("headlineMine") }, { value: "llm" as Watermark["hlSource"], name: t("headlineLLM") }]} />
        </Row>
        {/* Dimatikan pada sumber "llm": kotak yang isinya tidak dipakai lebih buruk daripada kotak yang jelas mati. */}
        <Row label={t("headlineText")}>
          <input value={watermark.hlText} spellCheck={false} disabled={llm} aria-label={t("headlineText")}
            placeholder={llm ? t("headlineFromLLM") : t("headlinePlaceholder")}
            onChange={(e) => setWatermark({ hlText: e.target.value })} />
        </Row>
        <Row label={t("headlineSize")}>
          <Stepper value={watermark.hlSize} onChange={(v) => setWatermark({ hlSize: v })} min={24} max={140} step={2} />
        </Row>
        <Row label={t("outline")}>
          <Stepper value={watermark.hlOutline} onChange={(v) => setWatermark({ hlOutline: v })} min={0} max={12} />
        </Row>
        <Row label={t("headlinePosition")}>
          <Position x={Math.round(hlAt.x)} y={Math.round(hlAt.y)} onReset={() => setWatermark({ hlDX: 0, hlDY: 0 })} />
        </Row>
    </>
  );
}

// Sasaran warna headline — SATU daftar untuk halaman klip dan Watermark. "Box"
// ikut hanya saat kotak judul menyala (halaman klip tidak punya kotak judul).
export const watermarkColors = (w: Watermark, set: (patch: Partial<Watermark>) => void, t: (k: MessageKey) => string): ColorTarget[] => [
  { key: "text", name: t("mcColText"), value: w.hlColor, set: (c) => set({ hlColor: c }) },
  ...(w.hlBox ? [{ key: "box", name: t("wmColBox"), value: w.hlBoxColor || "white", set: (c: string) => set({ hlBoxColor: c }) }] : []),
  { key: "outline", name: t("outline"), value: w.hlOutlineColor || (w.hlColor === "black" ? "white" : "black"), set: (c) => set({ hlOutlineColor: c }) },
];
