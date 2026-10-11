"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { ChevronRight } from "lucide-react";
import { eng } from "./engine";
import { useI18n, type MessageKey } from "./i18n";
import { useAISettings } from "./ai";
import { useEngines } from "./engine-picker";

// Kaki rail (DESIGN-Navigasi §4): kartu pekerjaan berjalan + tombol status
// mesin AI. Titik selalu berpasangan dengan kalimat — titik saja tidak dibaca
// pembaca layar maupun orang buta warna.

type Kind = "clips" | "writer" | "captions" | "watermark";
export type Running = { kind: Kind; progress: number; created: string };

const KIND_LABEL: Record<Kind, MessageKey> = {
  clips: "navClips", writer: "navWriter", captions: "navCaptions", watermark: "navWatermark",
};

// useRunning: pekerjaan yang sedang jalan di keempat alat yang punya job latar.
// ponytail: polling 5 detik ke empat endpoint; ganti ke SSE bila terasa berat.
export function useRunning(): Running[] {
  const [list, setList] = useState<Running[]>([]);
  useEffect(() => {
    let alive = true;
    const get = (p: string) => fetch(eng(p)).then((r) => r.json()).catch(() => null);
    const poll = async () => {
      const [jobs, posts, caps, wms] = await Promise.all([get("/api/jobs"), get("/api/posts"), get("/api/captions"), get("/api/watermark")]);
      if (!alive) return;
      const out: Running[] = [];
      const add = (kind: Kind, arr: any[] | undefined) => (arr || []).forEach((j) => {
        if (j.status === "running" || j.status === "queued") out.push({ kind, progress: j.progress || 0, created: j.created_at });
      });
      add("clips", Array.isArray(jobs) ? jobs : []);
      add("writer", posts?.posts); add("captions", caps?.captions); add("watermark", wms?.jobs);
      setList(out);
    };
    poll();
    const id = setInterval(poll, 5000);
    return () => { alive = false; clearInterval(id); };
  }, []);
  return list;
}

// Sisa waktu kasar dari kemajuan & lama berjalan; -1 = belum bisa dihitung.
const remaining = (r: Running) => {
  const el = (Date.now() - Date.parse(r.created)) / 1000;
  return r.progress > 0.05 ? (el * (1 - r.progress)) / r.progress : -1;
};

export function RailJob({ running }: { running: Running[] }) {
  const { t } = useI18n();
  if (running.length === 0) return null;
  // Yang paling cepat selesai di depan; yang belum terhitung paling belakang.
  const first = running.slice().sort((a, b) => {
    const ra = remaining(a), rb = remaining(b);
    return (ra < 0 ? Infinity : ra) - (rb < 0 ? Infinity : rb);
  })[0];
  const rem = remaining(first);
  const pct = Math.round(first.progress * 100);
  const eta = rem < 0 ? t("etaUnknown") : rem < 60 ? t("etaUnderMin") : t("etaAbout", { n: Math.round(rem / 60) });
  return (
    <Link href={`/history?f=${first.kind}`} className="rail-job" aria-live="polite">
      <span className="rail-job-head">
        <span className="rail-dot run pulse" aria-hidden="true" />
        {running.length === 1 ? t("navRunningOne") : t("navRunning", { n: running.length })}
      </span>
      <span className="rail-job-bar" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={pct}>
        <span style={{ width: `${pct}%` }} />
      </span>
      <span className="rail-job-meta">
        {[t(KIND_LABEL[first.kind]), `${pct}%`, eta].join(" · ")}
        {running.length > 1 && ` · ${t("navRunningMore", { n: running.length - 1 })}`}
      </span>
    </Link>
  );
}

export function RailAI() {
  const { t } = useI18n();
  const { data } = useAISettings();
  const { engines } = useEngines();
  const g = data?.global;
  if (!g) return null;
  const info = engines.find((e) => e.id === g.engine);
  const ready = g.engine === "heuristic" || !!info?.ready;
  const state = ready ? "ok" : info && !info.has_key ? "idle" : "bad";
  const line1 = state === "ok" ? t("railAIReady") : state === "idle" ? t("navAINotSet") : t("navAIDown");
  // "Custom (OpenAI-compatible)" → "Custom": keterangan dalam kurung tidak muat
  // di 220px dan tidak dibutuhkan untuk mengenali mesinnya.
  const name = (info?.name || g.engine).replace(/\s*\(.*\)$/, "");
  const line2 = state === "ok" ? [name, g.model].filter(Boolean).join(" · ")
    : state === "idle" ? t("navAINotSetHint") : t("navAIDownHint");
  return (
    <Link href="/requirements" className={"rail-ai " + state} aria-label={`${line1}. ${line2}`} data-tip={line1}>
      <span className={"rail-dot " + state} aria-hidden="true" />
      <span className="rail-ai-short" aria-hidden="true">AI</span>
      <span className="rail-ai-text">
        <span className="rail-ai-1">{line1}</span>
        <span className="rail-ai-2">{line2}</span>
      </span>
      <ChevronRight className="rail-ai-go" aria-hidden="true" />
    </Link>
  );
}
