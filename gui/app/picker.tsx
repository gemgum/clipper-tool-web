"use client";

// Ikon: lucide-react (ISC) — alasannya di gui/app/page.tsx.
import { Folder, Film, File } from "lucide-react";

// Pemilih berkas milik sendiri, dilayani engine lewat /api/browse.
//
// Alasannya bukan estetika: pemilih bawaan browser hanya memberi ISI berkas,
// tidak pernah lokasinya, jadi satu-satunya cara memakainya adalah mengunggah
// salinan — 3,84 GB digandakan sebelum pekerjaan dimulai. Engine jalan di mesin
// yang sama dengan videonya, jadi ia bisa membacanya di tempat; yang perlu
// dikirim hanyalah path-nya.

import { useCallback, useEffect, useState } from "react";
import { useI18n } from "./i18n";
import { eng, upload, useWeb } from "./engine";

type Entry = {
  name: string;
  path: string;
  dir: boolean;
  video: boolean;
  size: number;
};

type Place = { name: string; path: string };

type Listing = {
  dir: string;
  parent: string;
  entries: Entry[];
  places: Place[];
  truncated: boolean;
};

function humanSize(bytes: number): string {
  if (bytes >= 1e9) return `${(bytes / 1e9).toFixed(1)} GB`;
  if (bytes >= 1e6) return `${Math.round(bytes / 1e6)} MB`;
  if (bytes >= 1e3) return `${Math.round(bytes / 1e3)} KB`;
  return `${bytes} B`;
}

type PickerProps = {
  mode: "file" | "folder";
  start?: string;
  onPick: (path: string) => void;
  onClose: () => void;
  /** Judul & keterangan khusus; tanpa ini dipakai kalimat untuk memilih video. */
  title?: string;
  hint?: string;
};

// Satu titik pilih untuk semua halaman: di mode web (notes/42) folder server
// tidak boleh dijelajah, jadi berkas DIUNGGAH dan path hasil unggahan itulah
// yang diserahkan ke onPick — pemanggilnya tidak perlu tahu bedanya.
export default function Picker(props: PickerProps) {
  const web = useWeb();
  if (web === null) return null;
  return web ? <UploadPicker {...props} /> : <BrowsePicker {...props} />;
}

function UploadPicker({ mode, onPick, onClose, title }: PickerProps) {
  const { t } = useI18n();
  const [pct, setPct] = useState<number | null>(null);
  const [error, setError] = useState("");

  // Esc menutup, sama seperti pemilih berkas di bawah.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") onClose(); };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const send = async (f: File) => {
    setError(""); setPct(0);
    try { onPick(await upload(f, setPct)); }
    catch (e: any) { setError(e.message); setPct(null); }
  };

  return (
    <div className="modal-back" onClick={onClose}>
      <div className="modal" onClick={(ev) => ev.stopPropagation()}>
        <div className="modal-head">
          <strong>{mode === "folder" ? t("pickerFolderTitle") : title || t("webUploadTitle")}</strong>
          <button className="ghost" onClick={onClose} aria-label={t("close")} title={t("close")}>✕</button>
        </div>
        <div className="picker-list">
          {mode === "folder" ? (
            <div className="meta">{t("webNoFolder")}</div>
          ) : (
            <>
              <input type="file" disabled={pct !== null}
                onChange={(e) => { const f = e.target.files?.[0]; if (f) send(f); }} />
              {pct !== null && <div className="meta">{t("uploadingPct", { pct: Math.round(pct * 100) })}</div>}
              {error && <div className="err">{error}</div>}
            </>
          )}
        </div>
        <div className="modal-foot">
          <span className="meta">{mode === "folder" ? "" : t("webUploadHint")}</span>
          <span style={{ flex: 1 }} />
          <button className="ghost" onClick={onClose}>{t("pickerCancel")}</button>
        </div>
      </div>
    </div>
  );
}

