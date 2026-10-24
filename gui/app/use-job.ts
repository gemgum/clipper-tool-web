"use client";

// useJob = siklus SATU job latar untuk halaman Watermark, Captions, Writer.
// Dulu tiga salinan kata per kata: state job/log, langganan SSE, start, cancel.
// Bedanya cuma alamat API, nama event SSE, dan apa yang dikerjakan saat selesai.
//
// GUI hanya memulai lalu BERLANGGANAN (CLAUDE.md "GUI tidak mengendalikan apa
// pun yang memakan waktu"): satu EventSource untuk seluruh halaman, dibuka
// sekali, dan kabar job yang sudah berjalan sebelum halaman dibuka ikut diambil.

import { useEffect, useRef, useState } from "react";
import { eng, post } from "./engine";

export type Job<R> = {
  id: string;
  status: string;
  stage: string;
  progress: number;
  log?: string[];
  error?: string;
  result?: R;
};

// api = "/api/watermark" (POST memulai, /events = SSE, /<id>/cancel);
// event = nama event SSE-nya; onDone dipanggil sekali saat job selesai.
export function useJob<R>(api: string, event: string, setError: (msg: string) => void, onDone?: (job: Job<R>) => void) {
  const [jobId, setJobId] = useState("");
  const [job, setJob] = useState<Job<R> | null>(null);
  const [logs, setLogs] = useState<string[]>([]);
  // Sibuk SEJAK tombol ditekan, bukan sejak engine menjawab: POST pembuka bisa
  // tertahan beberapa detik, dan klik kedua di sela itu dulu memulai job kedua
  // (terukur 24 Oktober 2026: dua job caption berselisih 0,1 detik).
  const [starting, setStarting] = useState(false);
  // onDone terbaru tanpa membuka ulang langganan setiap render.
  const done = useRef(onDone);
  done.current = onDone;

  useEffect(() => {
    const es = new EventSource(eng(`${api}/events`));
    es.addEventListener(event, (ev) => {
      const j: Job<R> = JSON.parse((ev as MessageEvent).data);
      setJobId((cur) => {
        // Kabar job lain (tab lain, pengguna lain) diabaikan.
        if (cur && j.id !== cur) return cur;
        setJob(j);
        if (j.log) setLogs(j.log);
        if (j.error) setError(j.error);
        if (j.status === "done") done.current?.(j);
        return cur || j.id;
      });
    });
    return () => es.close();
  }, [api, event, setError]);

  const start = async (body: unknown) => {
    if (starting) return;
    setError(""); setLogs([]); setJob(null); setJobId(""); setStarting(true);
    try {
      const d = await post<{ id: string }>(api, body, "failed to start");
      setJobId(d.id);
    } catch (e) {
      setError(String(e));
    } finally {
      setStarting(false);
    }
  };

  const cancel = async () => {
    if (!jobId) return;
    try {
      await post(`${api}/${jobId}/cancel`);
    } catch (e) {
      setError(String(e));
    }
  };

  // Sudah dimulai tapi kabar pertama belum datang = sibuk juga.
  const busy = starting || job?.status === "running" || (!!jobId && !job);
  return { jobId, job, logs, busy, start, cancel };
}
