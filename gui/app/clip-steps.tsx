"use client";

import type { LucideIcon } from "lucide-react";

// Potongan layar "Atur klip" (DESIGN.md §4, §8). Semua kendali elemen asli:
// <button> dengan aria-pressed untuk pilihan, bukan div ber-onClick — Tab dan
// pembaca layar harus bisa memakainya tanpa kerja tambahan.

/** Kartu berikon: satu keputusan per kartu (rancangan 14 Oktober 2026). */
export function StepCard({
  icon: Icon, title, hint, disabled, children,
}: {
  icon: LucideIcon; title: string; hint?: string; disabled?: boolean; children: React.ReactNode;
}) {
  return (
    <section className={"step-card" + (disabled ? " off" : "")} aria-disabled={disabled || undefined}>
      <div className="step-head">
        <span className="step-ico" aria-hidden="true"><Icon className="ico" /></span>
        <div>
          <h2>{title}</h2>
          {hint && <p className="step-hint">{hint}</p>}
        </div>
      </div>
      {/* fieldset disabled mematikan SEMUA kendali di dalamnya sekaligus —
          kartu 2–4 nonaktif selama belum ada video (DESIGN.md §9). */}
      <fieldset className="step-body" disabled={disabled}>{children}</fieldset>
    </section>
  );
}

/** Kartu pilihan: nama + satu kalimat penjelas. Terpilih = garis aksen + cincin. */
export function ChoiceCards<T extends string>({
  value, onChange, options, label,
}: {
  value: T; onChange: (v: T) => void; label: string;
  options: { value: T; name: string; desc: string; icon?: LucideIcon }[];
}) {
  return (
    <div className="choice-grid" role="group" aria-label={label}>
      {options.map((o) => (
        <button key={o.value} type="button" className={"choice" + (o.value === value ? " on" : "")}
          aria-pressed={o.value === value} onClick={() => onChange(o.value)}>
          {o.icon && <span className="choice-ico" aria-hidden="true"><o.icon className="ico" /></span>}
          <span className="choice-name">{o.name}</span>
          <span className="choice-desc">{o.desc}</span>
        </button>
      ))}
    </div>
  );
}

/** Deret tombol dalam satu kotak bergaris; yang aktif berlatar gelap. */
export function Segmented<T extends string | number>({
  value, onChange, options, label,
}: {
  value: T; onChange: (v: T) => void; label: string;
  options: { value: T; name: string }[];
}) {
  return (
    <div className="segmented" role="group" aria-label={label}>
      {options.map((o) => (
        <button key={String(o.value)} type="button" className={o.value === value ? "on" : ""}
          aria-pressed={o.value === value} onClick={() => onChange(o.value)}>{o.name}</button>
      ))}
    </div>
  );
}

/** Kotak warna 44×44; namanya ada di aria-label & title, bukan hanya di warnanya. */
export function Swatches({
  value, onChange, options, label,
}: {
  value: string; onChange: (v: string) => void; label: string;
  options: { value: string; name: string }[];
}) {
  return (
    <div className="swatches" role="group" aria-label={label}>
      {options.map((o) => (
        <button key={o.value} type="button" className={"swatch-btn" + (o.value.toLowerCase() === value.toLowerCase() ? " on" : "")}
          style={{ background: o.value }} aria-pressed={o.value.toLowerCase() === value.toLowerCase()}
          aria-label={o.name} title={o.name} onClick={() => onChange(o.value)} />
      ))}
    </div>
  );
}

/** Saklar on/off: kotak centang asli yang digambar ulang, jadi papan ketik dan
 *  pembaca layar tetap melihat <input type="checkbox" role="switch">. */
export function Toggle({
  checked, onChange, label, desc, icon,
}: {
  checked: boolean; onChange: (v: boolean) => void; label: string; desc?: string; icon?: React.ReactNode;
}) {
  return (
    <label className="toggle-row">
      {icon}
      <span className="toggle-text">
        <span className="toggle-label">{label}</span>
        {desc && <span className="toggle-desc">{desc}</span>}
      </span>
      <input type="checkbox" role="switch" checked={checked} onChange={(e) => onChange(e.target.checked)} />
      <span className="toggle-track" aria-hidden="true" />
    </label>
  );
}

/** "42 menit 18 detik", bukan "2538" (DESIGN.md §10). */
export function humanDuration(sec: number, t: (k: "durMinSec" | "durHourMin", v: Record<string, number>) => string): string {
  const s = Math.max(0, Math.round(sec));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  return h > 0 ? t("durHourMin", { h, m }) : t("durMinSec", { m, s: s % 60 });
}
