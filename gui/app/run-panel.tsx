"use client";

import { useI18n } from "./i18n";

// Tombol Mulai/Batal + kemajuan job, ditaruh di dalam <PageHeader>.
//
// Pindah dari dasar kolom kanan ke kepala halaman 9 Oktober 2026 (DESIGN.md
// §5.2): aksi utama dan statusnya selalu terlihat tanpa menggulir. Dua aturan
// lama tetap berlaku di tempat barunya:
// - bilah kemajuan SELALU ada (kosong saat diam) dan menempel di tepi bawah
//   kepala halaman, jadi memulai job tidak menggeser apa pun;
// - Cancel selalu dirender, dimatikan bila tidak ada yang bisa dibatalkan —
//   tombol yang muncul-hilang menggeser tombol di bawah kursor.
// Teks status cuma "siap" atau persentase. Kalimat tahapnya ada di kotak log;
// mengulangnya di sini dulu membuat panelnya menembus lebarnya sendiri.
export default function RunPanel({
  busy, testing, disabled, cancellable, onStart, onCancel, progress,
}: {
  busy: boolean;
  /** Uji LLM sedang berjalan — tombolnya mati DAN mengatakan alasannya. */
  testing: boolean;
  disabled: boolean;
  cancellable: boolean;
  onStart: () => void;
  onCancel: () => void;
  progress: number;
}) {
  const { t } = useI18n();

  const pct = Math.round(progress * 100);
  return (
    <>
      <span className={"run-status" + (busy ? " on" : "")} aria-live="polite">
        <span className="run-dot" aria-hidden="true" />
        {busy ? t("statusRunning", { pct }) : t("statusIdle")}
      </span>
      {/* Tombol mati saja tidak cukup: tombol yang kelabu tanpa sebab terbaca
          sebagai aplikasi macet. Selama uji LLM ia menyebutkan apa yang
          ditunggu. */}
      <button className="primary" onClick={onStart} disabled={disabled}>
        {testing ? t("llmTesting") : busy ? t("processing") : t("start")}
      </button>
      <button className="ghost" onClick={onCancel} disabled={!cancellable}>{t("cancel")}</button>
      <div className="ph-progress" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={pct}>
        <div style={{ width: `${pct}%` }} />
      </div>
    </>
  );
}
