"use client";

// Potongan layar "Atur klip" (DESIGN.md §4, §8). Semua kendali elemen asli:
// <button> dengan aria-pressed untuk pilihan, bukan div ber-onClick — Tab dan
// pembaca layar harus bisa memakainya tanpa kerja tambahan.

/** Kartu bernomor: satu keputusan per kartu. */
export function StepCard({
  n, title, hint, disabled, children,
}: {
  n: number; title: string; hint?: string; disabled?: boolean; children: React.ReactNode;
}) {
  return (
    <section className={"step-card" + (disabled ? " off" : "")} aria-disabled={disabled || undefined}>
      <div className="step-head">
        <span className="step-n" aria-hidden="true">{n}</span>
        <h2>{title}</h2>
      </div>
      {hint && <p className="step-hint">{hint}</p>}
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
  options: { value: T; name: string; desc: string }[];
}) {
  return (
    <div className="choice-grid" role="group" aria-label={label}>
      {options.map((o) => (
        <button key={o.value} type="button" className={"choice" + (o.value === value ? " on" : "")}
          aria-pressed={o.value === value} onClick={() => onChange(o.value)}>
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

/** "42 menit 18 detik", bukan "2538" (DESIGN.md §10). */
export function humanDuration(sec: number, t: (k: "durMinSec" | "durHourMin", v: Record<string, number>) => string): string {
  const s = Math.max(0, Math.round(sec));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  return h > 0 ? t("durHourMin", { h, m }) : t("durMinSec", { m, s: s % 60 });
}
