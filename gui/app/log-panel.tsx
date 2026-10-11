"use client";

import { useEffect, useRef, useState } from "react";
import { ChevronDown, Copy } from "lucide-react";
import { useI18n } from "./i18n";

// Kotak log proses, kini LACI yang bisa dilipat (DESIGN.md §5.7, 9 Oktober 2026).
//
// Tertutup ia setinggi satu baris dan menampilkan baris TERAKHIR — keadaan
// job tetap terbaca tanpa kotak hitam besar yang kosong. Ia membuka sendiri
// begitu baris pertama masuk, sebab saat itulah orang ingin melihatnya;
// sesudahnya pengguna yang memutuskan.
//
// Inilah satu-satunya kotak di kolom ini yang memang BOLEH bergulir sendiri,
// dan itu bukan pelanggaran melainkan cara aplikasi desktop bekerja: yang haram
// adalah harus menggulir HALAMAN untuk menemukan tombol (notes/29).
//
// Barisnya tetap string, bukan komponen — itu sebabnya lambangnya memakai
// ✓ ↓ ↑ ↻ yang ada di font mana pun, bukan emoji (notes/29). Warnanya dibaca
// dari lambang dan kata di baris itu, bukan dari jenis pesan yang dikirim.
export default function LogPanel({ logs }: { logs: string[] }) {
  const { t } = useI18n();
  const boxRef = useRef<HTMLDivElement | null>(null);
  const [open, setOpen] = useState(logs.length > 0);
  const [copied, setCopied] = useState(false);
  const had = useRef(logs.length > 0);

  useEffect(() => {
    if (!had.current && logs.length > 0) setOpen(true);
    had.current = logs.length > 0;
    boxRef.current?.scrollTo(0, boxRef.current.scrollHeight);
  }, [logs, open]);

  const copy = async () => {
    try { await navigator.clipboard.writeText(logs.join("\n")); setCopied(true); setTimeout(() => setCopied(false), 1500); }
    catch { /* papan klip ditolak browser: tombolnya diam, isinya masih bisa diseleksi */ }
  };
  const last = logs[logs.length - 1];

  return (
    <div className={"panel log-panel" + (open ? "" : " closed")}>
      <div className="log-head">
        <button type="button" className="group-title group-toggle log-toggle"
          aria-expanded={open} onClick={() => setOpen(!open)}>
          <span>{t("log")}</span>
          {!open && <span className="log-last">{last ? <LogLine line={last} /> : t("logEmpty")}</span>}
          <ChevronDown className={"ico chev" + (open ? " open" : "")} aria-hidden="true" />
        </button>
        <button type="button" className="ghost tiny" onClick={copy} disabled={!logs.length}
          title={t("logCopy")} aria-label={t("logCopy")}>
          <Copy className="ico" aria-hidden="true" /> {copied ? t("copied") : t("logCopy")}
        </button>
      </div>
      {open && (
        <div className="logbox" ref={boxRef} role="log" aria-live="polite">
          {logs.length
            ? logs.map((l, i) => <div key={i}><LogLine line={l} /></div>)
            : <div className="meta">{t("logEmpty")}</div>}
        </div>
      )}
    </div>
  );
}

// Satu baris log: stempel waktu diredupkan, sisanya diwarnai menurut isinya.
function LogLine({ line }: { line: string }) {
  const m = line.match(/^(\[\d{2}:\d{2}:\d{2}\])\s?(.*)$/s);
  const ts = m ? m[1] : "";
  const text = m ? m[2] : line;
  return <>{ts && <span className="ts">{ts} </span>}<span className={kind(text)}>{text}</span></>;
}

function kind(text: string): string {
  if (/^✓|\bdone\b|\bfinished\b|\bselesai\b/i.test(text)) return "ok";
  if (/^⚠|^✕|error|failed|gagal|could not|refused/i.test(text)) return "err";
  if (/^(transcribing|scoring|correcting|rendering|extracting|writing)\b/i.test(text)) return "stage";
  return "";
}
