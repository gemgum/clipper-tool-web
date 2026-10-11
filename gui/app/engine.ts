"use client";

// Alamat engine + kunci sesi.
//
// Dua hal yang harus diketahui setiap permintaan ke engine, dan keduanya baru
// diketahui SAAT HALAMAN DIBUKA — bukan saat GUI dibangun:
//
//   alamat  engine memakai port acak begitu ia terpasang, jadi memaku
//           "127.0.0.1:8787" ke dalam berkas hasil build membuat aplikasi
//           bertanya ke port yang kosong;
//   kunci   dibuat baru setiap engine dijalankan, dan sampai ke halaman lewat
//           "?token=…" pada alamat yang dibuka jendela aplikasi.

import { useEffect, useState } from "react";

const DEV_ENGINE = "http://127.0.0.1:8787";

// Port server pengembangan Next. Halaman yang disajikan dari sini BUKAN berasal
// dari engine, jadi hanya dalam keadaan itulah alamat engine perlu ditebak.
const DEV_PORT = "3000";

let base: string | null = null;
let ready = false;

/** engineBase menentukan ke mana permintaan API dikirim. */
function engineBase(): string {
  if (base !== null) return base;

  const fromEnv = process.env.NEXT_PUBLIC_ENGINE_URL;
  if (fromEnv) {
    base = fromEnv;
    return base;
  }
  if (typeof window !== "undefined") {
    const { protocol, port, origin } = window.location;
    // Halaman ini disajikan engine sendiri (aplikasi jadi): API-nya ada di
    // alamat yang sama persis, berapa pun portnya. Alamat relatif akan lebih
    // sederhana, tapi origin penuh dipakai supaya nilainya bisa dicetak apa
    // adanya saat mencari masalah.
    if (protocol.startsWith("http") && port !== DEV_PORT) {
      base = origin;
      return base;
    }
  }
  // Mode pengembangan: GUI di :3000, engine di tempat biasanya.
  base = DEV_ENGINE;
  return base;
}

function init() {
  if (ready || typeof window === "undefined") return;
  ready = true;
  try {
    // Kunci dibersihkan dari bilah alamat setelah halaman termuat. Ia sudah
    // ditukar jadi cookie oleh engine SEBELUM skrip ini jalan (lihat token.go),
    // jadi yang tersisa di sini hanya membereskan jejaknya: tidak perlu
    // terlihat, dan tidak boleh ikut tersalin saat pengguna menyalin URL.
    const url = new URL(window.location.href);
    if (url.searchParams.has("token")) {
      url.searchParams.delete("token");
      window.history.replaceState({}, "", url.toString());
    }
  } catch {
    // Bilah alamat yang tidak bisa disentuh bukan alasan menghentikan apa pun.
  }
}

/**
 * eng membentuk URL lengkap ke engine.
 *
 * Kunci sesi TIDAK ikut di sini. Ia tinggal di cookie HttpOnly yang dipasang
 * engine saat halaman pertama dibuka, dan cookie ikut terkirim sendiri oleh
 * semua bentuk permintaan — termasuk yang dibuat browser, bukan oleh kode kita:
 * EventSource, <video src>, dan tautan unduh. Itu justru yang dulu memaksa
 * kuncinya ditaruh di query, tempat paling mudah bocor (Referer, riwayat,
 * tangkapan layar).
 *
 * Tetap dipakai di SEMUA pemanggilan engine: alamatnya sendiri baru diketahui
 * saat halaman dibuka, sebab port engine acak begitu aplikasinya terpasang.
 */
export function eng(path: string): string {
  init();
  return engineBase() + path;
}

/** ENGINE = alamat engine yang sedang dipakai. Untuk ditampilkan, bukan disusun. */
export function engineURL(): string {
  return engineBase();
}

// Mode web (branch webview, notes/42): engine di server, jadi berkas DIUNGGAH
// dan hasil DIUNDUH — pemilih folder & "buka folder" tidak bermakna di sana.
// Engine yang menentukan, lewat /api/health; satu build GUI untuk kedua mode.
let webPromise: Promise<boolean> | null = null;

/** isWeb melaporkan apakah engine berjalan dalam mode web. */
export function isWeb(): Promise<boolean> {
  webPromise ??= fetch(eng("/api/health"))
    .then((r) => r.json())
    .then((d) => d.web === true)
    .catch(() => false);
  return webPromise;
}

/** useWeb = isWeb sebagai hook; null selama jawabannya belum datang. */
export function useWeb(): boolean | null {
  const [web, setWeb] = useState<boolean | null>(null);
  useEffect(() => { isWeb().then(setWeb); }, []);
  return web;
}

/** upload mengirim satu berkas ke engine dan mengembalikan path-nya di server. */
export function upload(file: File, onProgress?: (fraction: number) => void): Promise<string> {
  return new Promise((resolve, reject) => {
    const form = new FormData();
    form.append("file", file);
    const xhr = new XMLHttpRequest();
    xhr.open("POST", eng("/api/upload"));
    xhr.upload.onprogress = (e) => { if (e.lengthComputable) onProgress?.(e.loaded / e.total); };
    xhr.onload = () => {
      try {
        const data = JSON.parse(xhr.responseText);
        if (data.path) resolve(data.path);
        else reject(new Error(data.error || "upload failed"));
      } catch { reject(new Error("upload failed")); }
    };
    xhr.onerror = () => reject(new Error("upload failed: network error"));
    xhr.send(form);
  });
}
