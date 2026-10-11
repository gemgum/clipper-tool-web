"use client";

import { useI18n } from "./i18n";
import Tip from "./tip";
import Stepper from "./stepper";
import Select from "./select";

// Zoom dibaca RELATIF terhadap titik awal modenya, jadi batas bawah & nilai
// awalnya berbeda per mode. Harus sama dengan engine/config.
//   fit    : mulai 0 (seluruh video masuk) → hanya bisa NAIK
//   center : mulai 100 (memenuhi bingkai)   → hanya bisa TURUN
// Keduanya berhenti di 100: di situ gambar sudah memenuhi bingkai.
export const ZOOM_MAX = 100, ZOOM_STEP = 5;
const ZOOM_BOUNDS: Record<string, { min: number; natural: number }> = {
  fit: { min: 0, natural: 0 },
  center: { min: 5, natural: 100 },
};
export const zoomBounds = (mode: string) => ZOOM_BOUNDS[mode] ?? ZOOM_BOUNDS.center;

// Cara video dipasang ke bingkai 9:16 — tiga kendali, satu baris.
//
// Ditaruh DI SEBELAH pratinjau, bukan di panel setelan render: ketiganya
// langsung mengubah gambar yang terlihat di sana, jadi hasilnya kelihatan saat
// itu juga.
//
// Pembatas platform TIDAK di sini (pindah 6 Agustus 2026): ia mengatur ke mana
// SUBTITLE boleh ditaruh, bukan bagaimana video dipasang — dua pertanyaan
// berbeda yang selama ini duduk di satu kelompok dan membingungkan.
export default function FramePanel({
  reframe, onReframe, background, setBackground, zoom, setZoom,
}: {
  reframe: string; onReframe: (v: string) => void;
  background: string; setBackground: (v: string) => void;
  zoom: number; setZoom: (v: number) => void;
}) {
  const { t } = useI18n();

  // Mulai zoom 100 gambar menutupi bingkai di kedua mode, jadi latarnya tidak
  // akan terlihat.
  const noEmptySpace = zoom >= 100;
  const bounds = zoomBounds(reframe);

  return (
    <div className="group">
      <div className="group-title" role="heading" aria-level={3}>{t("groupVideoInFrame")}</div>
      <div className="grid3">
        {/* Dua cara memasangkan video ke bingkai 9:16 — pilihan yang berdiri
            sendiri, bukan titik pada satu sumbu. Mode "video utuh" itulah alasan
            pilihan latar di sebelahnya ada. */}
        <div className="field"><label title={t("fitModeTip")}>{t("fitMode")}</label>
          <Select value={reframe} onChange={onReframe} options={[
            { value: "center", label: t("fitCenter") },
            { value: "fit", label: t("fitWhole") },
          ]} /></div>
        <div className="field"><label>{t("background")} <Tip text={noEmptySpace ? t("backgroundWhy") : t("backgroundTip")} /></label>
          <Select value={background} onChange={setBackground} disabled={noEmptySpace} options={[
            { value: "blur", label: t("backgroundBlur") },
            { value: "black", label: t("backgroundBlack") },
          ]} /></div>
        {/* Tombol −/+ menaruh angka yang persis — sesuatu yang tidak pernah bisa
            dilakukan menyeret penggeser dengan tetikus. Tombolnya mati sendiri
            di batas, jadi ujung skalanya tak perlu ditulis. */}
        <div className="field"><label title={t("zoomTip")}>{t("zoom")}</label>
          <Stepper value={zoom} onChange={setZoom} min={bounds.min} max={ZOOM_MAX} step={ZOOM_STEP} suffix="%" /></div>
      </div>
    </div>
  );
}
