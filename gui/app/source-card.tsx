"use client";

import { useEffect, useState } from "react";
import { Film, FolderOpen, Link2, Trash2, TriangleAlert, Upload } from "lucide-react";
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
  path, onPick, onRemove, onFile, onUrl, fetchPhase, fetchTitle, fetchWarn, uploading, uploadPct,
}: {
  path: string;
  onPick: () => void;
  onRemove: () => void;
  onFile: (f: File) => void;
  onUrl: (url: string) => void;
  fetchPhase: "" | "checking" | "downloading";
  fetchTitle: string;
  fetchWarn: string;
  uploading: boolean;
  uploadPct: number;
}) {
  const { t } = useI18n();
  const [info, setInfo] = useState<Info | null>(null);
  const [over, setOver] = useState(false);
  const [url, setUrl] = useState("");
  const fetching = fetchPhase !== "";

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

  // Sudah ada video: satu baris ringkas tanpa kepala kartu.
  if (path) return (
    <section className="step-card src-card">
      <div className="src-row">
        <div className="src-thumb">
          {info ? (
            /* eslint-disable-next-line @next/next/no-img-element */
            <img alt="" src={eng(`/api/frame?path=${encodeURIComponent(path)}&t=${Math.min(5, info.duration / 2).toFixed(1)}&reframe=fit&background=black&zoom=0`)} />
          ) : <Film className="ico" aria-hidden="true" />}
        </div>
        <div className="src-text">
          <h2 className="src-k">{t("step1Title")}</h2>
          <p className="src-name" title={name}>{name}</p>
          <p className="src-meta">{info ? [humanDuration(info.duration, t), res].filter(Boolean).join(" · ") : t("srcReading")}</p>
        </div>
        <div className="src-actions">
          <button type="button" className="ghost" onClick={onPick}><Upload className="ico" aria-hidden="true" /> {t("changeVideo")}</button>
          <button type="button" className="ghost icon-only" onClick={onRemove} aria-label={t("removeVideo")} title={t("removeVideo")}>
            <Trash2 className="ico" aria-hidden="true" />
          </button>
        </div>
      </div>
    </section>
  );

  return (
    <StepCard icon={FolderOpen} title={t("step1Title")}>
      <div className={"drop-zone" + (over ? " over" : "")}
        onDragOver={(e) => { e.preventDefault(); setOver(true); }}
        onDragLeave={() => setOver(false)}
        onDrop={(e) => { e.preventDefault(); setOver(false); const f = e.dataTransfer.files?.[0]; if (f) onFile(f); }}>
        <Upload className="drop-ico" aria-hidden="true" />
        <div>
          <p className="drop-title">{t("srcEmptyTitle")}</p>
          <p className="drop-hint">{t("srcEmptyHint")}</p>
        </div>
        <button type="button" className="primary" onClick={onPick} disabled={uploading || fetching}>{t("chooseVideo")}</button>
        <form className="src-url" onSubmit={(e) => { e.preventDefault(); if (url.trim()) onUrl(url.trim()); }}>
          <Link2 className="ico" aria-hidden="true" />
          <input type="url" value={url} onChange={(e) => setUrl(e.target.value)} disabled={fetching || uploading}
            placeholder="https://youtu.be/…" aria-label={t("srcUrlLabel")} />
          <button type="submit" className="ghost" disabled={fetching || uploading || !url.trim()}>
            {fetchPhase === "checking" ? t("srcChecking") : fetchPhase === "downloading" ? t("srcFetching") : t("srcUrlGo")}
          </button>
        </form>
        {fetchPhase === "downloading" && fetchTitle && <p className="src-url-note">{t("srcFetchingTitle", { title: fetchTitle })}</p>}
        {fetchWarn && !fetching && (
          <p className="src-url-warn" role="alert">
            <TriangleAlert className="ico" aria-hidden="true" />
            <span><b>{t("srcCannotFetch")}</b> {fetchWarn}</span>
          </p>
        )}
        {uploading && (
          <div className="drop-progress" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(uploadPct * 100)}>
            <div style={{ width: `${uploadPct * 100}%` }} />
            <span>{t("uploadingPct", { pct: Math.round(uploadPct * 100) })}</span>
          </div>
        )}
      </div>
    </StepCard>
  );
}
