"use client";

// Daftar berkas yang diproses sekaligus — SATU untuk Captions dan Watermark.
// Dulu dua salinan kata per kata (add, addFolder, dropFiles, tombol Add / Whole
// folder / tempel path); bedanya cuma saringan berkas dan isi tiap baris.

import { useCallback, useRef, useState } from "react";
import { Folder, Plus } from "lucide-react";
import { eng, isWeb, upload, post } from "./engine";
import { useI18n } from "./i18n";

type Entry = { video?: boolean; path: string };

// baseName = nama berkas dari path Windows atau POSIX.
export const baseName = (p: string) => p.split(/[\\/]/).pop() || p;

// useFiles: state daftar + cara menambahnya. keep = saringan isi folder
// (/api/browse sudah menandai mana yang video).
export function useFiles(keep: (e: Entry) => boolean, setError: (msg: string) => void) {
  const { t } = useI18n();
  const [files, setFiles] = useState<string[]>([]);
  const [upNote, setUpNote] = useState("");

  const add = useCallback((paths: string[]) => {
    setFiles((cur) => {
      const out = [...cur];
      for (const p of paths) {
        // Kutip pembungkus dari "Copy as path" di Explorer dibuang: daftar di
        // layar harus menampilkan nama berkas, bukan `"C:\…mp4"` lengkap.
        const path = p.trim().replace(/^["']|["']$/g, "").trim();
        if (path && !out.includes(path)) out.push(path);
      }
      return out;
    });
  }, []);

  // Seluruh isi folder sekaligus — jalan bulk yang sebenarnya.
  const addFolder = useCallback(async (dir: string) => {
    try {
      const r = await fetch(eng(`/api/browse?dir=${encodeURIComponent(dir)}`));
      const data = await r.json();
      if (!r.ok) throw new Error(data.error || "browse failed");
      const found = (data.entries || []).filter(keep).map((e: Entry) => e.path);
      if (!found.length) { setError(t("capFolderEmpty")); return; }
      add(found);
    } catch (e) {
      setError(String(e));
    }
  }, [add, keep, setError, t]);

  // Berkas yang dilepas TIDAK diunggah di mode lokal: engine ditanya di mana
  // berkasnya (notes/24). Mode web: engine di server, jadi diunggah.
  const dropFiles = useCallback(async (list: FileList | File[]) => {
    const web = await isWeb();
    for (const f of Array.from(list)) {
      if (web) {
        try {
          add([await upload(f, (x) => setUpNote(`${f.name}: ${t("uploadingPct", { pct: Math.round(x * 100) })}`))]);
        } catch (e) { setError(`${f.name}: ${String(e)}`); }
        setUpNote("");
        continue;
      }
      try {
        const data = await post("/api/locate", { name: f.name, size: f.size });
        if (data.path) { add([data.path]); continue; }
      } catch { /* engine mati: dilaporkan di bawah */ }
      setError(`${f.name}: ${t("capFailed")}`);
    }
  }, [add, setError, t]);

  return { files, setFiles, add, addFolder, dropFiles, upNote, setUpNote };
}

export type Files = ReturnType<typeof useFiles>;

// FileCard = kartu sumber baku (setara kartu Source di halaman klip): judul +
// jumlah, area lepas berkas, baris-baris milik halaman (children), lalu cara
// menambah. onPick membuka pemilih berkas/folder milik halaman.
export default function FileCard({ title, fl, web, accept, addLabel, note, onPick, children }: {
  title: string; fl: Files; web: boolean | null; accept: string; addLabel: string; note?: string;
  onPick: (mode: "file" | "folder") => void; children: React.ReactNode;
}) {
  const { t } = useI18n();
  const [over, setOver] = useState(false);
  const [paste, setPaste] = useState("");
  const input = useRef<HTMLInputElement>(null);
  const addPasted = () => { fl.add(paste.split("\n")); setPaste(""); };
  const n = fl.files.length;
  return (
    <div className={"card cap-drop" + (over ? " over" : "")}
      onDragOver={(e) => { e.preventDefault(); setOver(true); }}
      onDragLeave={() => setOver(false)}
      onDrop={(e) => { e.preventDefault(); setOver(false); if (e.dataTransfer.files?.length) fl.dropFiles(e.dataTransfer.files); }}>
      <div className="card-head">
        <h2>{title}</h2>
        <span className="meta">{n === 1 ? t("capFilesOne") : t("capFiles", { n })}</span>
      </div>
      {children}
      {web ? (
        <>
          <input ref={input} type="file" accept={accept} multiple hidden
            onChange={(e) => { const fs = Array.from(e.target.files ?? []); e.target.value = ""; if (fs.length) fl.dropFiles(fs); }} />
          <div className="cap-add-row">
            <button type="button" className="ghost cap-add" disabled={!!fl.upNote} onClick={() => input.current?.click()}>
              <Plus className="ico" aria-hidden="true" /> {addLabel}
            </button>
          </div>
          {fl.upNote && <p className="meta">{fl.upNote}</p>}
        </>
      ) : (
        <>
          <div className="cap-add-row">
            <button type="button" className="ghost cap-add" onClick={() => onPick("file")}><Plus className="ico" aria-hidden="true" /> {addLabel}</button>
            <button type="button" className="ghost" onClick={() => onPick("folder")}><Folder className="ico" aria-hidden="true" /> {t("capPickFolder")}</button>
          </div>
          <div className="path-row wr-gap">
            <input value={paste} onChange={(e) => setPaste(e.target.value)} aria-label={t("capPastePlaceholder")}
              onKeyDown={(e) => { if (e.key === "Enter") addPasted(); }} placeholder={t("capPastePlaceholder")} />
            <button type="button" className="ghost" disabled={!paste.trim()} onClick={addPasted}>{t("capAdd")}</button>
          </div>
        </>
      )}
      {note && <p className="meta cap-hint">{note}</p>}
    </div>
  );
}
