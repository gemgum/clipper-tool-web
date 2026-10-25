"use client";

// Kartu setelan bertab — bentuk baku dari halaman klip (rancangan "Redesain
// Video Clips", 14 Oktober 2026): tab sama lebar di satu bilah, panel tab
// terpilih di bawahnya, dibagi bagian bernama berisi baris label | kendali.
// Halaman lain memakai komponen ini, bukan menyalin markup-nya.

import type { LucideIcon } from "lucide-react";

export type Tab<K extends string> = { key: K; icon: LucideIcon; label: string; alert?: string };

export default function TabCard<K extends string>({ tabs, open, setOpen, label, disabled, children }: {
  tabs: Tab<K>[]; open: K; setOpen: (k: K) => void; label: string; disabled?: boolean; children: React.ReactNode;
}) {
  return (
    <section className={"step-card mc" + (disabled ? " off" : "")} aria-disabled={disabled || undefined}>
      <fieldset className="step-body" disabled={disabled}>
        <div className="mc-tabs" role="tablist" aria-label={label} style={{ gridTemplateColumns: `repeat(${tabs.length}, minmax(0, 1fr))` }}>
          {tabs.map(({ key, icon: Icon, label, alert }) => (
            <button key={key} type="button" role="tab" className={"mc-tab" + (open === key ? " on" : "")}
              aria-selected={open === key} onClick={() => setOpen(key)}>
              <span className="mc-tab-ic" aria-hidden="true"><Icon className="ico" /></span>
              <span className="mc-tab-lb">{label}</span>
              {/* Peringatan tab lain tetap terlihat walau tabnya tidak dipilih. */}
              {alert && open !== key && <span className="mc-alert" aria-label={alert} />}
            </button>
          ))}
        </div>
        <div className="mc-panel" role="tabpanel">{children}</div>
      </fieldset>
    </section>
  );
}

// Group = judul kecil + kotak baris. title kosong = tanpa judul.
export function Group({ title, children }: { title?: string; children: React.ReactNode }) {
  return (
    <section className="mc-group">
      {title && <h3 className="mc-group-t">{title}</h3>}
      <div className="mc-rows">{children}</div>
    </section>
  );
}

// Row = label di kiri, kendali di kanan, catatan opsional di bawah kendali.
// off = baris yang belum berlaku: TETAP tampil tapi dimatikan. Baris yang
// muncul-hilang mengubah tinggi panel dan menggeser semua di bawahnya.
export function Row({ label, title, note, off, children }: {
  label: string; title?: string; note?: React.ReactNode; off?: boolean; children: React.ReactNode;
}) {
  return (
    <div className={"mc-r" + (off ? " off" : "")}>
      <span className="mc-r-l" title={title}>{label}</span>
      <fieldset className="mc-r-c" disabled={off}>{children}{note && <span className="mc-note">{note}</span>}</fieldset>
    </div>
  );
}

// Split = dua bagian SEJAJAR di satu grup (keputusan pemilik 24 Oktober 2026):
// kiri = baris/daftar utama, kanan = panel rincian (label di atas kendali).
// Panel kanan SELALU ada; sideOff mematikannya, tidak menyembunyikannya —
// memilih atau menambah sesuatu tidak pernah menggeser isi.
// mid = kolom tengah opsional: di jendela lebar berdiri sendiri di antara
// kiri & kanan (label di atas kendali), di jendela lebih sempit turun ke bawah
// kolom kiri dengan baris yang sama bentuknya.
export function Split({ side, sideOff, mid, children }: { side: React.ReactNode; sideOff?: boolean; mid?: React.ReactNode; children: React.ReactNode }) {
  return (
    <div className={"mc-split" + (mid ? " three" : "")}>
      <div className="mc-split-l">{children}</div>
      {mid && <div className="mc-split-m">{mid}</div>}
      <fieldset className="mc-split-r" disabled={sideOff}>{side}</fieldset>
    </div>
  );
}

// Pad = baris selebar kotak (daftar, tombol tambah, sakelar).
export function Pad({ children }: { children: React.ReactNode }) {
  return <div className="mc-r mc-r-pad">{children}</div>;
}
