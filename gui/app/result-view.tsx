"use client";

import { useEffect, useMemo, useState } from "react";
import { Check, Download, Play, RotateCw } from "lucide-react";
import { useI18n } from "./i18n";
import { eng } from "./engine";

// Bentuk klip yang dikirim engine (types.Clip) — hanya field yang dipakai GUI.
export type JobClip = {
  id: string; job_id: string; start: number; end: number; duration: number;
  score: number; title: string; transcript: string; status: string; error?: string;
};

// Kuat / Sedang / Biasa: warna DAN kata, tidak pernah warna saja (DESIGN.md §6).
export function scoreBand(score: number): "strong" | "medium" | "plain" {
  return score >= 75 ? "strong" : score >= 60 ? "medium" : "plain";
}

const fmt = (sec: number) => {
  const s = Math.max(0, Math.floor(sec));
  return `${String(Math.floor(s / 60)).padStart(2, "0")}:${String(s % 60).padStart(2, "0")}`;
};

// Kalimat pembuka klip: kalimat pertama ucapannya, dipotong di 110 huruf.
const opening = (text: string) => {
  const first = (text || "").trim().split(/(?<=[.?!])\s/)[0] || "";
  return first.length > 110 ? first.slice(0, 107).trimEnd() + "…" : first;
};

// Klip yang sudah diunduh dicatat di browser ini saja — itu satu-satunya
// tempat yang tahu, dan filter "Belum diunduh" memang soal perangkat ini.
const DL_KEY = "clipper.downloaded";
const loadDownloaded = (): Set<string> => {
  try { return new Set(JSON.parse(localStorage.getItem(DL_KEY) || "[]")); } catch { return new Set(); }
};

// Layar 3 — Hasil klip (DESIGN.md §6, §9).
export default function ResultView({
  clips, platformName, onRerender, rerendering,
}: {
  clips: JobClip[];
  platformName: string;
  onRerender: () => void;
  rerendering: boolean;
}) {
  const { t } = useI18n();
  const [filter, setFilter] = useState<"all" | "strong" | "new">("all");
  const [downloaded, setDownloaded] = useState<Set<string>>(() => new Set());
  const [bytes, setBytes] = useState(0);
  useEffect(() => { setDownloaded(loadDownloaded()); }, []);

  const ok = useMemo(() => clips.filter((c) => c.status !== "failed").sort((a, b) => b.score - a.score), [clips]);
  const failed = clips.filter((c) => c.status === "failed");
  const key = (c: JobClip) => `${c.job_id}/${c.id}`;
  const fileURL = (c: JobClip) => eng(`/api/jobs/${c.job_id}/clips/${c.id}/file`);

  // Total ukuran dari Content-Length tiap berkas — HEAD, tanpa mengunduhnya.
  useEffect(() => {
    let alive = true;
    Promise.all(ok.map((c) => fetch(fileURL(c), { method: "HEAD" })
      .then((r) => Number(r.headers.get("content-length")) || 0).catch(() => 0)))
      .then((sizes) => { if (alive) setBytes(sizes.reduce((a, b) => a + b, 0)); });
    return () => { alive = false; };
  }, [ok]); // eslint-disable-line react-hooks/exhaustive-deps

  const markDownloaded = (keys: string[]) => {
    setDownloaded((prev) => {
      const next = new Set(prev); keys.forEach((k) => next.add(k));
      try { localStorage.setItem(DL_KEY, JSON.stringify([...next])); } catch {}
      return next;
    });
  };

  const shown = ok.filter((c) => filter === "all" ? true
    : filter === "strong" ? scoreBand(c.score) === "strong" : !downloaded.has(key(c)));
  const strongN = ok.filter((c) => scoreBand(c.score) === "strong").length;
  const newN = ok.filter((c) => !downloaded.has(key(c))).length;
  const size = bytes > 0 ? (bytes > 1e9 ? `${(bytes / 1e9).toFixed(1)} GB` : `${Math.round(bytes / 1e6)} MB`) : "";

  if (ok.length === 0 && failed.length === 0) {
    return (
      <section className="banner neutral" role="status">
        <p><b>{t("noClipsTitle")}</b> {t("noClipsHint")}</p>
      </section>
    );
  }

  return (
    <div className="result">
      {ok.length > 0 && (
        <section className="banner good" role="status">
          <span className="banner-ico" aria-hidden="true"><Check className="ico" /></span>
          <p><b>{ok.length === 1 ? t("resultReadyOne") : t("resultReady", { n: ok.length })}</b> {t("resultReadyMore", { platform: platformName })}</p>
          {size && <span className="banner-side">{t("totalSize", { size })}</span>}
        </section>
      )}
      {failed.length > 0 && (
        <section className="banner warn" role="status">
          <p>{failed.map((c) => `${c.id}: ${c.error || t("clipFailed")}`).join(" · ")}</p>
          <button type="button" className="primary" onClick={onRerender} disabled={rerendering}>
            <RotateCw className="ico" aria-hidden="true" /> {t("rerenderFailed", { n: failed.length })}
          </button>
        </section>
      )}

      <div className="result-bar">
        <h2>{t("sortedByScore")}</h2>
        <div className="filters" role="group">
          {([["all", t("filterAll", { n: ok.length })], ["strong", t("filterStrong", { n: strongN })],
            ["new", t("filterNotDownloaded", { n: newN })]] as const).map(([v, label]) => (
            <button key={v} type="button" className={filter === v ? "on" : ""} aria-pressed={filter === v}
              onClick={() => setFilter(v)}>{label}</button>
          ))}
        </div>
      </div>

      <div className="clip-gallery">
        {shown.map((c) => {
          const band = scoreBand(c.score);
          const no = String(ok.indexOf(c) + 1).padStart(2, "0");
          return (
            <article key={key(c)} className="g-card">
              <div className="g-thumb">
                <video src={fileURL(c) + "#t=1"} preload="metadata" muted id={`v-${c.id}`} />
                <span className="g-no">{no}</span>
                <span className={"g-score " + band}>{t(band === "strong" ? "scoreStrong" : band === "medium" ? "scoreMedium" : "scorePlain")}</span>
                <span className="g-dur">{Math.round(c.duration)}s</span>
              </div>
              <div className="g-body">
                <p className="g-quote" title={c.title || undefined}>“{opening(c.transcript)}”</p>
                <p className="g-meta">{t("minuteInSource", { t: fmt(c.start) })}</p>
                <div className="g-actions">
                  <a className="btn-ghost" href={fileURL(c)} download={`${c.id}.mp4`} onClick={() => markDownloaded([key(c)])}>
                    <Download className="ico" aria-hidden="true" /> {t("downloadClip")}
                  </a>
                  <button type="button" className="ghost icon-only" aria-label={t("playClip", { n: no })}
                    onClick={() => {
                      const v = document.getElementById(`v-${c.id}`) as HTMLVideoElement | null;
                      if (!v) return;
                      v.controls = true; v.muted = false; v.play().catch(() => {});
                    }}>
                    <Play className="ico" aria-hidden="true" />
                  </button>
                </div>
              </div>
            </article>
          );
        })}
      </div>
    </div>
  );
}

/** Alamat zip "Unduh semua" untuk klip yang berhasil dirender. */
export function downloadAllURL(clips: JobClip[]): string {
  const q = clips.filter((c) => c.status !== "failed").map((c) => `clip=${encodeURIComponent(`${c.job_id}/${c.id}`)}`).join("&");
  return eng(`/api/download?${q}`);
}
