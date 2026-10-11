"use client";

import { useEffect, useState } from "react";
import { Film, Upload } from "lucide-react";
import { useI18n } from "./i18n";
import { eng } from "./engine";
import { StepCard, humanDuration } from "./clip-steps";

type Info = { duration: number; width: number; height: number };

// Kartu 1 — Video sumber (DESIGN.md §4.2): thumbnail, NAMA berkas (bukan path
// sistem), durasi dalam kata, resolusi, dan "Ganti video". Tanpa video, kartu
// ini sendiri yang jadi area unggah (§9). "Bahasa terdeteksi" dari mockup
// TIDAK ditampilkan: bahasanya baru diketahui setelah transkripsi, dan angka
// yang belum ada tidak dikarang.
export default function SourceCard({
  path, onPick, onFile, uploading, uploadPct,
}: {
  path: string;
  onPick: () => void;
  onFile: (f: File) => void;
  uploading: boolean;
  uploadPct: number;
}) {
  const { t } = useI18n();
  const [info, setInfo] = useState<Info | null>(null);
  const [over, setOver] = useState(false);

  useEffect(() => {
    setInfo(null);
    if (!path) return;
    const timer = setTimeout(() => {
      fetch(eng(`/api/probe?path=${encodeURIComponent(path)}`))
        .then((r) => (r.ok ? r.json() : null)).then((d) => d && setInfo(d)).catch(() => {});
    }, 400);
    return () => clearTimeout(timer);
  }, [path]);

  const name = path.split(/[\\/]/).pop() || path;
  const res = info?.height ? `${Math.min(info.width, info.height)}p` : "";

  return (
    <StepCard n={1} title={t("step1Title")}>
      {!path ? (
        <div className={"drop-zone" + (over ? " over" : "")}
          onDragOver={(e) => { e.preventDefault(); setOver(true); }}
          onDragLeave={() => setOver(false)}
          onDrop={(e) => { e.preventDefault(); setOver(false); const f = e.dataTransfer.files?.[0]; if (f) onFile(f); }}>
          <Upload className="drop-ico" aria-hidden="true" />
          <div>
            <p className="drop-title">{t("srcEmptyTitle")}</p>
            <p className="drop-hint">{t("srcEmptyHint")}</p>
          </div>
          <button type="button" className="primary" onClick={onPick} disabled={uploading}>{t("chooseVideo")}</button>
          {uploading && (
            <div className="drop-progress" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(uploadPct * 100)}>
              <div style={{ width: `${uploadPct * 100}%` }} />
              <span>{t("uploadingPct", { pct: Math.round(uploadPct * 100) })}</span>
            </div>
          )}
        </div>
      ) : (
        <div className="src-row">
          <div className="src-thumb">
            {info ? (
              /* eslint-disable-next-line @next/next/no-img-element */
              <img alt="" src={eng(`/api/frame?path=${encodeURIComponent(path)}&t=${Math.min(5, info.duration / 2).toFixed(1)}&reframe=fit&background=black&zoom=0`)} />
            ) : <Film className="ico" aria-hidden="true" />}
          </div>
          <div className="src-text">
            <p className="src-name" title={name}>{name}</p>
            <p className="src-meta">{info ? [humanDuration(info.duration, t), res].filter(Boolean).join(" · ") : t("srcReading")}</p>
          </div>
          <button type="button" className="ghost" onClick={onPick}>{t("changeVideo")}</button>
        </div>
      )}
    </StepCard>
  );
}
