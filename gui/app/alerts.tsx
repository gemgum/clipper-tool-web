"use client";

import { useState } from "react";
import { OctagonX, TriangleAlert, X } from "lucide-react";
import { useI18n } from "./i18n";

// Notifikasi galat & peringatan — MELAYANG, bukan disisipkan ke halaman.
//
// Sebelumnya tiap peringatan dirender sebagai baris `.screen-head`, jadi
// munculnya mendorong seluruh isi halaman turun dan hilangnya menariknya
// kembali: ukuran pratinjau berubah, tombol berpindah di bawah kursor, dan
// panel yang sedang dibaca melompat. `position: fixed` membuat lapisan ini
// tidak punya tempat di aliran tata letak sama sekali — nol piksel pergeseran,
// apa pun yang muncul.
//
// Yang membedakannya dari teks biasa: WARNA + LAMBANG. Peringatan yang tampil
// dengan warna teks biasa terbaca sebagai keterangan, dan itu persis kenapa
// pengguna tidak pernah sadar ada yang salah.
export type Alert = { kind: "error" | "warn"; text: React.ReactNode; key?: string };

// Nilai kosong diterima apa adanya ("" dari state galat, false dari `&&`),
// jadi pemanggilnya tidak perlu menulis penjaga tiap kali.
export default function Alerts({ items }: { items: (Alert | "" | 0 | null | false | undefined)[] }) {
  const { t } = useI18n();
  // Ditutup manual, disimpan per kunci. Tanpa hilang sendiri: sebagian
  // peringatan (komponen belum terpasang) masih benar sepuluh menit kemudian.
  const [hidden, setHidden] = useState<Set<string>>(new Set());

  const live = items.filter((a): a is Alert => !!a)
    .map((a) => ({ ...a, key: a.key || String(a.text) }))
    .filter((a) => !hidden.has(a.key));
  if (live.length === 0) return null;

  return (
    <div className="alerts" role="status" aria-live="polite">
      {live.map((a) => (
        <div className={`alert ${a.kind}`} key={a.key}>
          {a.kind === "error"
            ? <OctagonX className="ico" aria-hidden="true" />
            : <TriangleAlert className="ico" aria-hidden="true" />}
          <div className="alert-text">{a.text}</div>
          <button className="alert-x" aria-label={t("close")} title={t("close")}
            onClick={() => setHidden((s) => new Set(s).add(a.key))}>
            <X className="ico" aria-hidden="true" />
          </button>
        </div>
      ))}
    </div>
  );
}
