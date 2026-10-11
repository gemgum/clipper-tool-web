"use client";

import { useEffect, useState } from "react";
import { Check, Clock, X } from "lucide-react";
import { useI18n } from "./i18n";
import { eng } from "./engine";
import LogPanel from "./log-panel";
import type { JobClip } from "./result-view";

// Layar 2 — Sedang diproses (DESIGN.md §5). Menggantikan "Running · 24%" dan
// kotak log hitam: judul + persen, bilah penuh, perkiraan sisa waktu dalam
// kata, tiga kartu tahap, klip yang sudah jadi muncul satu per satu, dan
// ringkasan setelan. Log mentah tetap ada, dilipat di "Catatan teknis".

// Tahap engine → tiga tahap yang dikenali operator.
const STAGE_OF: Record<string, number> = {
  extracting: 0, transcribing: 0,
  correcting: 1, segmenting: 1, scoring: 1,
  rendering: 2, done: 3,
};

export default function RunView({
  stage, progress, startedAt, total, clips, error, logs, summary, failedStage,
  onRetry, retrying, onChangeSettings,
}: {
  stage: string;
  progress: number;
  /** Waktu job dimulai (ms) — dasar perkiraan sisa waktu. */
  startedAt: number;
  /** Jumlah klip yang diminta. */
  total: number;
  clips: JobClip[];
  error: string;
  logs: string[];
  summary: { label: string; value: string }[];
  /** Tahap tempat job berhenti (0–2) bila gagal; -1 bila tidak. */
  failedStage: number;
  onRetry: () => void;
  retrying: boolean;
  /** Batalkan job lalu kembali ke layar Atur klip. */
  onChangeSettings: () => void;
}) {
  const { t } = useI18n();
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 5000);
    return () => clearInterval(id);
  }, []);

  const active = failedStage >= 0 ? failedStage : (STAGE_OF[stage] ?? 0);
  const pct = Math.round(progress * 100);
  // Perkiraan JUJUR: laju sejauh ini diteruskan ke sisa pekerjaan. Di bawah 5%
  // lajunya belum berarti apa-apa (ekstrak audio sekejap, transkripsi lama),
  // jadi yang ditulis "menghitung", bukan angka karangan.
  const elapsed = (now - startedAt) / 1000;
  const remaining = progress > 0.05 ? (elapsed * (1 - progress)) / progress : -1;
  const eta = remaining < 0 ? t("etaUnknown")
    : remaining < 60 ? t("etaUnderMin")
    : t("etaAbout", { n: Math.round(remaining / 60) });

  const rendered = clips.filter((c) => c.status === "rendered");
  const stages = [
    { name: t("stageTranscribe"), desc: t("stageTranscribeDesc") },
    { name: t("stagePick"), desc: t("stagePickDesc") },
    { name: t("stageRender"), desc: t("stageRenderDesc") },
  ];

  return (
    <div className="run-grid">
      <div className="run-main">
        <section className="card status-card" aria-live="polite">
          <div className="status-top">
            <h2>{t("runTitle")}</h2>
            <span className="status-pct">{pct}%</span>
          </div>
          <div className="bar" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={pct}>
            <div style={{ width: `${pct}%` }} />
          </div>
          {failedStage < 0 && (
            <p className="status-eta">{t("etaLead")} <b>{eta}</b> · {t("runCanLeave")}</p>
          )}
          <div className="stage-grid">
            {stages.map((s, i) => {
              const state = failedStage === i ? "fail" : i < active ? "done" : i === active ? "on" : "wait";
              return (
                <div key={i} className={"stage-card " + state}>
                  <div className="stage-head">
                    <span className="stage-dot" aria-hidden="true">
                      {state === "done" ? <Check className="ico" /> : state === "on" ? <Clock className="ico" /> : state === "fail" ? <X className="ico" /> : null}
                    </span>
                    <span className="stage-name">{s.name}</span>
                  </div>
                  <p>{state === "fail" ? error : s.desc}</p>
                  {state === "fail" && (
                    <>
                      <button type="button" className="primary" onClick={onRetry} disabled={retrying}>{t("retryFromStage")}</button>
                      <p className="stage-note">{t("retryCached")}</p>
                    </>
                  )}
                </div>
              );
            })}
          </div>
        </section>

        <section className="card">
          <div className="card-head">
            <h2>{t("clipsDone")}</h2>
            <span className="meta">{t("nOfM", { n: rendered.length, m: Math.max(total, clips.length) })}</span>
          </div>
          <div className="thumb-grid">
            {Array.from({ length: Math.max(Math.min(total, 10), clips.length) }, (_, i) => {
              const c = clips[i];
              if (!c) return <div key={i} className="thumb wait"><span>{t("waiting")}</span></div>;
              return (
                <a key={c.id} className={"thumb" + (c.status === "failed" ? " failed" : "")}
                  href={c.status === "rendered" ? eng(`/api/jobs/${c.job_id}/clips/${c.id}/file`) : undefined}
                  download={c.status === "rendered" ? `${c.id}.mp4` : undefined}>
                  {c.status === "rendered" && <video src={eng(`/api/jobs/${c.job_id}/clips/${c.id}/file`) + "#t=1"} preload="metadata" muted />}
                  <span className="thumb-badge">{c.status === "failed" ? t("clipFailed") : `${Math.round(c.duration)}s`}</span>
                </a>
              );
            })}
          </div>
          <p className="meta">{t("clipsAppear")}</p>
        </section>

        <details className="card tech">
          <summary>{t("techNotes")}</summary>
          <LogPanel logs={logs} />
        </details>
      </div>

      <aside className="run-side">
        <section className="card">
          <h2>{t("settingsUsed")}</h2>
          <dl className="sum-list">
            {summary.map((r) => (
              <div key={r.label}><dt>{r.label}</dt><dd>{r.value}</dd></div>
            ))}
          </dl>
          <button type="button" className="link-btn" onClick={onChangeSettings}>{t("changeAndRetry")}</button>
        </section>
      </aside>
    </div>
  );
}
