"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { eng } from "./engine";
import { useI18n } from "./i18n";
import { useAISettings } from "./ai";
import { useEngines } from "./engine-picker";

// Kaki rail: status mesin AI sebagai satu titik. Titik selalu berpasangan
// dengan kalimat (aria-label + tooltip) — titik saja tidak dibaca pembaca layar
// maupun orang buta warna. Pekerjaan berjalan tampil sebagai lencana Riwayat.

type Kind = "clips" | "writer" | "captions" | "watermark";
export type Running = { kind: Kind; progress: number; created: string };


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
    <Link href="/requirements" className={"rail-ai " + state} aria-label={`${line1}. ${line2}`} data-tip={`${line1} · ${line2}`}>
      <span className={"rail-dot " + state} aria-hidden="true" />
    </Link>
  );
}
