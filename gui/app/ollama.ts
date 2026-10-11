"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { eng } from "./engine";

// Satu sumber kebenaran untuk "Ollama itu apa, di mana, dan model apa saja".
//
// Dua halaman menanyakan hal yang sama ke engine — tab klip dan tab kartu — dan
// sebelum ini keduanya menulis jawabannya sendiri-sendiri. Akibatnya terlihat:
// daftar di tab klip menyebut ukuran, jumlah parameter, dan siap/tidaknya tiap
// model, sedangkan daftar di tab kartu cuma menampilkan namanya. Dua tampilan
// untuk data yang sama persis, dan yang satu tidak pernah ikut diperbaiki.

export type OllamaModel = {
  name: string; base: string; params: string; quant: string;
  bytes: number; context: number; ready: boolean; note: string;
};

export type OllamaStatus = {
  running: boolean;
  /** Alamat yang benar-benar menjawab + di sistem mana ia berjalan. */
  url?: string;
  where?: string;
  /** Satu kata: "Windows" | "WSL" | "Linux" | "macOS" | "remote". */
  os?: string;
  /** Nama server yang dikenali pengguna: "Ollama", "LM Studio", "Jan", … */
  server?: string;
  /** Protokolnya: "ollama" atau "openai". Menentukan apa yang BISA dilakukan
   *  aplikasi ini — hanya Ollama yang modelnya bisa diunduh dari sini. */
  kind?: string;
  models?: string[];
  installed?: OllamaModel[];
};

/** Nama tanpa tag: "qwen2.5:latest" → "qwen2.5". */
export const baseName = (m: string) => (m.includes(":") ? m.slice(0, m.indexOf(":")) : m);
export const sameModel = (a: string, b: string) => a === b || baseName(a) === baseName(b);

const sizeGB = (b: number) => (b > 0 ? `${(b / 1e9).toFixed(1)} GB` : "");

// Saran bila belum ada satu pun model terpasang. Urutan = urutan saran;
// llama3.1 lebih dulu karena untuk koreksi transkrip ia terukur lebih baik
// daripada qwen2.5 (notes/22).
export const SUGGESTED = ["llama3.1", "qwen2.5", "gemma2"];

/**
 * useOllama menarik status Ollama dan menjaganya tetap segar.
 *
 * Disegarkan berkala DAN saat jendela kembali fokus: pengguna sering menyalakan
 * Ollama atau menarik model di terminal sebelah, dan halaman yang tidak pernah
 * bertanya lagi akan terus mengatakan "tidak ada" sampai dimuat ulang.
 */
export function useOllama(active: boolean) {
  const [status, setStatus] = useState<OllamaStatus | null>(null);

  const check = useCallback((silent = false) => {
    if (!silent) setStatus(null);
    fetch(eng(`/api/ollama/status`))
      .then((r) => r.json())
      .then(setStatus)
      .catch(() => setStatus({ running: false }));
  }, []);

  useEffect(() => {
    if (!active) return;
    check();
    const timer = setInterval(() => check(true), 15000);
    const onFocus = () => check(true);
    window.addEventListener("focus", onFocus);
    return () => { clearInterval(timer); window.removeEventListener("focus", onFocus); };
  }, [active, check]);

  const installed = useMemo<OllamaModel[]>(() => {
    if (status?.installed?.length) return status.installed;
    // Engine versi lama hanya mengirim daftar nama.
    return (status?.models || []).map((n) => ({
      name: n, base: baseName(n), params: "", quant: "", bytes: 0, context: 0, ready: true, note: "",
    }));
  }, [status]);

  return { status, installed, check };
}

/**
 * modelOptions membangun daftar pilihan yang SAMA di mana pun ia dipakai:
 * yang terpasang lebih dulu (dengan spesifikasi & keadaannya), lalu saran yang
 * belum ada.
 */
export function modelOptions(
  installed: OllamaModel[],
  labels: { ready: string; notCapable: string; needsDownload: string },
  /** Sistem tempat Ollama-nya berjalan ("WSL", "Windows", …) — ikut di tiap baris. */
  os?: string,
  /**
   * Boleh menawarkan model yang belum ada? Hanya untuk Ollama.
   *
   * Tombol unduhnya memanggil `/api/ollama/pull`, dan itu API Ollama. Pada
   * llama.cpp, LM Studio, atau KoboldCpp, baris "needs download" adalah janji
   * yang tidak bisa ditepati aplikasi ini — modelnya diurus aplikasi itu
   * sendiri, atau berkas .gguf yang dipilih pengguna.
   */
  canPull = true,
) {
  return [
    ...installed.map((m) => ({
      value: m.name,
      // Nilainya TETAP nama asli — itu yang dikirim balik ke servernya. Yang
      // dipendekkan cuma labelnya, dan hanya bila nama aslinya sebuah path:
      // llama.cpp memakai path lengkap berkas .gguf sebagai nama model, dan 60
      // karakter path melebarkan kotak pilihan tanpa memberitahu apa pun.
      label: /[\\/]/.test(m.name) ? m.base : m.name,
      // Spesifikasi jadi keterangan kecil, bukan sambungan nama: satu baris
      // "llama3.1:latest — 8.0B · Q4_K_M · 4.9 GB ✓ ready" membuat daftarnya
      // sesak dan justru tidak terbaca.
      //
      // Sistem asalnya ikut DI TIAP BARIS, bukan cuma di label: model-model ini
      // datang dari satu Ollama tertentu, dan pada mesin yang punya Windows DAN
      // WSL, "dari mana daftar ini" adalah hal pertama yang perlu dipastikan.
      note: [m.params, sizeGB(m.bytes), m.ready ? labels.ready : labels.notCapable, os]
        .filter(Boolean).join(" · "),
    })),
    ...(canPull
      ? SUGGESTED.filter((x) => !installed.some((m) => sameModel(m.name, x)))
          .map((x) => ({ value: x, label: x, note: labels.needsDownload }))
      : []),
  ];
}
