"use client";

// Menyalin tautan artikel dari daftar berita — dipakai tab kartu berita DAN tab
// pembuat berita.
//
// Diangkat jadi satu hook karena isinya dua hal yang gampang salah kalau
// disalin: hasil pencarian membawa pengalih news.google.com (bukan alamat
// medianya, jadi harus diresolusi lebih dulu), dan navigator.clipboard TIDAK
// ADA saat GUI dibuka lewat alamat IP mesin — bukan hanya gagal, tapi undefined.
// Dua jebakan itu cukup untuk membuat salinan kedua diam-diam berbeda perilaku.

import { useCallback, useState } from "react";
import { eng } from "./engine";
import { useI18n } from "./i18n";

export function useCopyLink(opts: {
  /** Dipanggil bila alamat pengalih berhasil diresolusi jadi alamat asli. */
  onResolved?: (from: string, to: string) => void;
  onError?: (message: string) => void;
}) {
  const { t } = useI18n();
  const [copied, setCopied] = useState("");
  const [busy, setBusy] = useState("");
  const { onResolved, onError } = opts;

  const copyLink = useCallback(async (url: string) => {
    if (!url || busy) return;
    try {
      // Pengalih Google diresolusi dulu: yang berguna untuk dicek silang adalah
      // alamat medianya. Perlu satu peluncuran browser (~2-3 detik), tapi
      // hasilnya di-cache engine.
      if (url.includes("news.google.com/")) {
        setBusy(url);
        const res = await fetch(eng(`/api/news/resolve`), {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ url }),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || t("errOpenLink"));
        onResolved?.(url, data.url);
        url = data.url;
      }
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(url);
      } else {
        const area = document.createElement("textarea");
        area.value = url;
        area.style.position = "fixed";
        area.style.opacity = "0";
        document.body.appendChild(area);
        area.select();
        document.execCommand("copy");
        document.body.removeChild(area);
      }
      const done = url;
      setCopied(done);
      setTimeout(() => setCopied((c) => (c === done ? "" : c)), 1600);
    } catch (e: any) {
      onError?.(e?.message || t("errCopy"));
    } finally {
      setBusy("");
    }
  }, [busy, t, onResolved, onError]);

  return { copyLink, copied, busy };
}
