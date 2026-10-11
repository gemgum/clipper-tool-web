"use client";

// Ikon: lucide-react (ISC) — alasannya di gui/app/page.tsx.
import { FolderOpen, X } from "lucide-react";
import Tip from "./tip";

import { useI18n } from "./i18n";
import Section from "./section";
import { GridPicker, PositionField } from "./guides";
import Stepper from "./stepper";
import Select from "./select";
import type { Watermark } from "./watermark-model";
import { headlineAnchor, headlineBox, watermarkOn, wrapHeadline } from "./watermark-model";

// Watermark: banner PNG milik pengguna + teks di atasnya, dibakar ke tiap klip.
//
// Duduk di kolom KIRI bersama setelan pratinjau, bukan di kolom kanan: semuanya
// mengubah rupa gambar di sebelahnya, dan itu aturan pembagian kolom halaman ini
// (CLAUDE.md → Tampilan).
//
// Kelompok ini MENUKAR isi kolom, bukan menambahnya: selagi terbuka, kelompok
// subtitle, penempatan, dan bingkai disembunyikan (<PreviewPanel>). Terukur,
// itulah satu-satunya susunan yang muat — sepuluh kendali watermark setinggi
// 286 px, sedangkan kolomnya tidak punya satu piksel pun sisa. Menumpuknya di
// bawah kelompok lain membuat kolom kiri bergulir 268 px di jendela 1240x860,
// dan kolom setelan yang bergulir adalah hal yang paling dilarang di sini.
//
// Yang tetap terlihat di kedua keadaan: bingkai pratinjau — sebab semua yang
// diatur di sini mengubah gambar itu.
export default function WatermarkPanel({
  watermark, setWatermark, onPickImage, open, setOpen, allowLLM = true, gridControl,
}: {
  watermark: Watermark;
  setWatermark: (patch: Partial<Watermark>) => void;
  onPickImage: () => void;
  open: boolean;
  setOpen: (v: boolean) => void;
  // gridControl hanya diisi halaman watermark. Di halaman klip kendali kisi
  // sudah berdiri di kelompok Subtitle, dan dua kendali untuk satu hal adalah
  // dua tempat yang bisa tidak sinkron.
  gridControl?: React.ReactNode;
  // allowLLM mati di halaman watermark: di sana tidak ada klip, jadi tidak ada
  // judul yang dipilihkan LLM.
  //
  // Selnya DIBUANG, bukan dimatikan. Sempat dimatikan, dan pertanyaan pertama
  // yang datang adalah "ini kenapa tidak bisa dimasukkan?" — tepat. Kotak abu
  // berisi satu-satunya jawaban yang mungkin bukan keterangan, ia teka-teki.
  // Kalau tidak ada yang bisa dipilih, tidak ada yang perlu ditampilkan.
  allowLLM?: boolean;
}) {
  const { t } = useI18n();
  const name = watermark.image ? watermark.image.split(/[\\/]/).pop() : "";
  // Tanpa pilihan sumber, sumbernya SELALU teks sendiri. Dibaca dari sini, bukan
  // dari state: setelan tersimpan dari versi lain bisa membawa "llm", dan itu
  // akan mematikan kotak teksnya tanpa ada satu pun kendali untuk menghidupkannya
  // lagi — persis jebakan yang baru saja dibuang.
  const source = allowLLM ? watermark.hlSource : "text";
  // Angkanya ditampilkan sebagai TITIK SEBENARNYA di bidang, walau yang
  // disimpan geseran: "540·520" langsung terbaca sebagai tempat, sedangkan
  // "0·-40" menuntut pembacanya menghitung sendiri.
  const hlBox = headlineBox(watermark);
  const hlAt = headlineAnchor(
    wrapHeadline(watermark.hlText, watermark.hlSize, hlBox.w),
    watermark.hlSize, watermark.hlDX, watermark.hlDY, hlBox);

  return (
    <Section open={open} onToggle={setOpen}
      title={`${t("groupWatermark")} · ${watermarkOn(watermark) ? t("watermarkOn_") : t("watermarkOff_")}`}>
      <div className="grid3">
        <div className="field"><label>{t("wmImage")} <Tip text={t("wmImageTip")} /></label>
          <div className="field-inline">
            <button className="ghost" title={name || t("wmImagePick")} onClick={onPickImage}>
              <FolderOpen className="ico" aria-hidden="true" /> {name || t("wmImagePick")}
            </button>
            {watermark.image && (
              <button className="ghost tiny icon-only" title={t("wmImageClear")} aria-label={t("wmImageClear")}
                onClick={() => setWatermark({ image: "" })}>
                <X className="ico" aria-hidden="true" />
              </button>
            )}
          </div></div>
        {/* Lebar DAN tinggi: keduanya kotak tempat gambar diletakkan, dalam
            persen sisi bingkai. Gambarnya dimuat utuh ke dalam kotak — tidak
            digepengkan, tidak dipotong — jadi sisi yang lebih longgar cuma jadi
            ruang kosong. Satu angka saja memaksa pengguna menghitung sendiri
            tinggi yang akan muncul dari rasio gambarnya. */}
        <div className="field"><label>{t("wmWidth")} <Tip text={t("wmSizeTip")} /></label>
          <Stepper value={watermark.width} onChange={(v) => setWatermark({ width: v })}
            min={5} max={100} step={1} suffix="%" /></div>
        <div className="field"><label>{t("wmHeight")} <Tip text={t("wmSizeTip")} /></label>
          <Stepper value={watermark.height} onChange={(v) => setWatermark({ height: v })}
            min={5} max={100} step={1} suffix="%" /></div>
        <PositionField label={t("position")} x={watermark.x} y={watermark.y}
          onReset={() => setWatermark({ x: 540, y: 960 })} />

        {/* Waktu tampil. 0 detik durasi = sampai klip habis, dan itu bawaannya:
            syarat kontes lazimnya "identitas harus terlihat", bukan "berkedip". */}
        <div className="field"><label>{t("wmAt")} <Tip text={t("wmAtTip")} /></label>
          <Stepper value={watermark.at} onChange={(v) => setWatermark({ at: v })} min={0} max={60} step={1} suffix="s" /></div>
        <div className="field"><label>{t("wmFor")} <Tip text={t("wmForTip")} /></label>
          <Stepper value={watermark.dur} onChange={(v) => setWatermark({ dur: v })} min={0} max={180} step={1} suffix="s" /></div>
        <div className="field"><label>{t("headlineSize")}</label>
          <Stepper value={watermark.hlSize} onChange={(v) => setWatermark({ hlSize: v })} min={24} max={140} step={2} /></div>

        {allowLLM && (
          <div className="field"><label>{t("headlineSource")} <Tip text={t("headlineSourceTip")} /></label>
            <Select value={watermark.hlSource} onChange={(v) => setWatermark({ hlSource: v as Watermark["hlSource"] })}
              options={[
                { value: "text", label: t("headlineMine") },
                { value: "llm", label: t("headlineLLM") },
              ]} /></div>
        )}
        <div className="field"><label>{t("color")}</label>
          <Select value={watermark.hlColor} onChange={(v) => setWatermark({ hlColor: v })} options={[
            { value: "white", label: t("colorWhite") },
            { value: "yellow", label: t("colorYellow") },
          ]} /></div>
        <div className="field"><label>{t("outline")}</label>
          <Stepper value={watermark.hlOutline} onChange={(v) => setWatermark({ hlOutline: v })} min={0} max={12} /></div>
        {/* Penempatan teksnya SENDIRI, di dalam kotak gambarnya. Tombolnya
            mengembalikan ke TENGAH KOTAK — bukan tengah bingkai — sebab di
            situlah teks itu memang seharusnya berdiri. */}
        <PositionField label={t("headlinePosition")} x={Math.round(hlAt.x)} y={Math.round(hlAt.y)}
          onReset={() => setWatermark({ hlDX: 0, hlDY: 0 })} />
        {gridControl}

        {/* Teks headline melintasi seluruh kisi: ia satu-satunya isian bebas di
            kelompok ini, dan memaksanya masuk sepertiga lebar berarti tidak ada
            satu pun judul yang terlihat utuh saat diketik.

            Dimatikan pada sumber "llm" — kotak yang isinya tidak dipakai lebih
            buruk daripada kotak yang jelas mati. */}
        <div className="field field-wide"><label>{t("headlineText")}</label>
          <input value={watermark.hlText} spellCheck={false} disabled={source === "llm"}
            placeholder={source === "llm" ? t("headlineFromLLM") : t("headlinePlaceholder")}
            onChange={(e) => setWatermark({ hlText: e.target.value })} /></div>
      </div>
    </Section>
  );
}
