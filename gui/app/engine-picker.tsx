"use client";

// Pemilih mesin LLM — SATU bentuk untuk seluruh aplikasi (notes/39).
//
// Sebelumnya tiap tab merakit pilihannya sendiri: halaman klip menyebutnya
// "Scoring engine", tab kartu "Engine", tab pembuat berita punya yang ketiga —
// dan menambah satu penyedia berarti menyentuh tiga tempat. Satu komponen
// berarti penyedia berikutnya cuma satu baris di tabel engine.
//
// Yang TIDAK ada di sini juga disengaja: kunci API tinggal di setelan utama,
// sebab ia disetel sekali seumur pemasangan sementara mesin & model dipilih
// tiap kali bekerja.

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Plug } from "lucide-react";
import { eng } from "./engine";
import { useI18n } from "./i18n";

export type EngineInfo = {
  id: string;
  name: string;
  kind: string;
  base_url: string;
  model: string;
  has_key: boolean;
  ready: boolean;
  keys_url?: string;
  // Mesin yang ditambahkan pengguna sendiri; hanya ini yang bisa dihapus.
  user?: boolean;
};

/** Nilai khusus: bukan mesin, melainkan pintu ke setelan. */
export const ADD_ENGINE = "__add";

/** useEngines menarik daftar mesin beserta keadaannya. */
export function useEngines() {
  const [engines, setEngines] = useState<EngineInfo[]>([]);
  const [loaded, setLoaded] = useState(false);
  const reload = useCallback(() => {
    fetch(eng("/api/engines"))
      .then((r) => r.json())
      .then((d) => setEngines(d.engines || []))
      .catch(() => setEngines([]))
      .finally(() => setLoaded(true));
  }, []);
  useEffect(() => { reload(); }, [reload]);
  return { engines, loaded, reload };
}

type Result = { ok: boolean; schema: boolean; strict: boolean; error?: string } | null;

