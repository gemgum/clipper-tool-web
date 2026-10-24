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
import { usePathname } from "next/navigation";

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

/**
 * useSticky = useState yang diingat BROWSER (localStorage, bukan server): layar
 * & tab yang sedang terbuka tetap sama saat pengguna pindah ke halaman lain
 * lalu kembali — Next merender ulang halamannya dari awal setiap kali.
 */
export function useSticky<T>(key: string, initial: T) {
  const [value, setValue] = useState<T>(initial);
  useRestore<T>("view." + key, setValue);
  useKeep("view." + key, value);
  return [value, setValue] as const;
}

/** recall membaca nilai useSticky secara langsung (mis. di dalam fetch yang
 *  selesai sebelum useRestore sempat menjalankan setter-nya). */
export function recall<T>(key: string): T | undefined {
  try {
    const raw = localStorage.getItem(PREFIX + "view." + key);
    return raw ? (JSON.parse(raw) as T) : undefined;
  } catch { return undefined; }
}

// Kotak yang bergulir di halaman-halaman (CLAUDE.md "Tampilan"): halaman
// berlayar-gulir, kolom kiri, kolom hasil.
const SCROLLERS = [".screen.scroll", ".setup-main", ".io-in", ".io-out"];

/**
 * ScrollKeep mengingat posisi gulir tiap halaman di browser dan memulihkannya
 * saat halaman itu dibuka lagi. Dipasang SEKALI di rail (nav.tsx). Isi halaman
 * bisa datang belakangan (daftar dari engine), jadi pemulihan dicoba ulang
 * sebentar sampai kotaknya cukup tinggi.
 */
export function ScrollKeep() {
  const path = usePathname();
  useEffect(() => {
    const key = (sel: string) => `${PREFIX}scroll.${path}.${sel}`;
    let tries = 0;
    const restore = () => {
      let pending = false;
      for (const sel of SCROLLERS) {
        const el = document.querySelector<HTMLElement>(sel);
        let want = 0;
        try { want = Number(localStorage.getItem(key(sel))) || 0; } catch {}
        if (!el || !want) continue;
        el.scrollTop = want;
        if (Math.abs(el.scrollTop - want) > 2) pending = true;
      }
      if (pending && ++tries < 20) id = setTimeout(restore, 100);
    };
    let id = setTimeout(restore, 50);
    const save = (e: Event) => {
      const el = e.target as HTMLElement;
      const sel = SCROLLERS.find((s) => el.matches?.(s));
      if (!sel) return;
      try { localStorage.setItem(key(sel), String(Math.round(el.scrollTop))); } catch {}
    };
    document.addEventListener("scroll", save, true);
    return () => { clearTimeout(id); document.removeEventListener("scroll", save, true); };
  }, [path]);
  return null;
}
