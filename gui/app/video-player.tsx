"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { X } from "lucide-react";
import { useI18n } from "./i18n";

// Pemutar video dalam jendela: <dialog> asli (Esc menutup, fokus terkunci di
// dalamnya) + <video controls> asli. Engine menyajikan berkasnya dengan Range,
// jadi video berjam-jam pun bisa langsung digeser tanpa diunduh utuh.

type Item = { src: string; title: string };

/** usePlayer: `open(src, title)` memutar; `el` dirender sekali di komponennya. */
export function usePlayer() {
  const [item, setItem] = useState<Item | null>(null);
  const open = useCallback((src: string, title: string) => setItem({ src, title }), []);
  const el = item ? <VideoPlayer {...item} onClose={() => setItem(null)} /> : null;
  return { open, el };
}

function VideoPlayer({ src, title, onClose }: Item & { onClose: () => void }) {
  const { t } = useI18n();
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const d = ref.current;
    if (d && !d.open) d.showModal();
  }, []);
  return (
    // Klik di luar video (latar dialog) = tutup.
    <dialog ref={ref} className="player" aria-label={title} onClose={onClose}
      onClick={(e) => { if (e.target === ref.current) ref.current?.close(); }}>
      <div className="player-head">
        <p className="player-title" title={title}>{title}</p>
        <button type="button" className="ghost icon-only" aria-label={t("closePlayer")} onClick={() => ref.current?.close()}>
          <X className="ico" aria-hidden="true" />
        </button>
      </div>
      <video src={src} controls autoPlay playsInline />
    </dialog>
  );
}
