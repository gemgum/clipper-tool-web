"use client";

import { useI18n } from "./i18n";
import { eng } from "./engine";

// Cuplikan frame sebuah video, dibuat engine sekali lalu disimpan (/api/thumb).
// Rumus detiknya HARUS sama dengan thumbAt di engine/internal/api/source.go.
export const THUMB_COUNT = 8;
export const thumbAt = (duration: number, i: number) => (duration * (i + 0.5)) / THUMB_COUNT;
/** Frame yang mewakili detik t: bagian ke-berapa dari delapan. */
export const thumbIndex = (duration: number, t: number) =>
  duration > 0 ? Math.min(THUMB_COUNT - 1, Math.max(0, Math.floor((t / duration) * THUMB_COUNT))) : -1;

/** Deret cuplikan; klik satu = pratinjau melompat ke detik itu. */
export default function Filmstrip({ path, duration, time, onPick, className = "" }: {
  path: string; duration: number; time: number; onPick: (t: number) => void; className?: string;
}) {
  const { t } = useI18n();
  const active = thumbIndex(duration, time);
  return (
    <div className={"filmstrip " + className} role="group" aria-label={t("framesLabel")}>
      {Array.from({ length: THUMB_COUNT }, (_, i) => {
        const at = thumbAt(duration, i);
        return (
          <button key={i} type="button" className={"film-frame" + (i === active ? " on" : "")}
            disabled={!duration} aria-pressed={i === active} onClick={() => onPick(at)}
            aria-label={t("frameAt", { t: at.toFixed(1) })} title={t("frameAt", { t: at.toFixed(1) })}>
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img src={eng(`/api/thumb?path=${encodeURIComponent(path)}&i=${i}`)} alt="" loading="lazy" draggable={false} />
          </button>
        );
      })}
    </div>
  );
}
