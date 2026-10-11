"use client";

// Ikon: lucide-react (ISC) — alasannya di gui/app/page.tsx.
import { Crosshair } from "lucide-react";

import { useI18n } from "./i18n";
import Select from "./select";
import { CENTER_X, PLAY_H, PLAY_W } from "./drag";
import type { Point } from "./drag";

// Penanda penempatan di atas bingkai pratinjau: kisi, garis tengah, dan sumbu
// berikut angkanya saat sesuatu sedang digeser.
//
// Dipakai DUA halaman (klip & watermark), dan itu alasannya berdiri sendiri.
// Sempat hanya halaman klip yang punya, dan akibatnya bukan sekadar kurang
// rapi: di halaman watermark tidak ada satu pun tanda bahwa gambar dan teksnya
// BISA dipegang — dilaporkan sebagai "tidak ada penanda grid tengah", dan
// pertanyaan berikutnya adalah "teksnya tidak bisa dipindah?" padahal bisa.

// Pilihan kerapatan kisi di ruang 1080x1920. 0 = mati.
//
// 20 jadi bawaan: 54x96 kotak — cukup kasar untuk membuat penempatan bisa
// diulang, cukup halus untuk tidak terasa seperti pagar. Yang lain disediakan
// karena "rasa" kisi hanya bisa dinilai dengan mencoba, bukan dari angka.
export const GRIDS = [0, 10, 20, 24, 40] as const;

export default function Guides({
  grid, visible, atX, atY, dragAt,
}: {
  grid: number;
  /** Garis tengah & kisi terlihat (sedang digeser, atau dikunci pengguna). */
  visible: boolean;
  atX: boolean;
  atY: boolean;
  /** Titik yang sedang dipegang; null = tidak ada yang digeser. */
  dragAt: Point | null;
}) {
  return (
    <>
      {visible && grid >= 20 && (
        /* Kotak kisi digambar sebagai latar berulang, bukan puluhan elemen:
           pada kisi 10 itu 108x192 kotak, dan menggambarnya satu per satu
           berarti 300 elemen yang harus dihitung ulang tiap seretan.

           Di bawah 20 meshnya TIDAK digambar: pratinjau selebar 270 px berarti
           kotak kisi 10 hanya 2,5 px di layar, dan yang terlihat bukan kisi
           melainkan kabut kelabu di atas frame videonya. Yang menempel tetap
           menempel — cuma gambarnya yang tidak menolong. */
        <div className="gridmesh" style={{
          backgroundSize: `${(grid / PLAY_W) * 100}% ${(grid / PLAY_H) * 100}%`,
        }} />
      )}
      {visible && (
        <>
          <div className={`guide v${atX ? " on" : ""}`} />
          <div className={`guide h${atY ? " on" : ""}`} />
          {atX && atY && <div className="guide xy" />}
        </>
      )}
      {/* Sumbu pada posisi yang SEKARANG dipegang, berikut angkanya. Tanpa
          angka, "pasti" hanya terasa — tidak bisa diulang di klip berikutnya. */}
      {dragAt && (
        <>
          <div className="axis v" style={{ left: `${(dragAt.x / PLAY_W) * 100}%` }} />
          <div className="axis h" style={{ top: `${(dragAt.y / PLAY_H) * 100}%` }} />
          <div className="axis-read" style={{
            left: `${(dragAt.x / PLAY_W) * 100}%`, top: `${(dragAt.y / PLAY_H) * 100}%`,
          }}>{dragAt.x} · {dragAt.y}</div>
        </>
      )}
    </>
  );
}

// Kendali kisi: kerapatannya + tombol yang mengunci garis panduan tetap terlihat.
export function GridPicker({
  grid, setGrid, always, setAlways,
}: {
  grid: number;
  setGrid: (v: number) => void;
  always: boolean;
  setAlways: (v: boolean) => void;
}) {
  const { t } = useI18n();
  return (
    <div className="field"><label>{t("grid")}</label>
      <div className="field-inline">
        <Select value={String(grid)} onChange={(v) => setGrid(Number(v))}
          options={GRIDS.map((g) => ({ value: String(g), label: g === 0 ? t("gridOff") : String(g) }))} />
        <button className={"ghost tiny icon-only" + (always ? " active" : "")}
          aria-pressed={always} title={t("guidesAlways")} aria-label={t("guidesAlways")}
          onClick={() => setAlways(!always)}>
          <Crosshair className="ico" aria-hidden="true" />
        </button>
      </div></div>
  );
}

// Satu titik penempatan sebagai ANGKA, plus tombol kembali ke tengah bidang.
//
// Menyeret memberi rasa; angka memberi hal yang bisa diulang. Keduanya perlu:
// tanpa angka tidak ada cara menaruh watermark di tempat yang sama persis pada
// job berikutnya, dan tanpa tombol tengah tidak ada jalan pulang.
export function PositionField({
  label, x, y, onReset,
}: {
  // ReactNode, bukan string: label posisi subtitle membawa lambang peringatan
  // "menabrak zona" di sebelahnya.
  label: React.ReactNode;
  x: number;
  y: number;
  onReset: () => void;
}) {
  const { t } = useI18n();
  return (
    <div className="field"><label>{label}</label>
      <div className="position-value" title={`x ${x} · y ${y}`}>
        {/* Tanpa spasi di sekitar titiknya. Terukur: pada koordinat terjauh
            ("1080 · 1920") bentuk berspasi meleset 3 px dari selnya dan
            terpotong jadi "1080 · 19…" — dan angka yang terpotong tidak lebih
            berguna daripada tidak ada angka sama sekali. */}
        <span>{x}·{y}</span>
        <button className="ghost tiny" title={t("resetCentre")} aria-label={t("resetCentre")}
          onClick={onReset}>
          <Crosshair className="ico" aria-hidden="true" />
        </button>
      </div></div>
  );
}

export type { Point };
