"use client";

import { useCallback, useEffect, useState } from "react";
import { eng } from "./engine";

// Mesin AI GLOBAL (DESIGN-Clipper-Lanjutan.md §1): satu mesin & model untuk
// semua alat, pengecualian per alat diatur di halaman Pengaturan. Halaman alat
// TIDAK lagi punya pemilih mesin; mereka membaca pilihan efektifnya di sini
// dan mengirimnya bersama permintaan kerja seperti sebelumnya.

export type AITool = "clips" | "news" | "writer" | "captions";
export type AIChoice = { engine: string; model: string };
export type AISettings = {
  global: AIChoice;
  overrides: Record<AITool, AIChoice | null>;
  effective: Record<AITool, AIChoice>;
  tools: AITool[];
  // Mesin transkripsi job klip (notes/43). Kunci AI Studio = kunci Gemini.
  transcriber: { engine: string; model: string; key_set: boolean }; // whisper | aistudio | id mesin
};

// Satu permintaan untuk seluruh halaman yang terbuka; disegarkan saat jendela
// kembali fokus (pilihan bisa diubah di tab lain).
let cache: Promise<AISettings> | null = null;
export function loadAISettings(fresh = false): Promise<AISettings> {
  if (!cache || fresh) {
    cache = fetch(eng("/api/settings/ai")).then((r) => r.json());
    cache.catch(() => { cache = null; });
  }
  return cache;
}

export async function saveAI(tool: AITool | "transcribe" | "", engine: string, model: string): Promise<AISettings> {
  const res = await fetch(eng("/api/settings/ai"), {
    method: "POST", headers: { "content-type": "application/json" },
    body: JSON.stringify({ tool, engine, model }),
  });
  const d = await res.json();
  if (!res.ok) throw new Error(d.error || "save failed");
  cache = Promise.resolve(d);
  // Semua yang menampilkan pilihan ini (status rail, halaman alat) ikut
  // berubah seketika, bukan menunggu jendela difokus ulang.
  window.dispatchEvent(new CustomEvent("clipper:ai", { detail: d }));
  return d;
}

export function useAISettings() {
  const [data, setData] = useState<AISettings | null>(null);
  const reload = useCallback((fresh = true) => {
    loadAISettings(fresh).then(setData).catch(() => {});
  }, []);
  useEffect(() => {
    loadAISettings().then(setData).catch(() => {});
    const onFocus = () => reload(true);
    const onSaved = (e: Event) => setData((e as CustomEvent<AISettings>).detail);
    window.addEventListener("focus", onFocus);
    window.addEventListener("clipper:ai", onSaved);
    return () => { window.removeEventListener("focus", onFocus); window.removeEventListener("clipper:ai", onSaved); };
  }, [reload]);
  return { data, setData, reload };
}

/** Pilihan mesin efektif untuk satu alat; null selama belum termuat. */
export function useAI(tool: AITool): AIChoice | null {
  const { data } = useAISettings();
  return data?.effective?.[tool] ?? null;
}