function BrowsePicker({ mode, start, onPick, onClose, title, hint }: PickerProps) {
  const { t } = useI18n();
  const [dir, setDir] = useState(start || "");
  const [listing, setListing] = useState<Listing | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(async (target: string) => {
    setBusy(true);
    setError("");
    try {
      const res = await fetch(eng(`/api/browse?dir=${encodeURIComponent(target)}`));
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "browse failed");
      setListing(data);
      setDir(data.dir);
    } catch (e: any) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }, []);

  useEffect(() => {
    load(start || "");
  }, [load, start]);

  // Esc menutup — pemilih ini menutupi seluruh layar, jadi harus ada jalan
  // keluar yang tidak menuntut membidik tombol.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  // Di mode folder, berkas tetap ditampilkan tapi diredupkan: melihat isinya
  // itulah cara pengguna memastikan ia berada di folder yang benar.
  const pickable = (e: Entry) => (mode === "folder" ? e.dir : true);

  return (
    <div className="modal-back" onClick={onClose}>
      <div className="modal" onClick={(ev) => ev.stopPropagation()}>
        <div className="modal-head">
          <strong>{title || (mode === "folder" ? t("pickerFolderTitle") : t("pickerFileTitle"))}</strong>
          <button className="ghost" onClick={onClose} aria-label={t("close")} title={t("close")}>
            ✕
          </button>
        </div>

        <div className="picker-path">
          <button
            className="ghost"
            disabled={!listing?.parent}
            onClick={() => listing?.parent && load(listing.parent)}
          >
            ↑ {t("pickerUp")}
          </button>
          <input
            value={dir}
            onChange={(e) => setDir(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && load(dir)}
            spellCheck={false}
          />
          <button className="ghost" onClick={() => load(dir)}>
            {t("pickerGo")}
          </button>
        </div>

        <div className="picker-body">
          <div className="picker-places">
            {listing?.places.map((p) => (
              <button key={p.path} className="ghost picker-place" onClick={() => load(p.path)}>
                {p.name}
              </button>
            ))}
          </div>

          <div className="picker-list">
            {error && <div className="err">{error}</div>}
            {busy && !listing && <div className="meta">{t("loading")}</div>}
            {listing?.entries.length === 0 && <div className="meta">{t("pickerEmpty")}</div>}
            {/* Klik-ganda tunduk pada penyaring yang SAMA dengan klik biasa.
                Dulu tidak, jadi klik-ganda pada berkas apa pun — termasuk
                whisper-cli.exe — menetapkannya sebagai "video sumber", dan
                pemilih berikutnya lalu dibuka dari path yang mustahil. */}
            {listing?.entries.map((e) => (
              <div
                key={e.path}
                className={`picker-row ${pickable(e) ? "" : "dim"} ${e.video ? "video" : ""}`}
                onDoubleClick={() => (e.dir ? load(e.path) : pickable(e) && onPick(e.path))}
                onClick={() => (e.dir ? load(e.path) : pickable(e) && onPick(e.path))}
                title={e.path}
              >
                <span className="picker-icon" aria-hidden="true">
                  {e.dir ? <Folder className="ico" /> : e.video ? <Film className="ico" /> : <File className="ico" />}
                </span>
                <span className="picker-name">{e.name}</span>
                {!e.dir && <span className="meta">{humanSize(e.size)}</span>}
              </div>
            ))}
            {listing?.truncated && <div className="meta">{t("pickerTruncated")}</div>}
          </div>
        </div>

        <div className="modal-foot">
          <span className="meta">{hint || (mode === "folder" ? t("pickerFolderHint") : t("pickerFileHint"))}</span>
          <span style={{ flex: 1 }} />
          {mode === "folder" && (
            <button onClick={() => onPick(listing?.dir || dir)}>{t("pickerUseFolder")}</button>
          )}
          <button className="ghost" onClick={onClose}>
            {t("pickerCancel")}
          </button>
        </div>
      </div>
    </div>
  );
}