export default function EnginePicker({
  engines, engine, setEngine, model, setModel, busy = false, title, extra = [], children,
}: {
  engines: EngineInfo[];
  engine: string;
  setEngine: (v: string) => void;
  model: string;
  /** Bentuk setter useState: komponen ini ikut MENGISI model bila kosong. */
  setModel: React.Dispatch<React.SetStateAction<string>>;
  busy?: boolean;
  /** Nama tahap, bila di satu halaman ada lebih dari satu pemilih. */
  title?: string;
  /** Pilihan yang BUKAN mesin LLM, mis. "Heuristic (no AI)" di halaman klip.
      Saat salah satunya dipilih, kotak model dan tombol uji ikut hilang —
      keduanya tidak punya arti tanpa LLM di belakangnya. */
  extra?: { id: string; name: string }[];
  /** Kendali tambahan milik tab itu sendiri, mis. tombol Analyse. */
  children?: React.ReactNode;
}) {
  const { t } = useI18n();
  const router = useRouter();
  const [models, setModels] = useState<string[]>([]);
  const [testing, setTesting] = useState(false);
  const [result, setResult] = useState<Result>(null);

  const ready = engines.filter((e) => e.ready);
  const current = engines.find((e) => e.id === engine);
  const isExtra = extra.some((x) => x.id === engine);

  // Mesin yang tidak (lagi) siap tidak boleh tetap terpilih diam-diam: kuncinya
  // bisa saja baru dihapus, dan job yang dijalankan dengan mesin itu gagal
  // dengan pesan yang membingungkan.
  useEffect(() => {
    if (ready.length === 0 || isExtra) return;
    if (!ready.some((e) => e.id === engine)) setEngine(ready[0].id);
  }, [engines]); // eslint-disable-line react-hooks/exhaustive-deps

  // Daftar model ditarik tiap kali mesinnya berganti. Endpoint-nya sengaja
  // TERPISAH dari /test: ini satu GET, sedangkan menguji memanggil LLM
  // sungguhan — dan mengganti pilihan mesin tidak boleh berbiaya token.
  // Mesin BERGANTI = model ikut berganti.
  //
  // Nama model milik satu penyedia tidak berarti apa-apa di penyedia lain, dan
  // membiarkannya bukan cuma janggal: memilih DeepSeek dengan "llama3.1" masih
  // tertulis di kotaknya mengirim permintaan berbayar untuk model yang tidak
  // ada di sana. Dilaporkan pemilik proyek, 18 Agustus 2026.
  //
  // Ref, bukan perbandingan biasa: yang boleh menimpa ketikan pengguna hanya
  // PERGANTIAN mesin, bukan render pertama halaman — di render pertama, model
  // yang tersimpan dari sesi sebelumnya harus dibiarkan.
  const prevEngine = useRef("");
  useEffect(() => {
    if (!engine || isExtra) return;
    let alive = true;
    setResult(null);
    if (prevEngine.current && prevEngine.current !== engine) {
      setModel(engines.find((e) => e.id === engine)?.model ?? "");
    }
    prevEngine.current = engine;
    fetch(eng(`/api/engines/${engine}/models`))
      .then((r) => r.json())
      .then((d) => {
        if (!alive) return;
        const list: string[] = d.models || [];
        setModels(list);
        // Kotak model yang kosong BUKAN pilihan yang aman: engine akan memakai
        // bawaan kliennya ("qwen2.5"), dan di komputer yang cuma punya
        // llama3.1 job itu gagal saat dijalankan. Kalau servernya menyebut apa
        // yang ia punya, pakai yang pertama.
        setModel((cur) => (cur.trim() === "" && list.length > 0 ? list[0] : cur));
      })
      .catch(() => { if (alive) setModels([]); });
    return () => { alive = false; };
  }, [engine]); // eslint-disable-line react-hooks/exhaustive-deps

  const test = async () => {
    setTesting(true);
    setResult(null);
    try {
      const r = await fetch(eng("/api/engines/test"), {
        method: "POST", headers: { "content-type": "application/json" },
        body: JSON.stringify({ id: engine, model }),
      });
      const d = await r.json();
      setResult({ ok: !!d.ok, schema: !!d.schema, strict: !!d.strict, error: d.error });
      if (d.models?.length) setModels(d.models);
    } catch (e) {
      setResult({ ok: false, schema: false, strict: false, error: String(e) });
    } finally {
      setTesting(false);
    }
  };

  const listID = `models-${engine}-${title || ""}`;

  return (
    <>
      {title && <div className="engine-stage meta">{title}</div>}
      <div className="engine-picker">
      <div className="field">
        <label htmlFor={"ep-engine-" + (title || "")}>{t("engineLabel")}</label>
        <select id={"ep-engine-" + (title || "")} value={engine} disabled={busy}
          onChange={(e) => {
            // Baris terakhir bukan mesin, melainkan pintu: tanpa itu, pengguna
            // yang belum pernah mengisi kunci Gemini tidak akan pernah tahu
            // Gemini didukung — dropdown-nya cuma berisi yang lokal.
            if (e.target.value === ADD_ENGINE) { router.push("/requirements"); return; }
            setEngine(e.target.value);
          }}>
          {ready.length === 0 && <option value="">{t("engineNone")}</option>}
          {ready.map((e) => <option key={e.id} value={e.id}>{e.name}</option>)}
          {extra.map((e) => <option key={e.id} value={e.id}>{e.name}</option>)}
          <option disabled>──────────</option>
          <option value={ADD_ENGINE}>{t("engineAdd")}</option>
        </select>
      </div>

      {!isExtra && (
      <div className="field">
        <label htmlFor={"ep-model-" + (title || "")}>
          {t("modelLabel")}{current ? ` · ${current.name}` : ""}
        </label>
        {/* Daftar dari server adalah BANTUAN, bukan pagar: model yang terbit
            kemarin harus bisa dipakai hari ini, jadi kotaknya tetap bisa
            diketik (notes/39). */}
        <input id={"ep-model-" + (title || "")} list={listID} value={model} disabled={busy}
          placeholder={current?.model || ""}
          onChange={(e) => setModel(e.target.value)} />
        <datalist id={listID}>
          {models.map((m) => <option key={m} value={m} />)}
        </datalist>
      </div>
      )}

      {!isExtra && (
      <div className="field engine-actions">
        {/* Tempatnya SELALU ada, juga saat belum diuji: tombol yang muncul
            setelah sesuatu terjadi menggeser kendali di sebelahnya. */}
        <label className="engine-result">
          {result?.ok && !result.strict && (
            <span className="meta" title={t("engineNoSchemaTip")}>{t("engineNoSchema")}</span>
          )}
          {result?.ok && result.strict && <span className="ok">{t("engineOK")}</span>}
          {result && !result.ok && (
            <span className="bad" title={result.error}>{t("engineFailed")}</span>
          )}
        </label>
        <button className="ghost" onClick={test} disabled={busy || testing || !engine}>
          <Plug className="ico" aria-hidden="true" /> {testing ? t("engineTesting") : t("engineTest")}
        </button>
      </div>
      )}

      {children}
      </div>
    </>
  );
}
