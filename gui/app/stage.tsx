"use client";

// Kolom pratinjau BAKU — kartu yang menempel di kanan halaman klip, dipakai
// semua halaman yang punya pratinjau (klip, watermark, kartu berita).
//
// Aturan yang dijaga di sini, bukan di tiap halaman: pratinjau TIDAK BOLEH
// bergerak atau bergetar.
//   - lebar kolom tetap (--stage-w), bukan mengikuti isi;
//   - bingkai duduk di slot setinggi bingkai 9:16 (StageFrame) — 4:5 dan 1:1
//     ditaruh di tengah slot yang sama, jadi berganti format tidak menggeser
//     apa pun di bawahnya;
//   - baris bantuan selalu ada dan tingginya dicadangkan, walau teksnya kosong;
//   - kolomnya sticky, jadi tetap terlihat saat setelan di kiri digulir.

import { useEffect, useState } from "react";
import { ChevronLeft, ChevronRight, X } from "lucide-react";
import { useI18n } from "./i18n";
import { GridPicker } from "./guides";
import Select from "./select";
import { useEscape } from "./use-popup";

// Lebar BINGKAI (--stage-in di globals.css): lebar kolom (340, atau 280 di
// jendela <= 1100 px) dikurangi 50 px padding & tepi kartu, dibatasi tinggi
// layar. RUMUSNYA HARUS SAMA dengan CSS — halaman yang menghitung ukuran huruf
// pratinjau dari ukuran bingkai butuh angkanya di JS. Berubah hanya saat
// jendela diubah ukurannya, tidak pernah karena isi.
const frameWidth = () => {
  const col = window.innerWidth <= 1100 ? 280 : 340;
  return Math.round(Math.max(120, Math.min(col - 50, ((window.innerHeight - 430) * 9) / 16)));
};

export function useStageWidth() {
  const [w, setW] = useState(290);
  useEffect(() => {
    const on = () => setW(frameWidth());
    on(); window.addEventListener("resize", on);
    return () => window.removeEventListener("resize", on);
  }, []);
  return w;
}

// action = tombol di tempat judul (mis. "Preview result"); title tetap jadi
// nama kartunya untuk pembaca layar.
export default function Stage({ title, action, badge, tools, help, bad, children }: {
  title: string; action?: React.ReactNode; badge?: string; tools?: React.ReactNode; help?: React.ReactNode; bad?: boolean; children: React.ReactNode;
}) {
  return (
    <aside className="setup-side" aria-label={title}>
      <section className="card preview-card stage">
        <div className="card-head">
          {action ?? <h2>{title}</h2>}
          {badge && <span className="chip">{badge}</span>}
        </div>
        {children}
        <div className="stage-tools">{tools}</div>
        <p className={"pv-help stage-help" + (bad ? " bad" : "")}>{help}</p>
      </section>
    </aside>
  );
}

// StageFrame = slot tetap setinggi bingkai 9:16; bingkai apa pun di tengahnya.
// StageBar = SATU baris alat DI ATAS bingkai, sama untuk Mark & News
// (keputusan pemilik 25 Oktober 2026): [lead] · kisi + garis tengah · rasio.
// Di atas bingkai, jadi tidak bergeser saat rasio berganti. lead = alat milik
// halaman (pager carousel di News, alat posisi subtitle di Clips). ratio
// kosong = bingkai berukuran tetap (Clips selalu 9:16).
export function StageBar({ lead, grid, ratio }: {
  lead?: React.ReactNode;
  grid: { grid: number; setGrid: (v: number) => void; always: boolean; setAlways: (v: boolean) => void };
  ratio?: { label: string; value: string; options: { value: string; label: string }[]; onChange: (v: string) => void };
}) {
  return (
    <div className="stage-bar">
      {lead}
      <GridPicker inline {...grid} />
      {ratio && (
        <div className="stage-bar-sel">
          <Select value={ratio.value} onChange={ratio.onChange} ariaLabel={ratio.label} title={ratio.label} options={ratio.options} />
        </div>
      )}
    </div>
  );
}

export function StageFrame({ children }: { children: React.ReactNode }) {
  return <div className="stage-slot">{children}</div>;
}

// Shot = hasil render sungguhan (tombol "Preview result") di atas layar penuh;
// klik di mana saja atau Escape menutupnya. SATU jendela untuk Watermark & News.
// pager (carousel News) = ‹ › + "3 / 8" di dalam jendela, juga panah keyboard;
// busy = halaman itu sedang dirender (gambar lama diredupkan, bukan dikosongkan).
export function Shot({ src, label, onClose, pager }: {
  src: string; label: string; onClose: () => void;
  pager?: { page: number; pages: number; onPage: (p: number) => void; busy?: boolean };
}) {
  const { t } = useI18n();
  useEscape(onClose);
  useEffect(() => {
    if (!pager) return;
    const key = (e: KeyboardEvent) => {
      if (e.key === "ArrowLeft" && pager.page > 0) pager.onPage(pager.page - 1);
      if (e.key === "ArrowRight" && pager.page < pager.pages - 1) pager.onPage(pager.page + 1);
    };
    window.addEventListener("keydown", key);
    return () => window.removeEventListener("keydown", key);
  }, [pager]);
  const stop = (e: React.MouseEvent) => e.stopPropagation();
  return (
    <div className={"stage-shot" + (pager && pager.pages > 1 ? " paged" : "")} role="dialog" aria-label={label} onClick={onClose}>
      {/* eslint-disable-next-line @next/next/no-img-element */}
      {src ? <img src={src} alt={label} className={pager?.busy ? "busy" : ""} /> : <span className="stage-shot-wait">{t("wmPreviewing")}</span>}
      {pager?.busy && src && <span className="stage-shot-wait over">{t("wmPreviewing")}</span>}
      {pager && pager.pages > 1 && (
        <div className="stage-shot-pager" onClick={stop}>
          <button type="button" className="ghost icon-only" aria-label={t("ncPagePrev")} disabled={pager.page === 0} onClick={() => pager.onPage(pager.page - 1)}>
            <ChevronLeft className="ico" aria-hidden="true" /></button>
          <span>{pager.page + 1} / {pager.pages}</span>
          <button type="button" className="ghost icon-only" aria-label={t("ncPageNext")} disabled={pager.page >= pager.pages - 1} onClick={() => pager.onPage(pager.page + 1)}>
            <ChevronRight className="ico" aria-hidden="true" /></button>
        </div>
      )}
      <button type="button" className="ghost stage-shot-close" aria-label={t("close")} title={t("close")}><X className="ico" aria-hidden="true" /></button>
    </div>
  );
}
