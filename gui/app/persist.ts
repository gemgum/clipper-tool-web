"use client";

// Menyimpan isian pengguna supaya tidak hilang saat halaman termuat ulang.
//
// Di aplikasi desktop, halaman bisa termuat ulang tanpa diminta: WebView2
// menidurkan atau membuang proses penampil saat jendela ditinggalkan lama atau
// saat memori menipis, lalu memuatnya lagi dari awal. Yang hilang bukan
// pekerjaan engine — itu berjalan di latar — melainkan seluruh isian di layar:
// artikel yang sudah diambil, caption yang sudah disunting, setelan kartu.
//
// Mengetik ulang semuanya karena berpindah jendela adalah hukuman yang tidak
// masuk akal, jadi isian disimpan dan dipulihkan sendiri.

import { useEffect, useRef, useState } from "react";

const PREFIX = "clipper.form.";

/**
 * useRestore memulihkan satu kali saat halaman dibuka.
 *
 * Pemulihan dilakukan di dalam effect, bukan sebagai nilai awal useState:
 * halaman ini diekspor statis, jadi render pertama di browser harus sama persis
 * dengan HTML yang dikirim engine. Membaca localStorage lebih awal membuat
 * keduanya berbeda dan React membuang hasil render pertamanya.
 */
export function useRestore<T>(key: string, apply: (saved: T) => void) {
  const done = useRef(false);
  useEffect(() => {
    if (done.current) return;
    done.current = true;
    try {
      const raw = localStorage.getItem(PREFIX + key);
      if (raw) apply(JSON.parse(raw) as T);
    } catch {
      // Isian tersimpan yang rusak atau dari versi lama tidak boleh
      // menghentikan halaman — lebih baik mulai dari kosong.
    }
    // apply sengaja tidak jadi dependency: ia dibuat ulang tiap render, dan
    // pemulihan memang hanya boleh terjadi sekali.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);
}

/**
 * useKeep menyimpan snapshot setiap kali isinya berubah.
 *
 * Ditunda sebentar supaya mengetik satu kalimat tidak berarti menulis puluhan
 * kali ke penyimpanan.
 */
export function useKeep(key: string, value: unknown) {
  const [ready, setReady] = useState(false);
  useEffect(() => {
    // Satu putaran ditunggu supaya snapshot kosong tidak menimpa isian
    // tersimpan sebelum useRestore sempat memakainya.
    const id = setTimeout(() => setReady(true), 0);
    return () => clearTimeout(id);
  }, []);

  useEffect(() => {
    if (!ready) return;
    const id = setTimeout(() => {
      try {
        localStorage.setItem(PREFIX + key, JSON.stringify(value));
      } catch {
        // Penyimpanan penuh atau ditolak: kenyamanan boleh hilang, halaman
        // tidak boleh.
      }
    }, 400);
    return () => clearTimeout(id);
  }, [key, value, ready]);
}

/** forget membuang isian tersimpan — dipakai tombol "mulai dari awal". */
export function forget(key: string) {
  try {
    localStorage.removeItem(PREFIX + key);
  } catch {}
}
