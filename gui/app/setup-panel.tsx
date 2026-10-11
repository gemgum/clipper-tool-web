"use client";

// Ikon: lucide-react (ISC) — alasannya di gui/app/page.tsx.
import { Download, Plug } from "lucide-react";
import Tip from "./tip";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useI18n } from "./i18n";
import { eng } from "./engine";
import Select from "./select";
import Warn from "./warn";
import { sameModel, useOllama } from "./ollama";
import { useEngines } from "./engine-picker";

export type WhisperModel = { name: string; size: string; downloaded: boolean };

// Semua yang disetel sebelum menekan Mulai: mesin, mutu, jumlah klip, dan mesin
// skor. Dulu dua panel terpisah (Render settings + AI engine), digabung 6
// Agustus 2026 — keduanya menjawab pertanyaan yang sama ("job ini dijalankan
// bagaimana"), dan dua bingkai + dua judul memakan tinggi yang justru membuat
// kolom ini harus digulir.
//
// Yang dipegang sendiri komponen ini: kunci API, status Ollama, dan unduhan
// model — tidak ada yang lain di halaman ini memakainya. Yang dititipkan
// halaman tetap di atas, sebab job dan preset tersimpan membutuhkannya.
export default function SetupPanel({
  model, setModel, models,
  resolution, setResolution, quality, setQuality, fps, setFps,
  durationPreset, setDurationPreset, maxClips, setMaxClips, saveMode, setSaveMode,
  engine, setEngine, llmModel, setLlmModel,
  transcriptFix, setTranscriptFix, terms, setTerms, addLog,
  testing, setTesting, transcriber, hideClips,
}: {
  model: string; setModel: (v: string) => void;
  models: WhisperModel[];
  resolution: string; setResolution: (v: string) => void;
  quality: string; setQuality: (v: string) => void;
  fps: number; setFps: (v: number) => void;
  durationPreset: string; setDurationPreset: (v: string) => void;
  maxClips: number; setMaxClips: (v: number) => void;
  saveMode: string; setSaveMode: (v: string) => void;
  /** Mesin skor: id dari daftar mesin bersama, atau "heuristic". */
  engine: string; setEngine: (v: string) => void;
  llmModel: string; setLlmModel: React.Dispatch<React.SetStateAction<string>>;
  transcriptFix: boolean; setTranscriptFix: (v: boolean) => void;
  terms: string; setTerms: (v: string) => void;
  addLog: (text: string) => void;
  /** Dititipkan halaman: tombol Mulai di panel sebelah ikut mati selama uji. */
  testing: boolean; setTesting: (v: boolean) => void;
  /** Mesin transkripsi dari Pengaturan; model whisper hanya berlaku untuk "whisper". */
  transcriber: string;
  /** Halaman klip baru: jumlah, panjang, dan versi simpan ada di kartu 4. */
  hideClips?: boolean;
}) {
  const { t } = useI18n();
  const { engines } = useEngines();

  const [pulling, setPulling] = useState(false);
  // Kemajuan unduhan model: -1 = besarnya belum diketahui (tahap verifikasi,
  // atau manifest belum terbaca). Ditampilkan DI DALAM tombol, bukan sebagai
  // baris baru — baris yang muncul-hilang menggeser seluruh kolomnya.
  const [pullPct, setPullPct] = useState(-1);
  // Hasil uji model: null = belum dicoba.
  const [ping, setPing] = useState<{
    ok: boolean; error?: string; ms: number;
    steps?: { name: string; ok: boolean; detail?: string; error?: string; ms: number }[];
  } | null>(null);

  // Kunci API TIDAK diisi di sini lagi: ia disetel sekali seumur pemasangan di
  // halaman setelan, sedangkan mesin & model dipilih tiap kali bekerja
  // (notes/39). Dulu kunci Claude hanya bisa diisi dari panel ini.
  const ollamaActive = engine === "ollama";
  // Status & daftar model dari hook bersama (ollama.ts): tab kartu memakai yang
  // sama persis, jadi keduanya tidak bisa lagi menampilkan bentuk berbeda untuk
  // data yang sama.
  const { status: ollamaStatus, installed: ollamaInstalled, check: checkOllama } = useOllama(ollamaActive);

  const selectedOllama = useMemo(
    () => ollamaInstalled.find((m) => sameModel(m.name, llmModel)),
    [ollamaInstalled, llmModel],
  );

  // Auto-pilih: kalau pilihan sekarang belum terpasang, ambil model terpasang
  // yang dinilai siap lebih dulu; kalau tak ada yang siap, ambil yang pertama.
  useEffect(() => {
    if (!ollamaStatus?.running || !ollamaInstalled.length) return;
    const match = ollamaInstalled.find((m) => sameModel(m.name, llmModel));
    if (match) {
      // Nama disamakan dengan yang TERPASANG ("qwen2.5" → "qwen2.5:latest").
      // Tanpa ini kotak pilihan menampilkan nilai yang tidak ada di daftarnya,
      // jadi ia terlihat kosong — dan job tetap berjalan dengan nama tersimpan
      // yang lama, yang membuat pesan galat menyebut model yang tidak pernah
      // terasa dipilih.
      if (match.name !== llmModel) setLlmModel(match.name);
      return;
    }
    setLlmModel((ollamaInstalled.find((m) => m.ready) || ollamaInstalled[0]).name);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ollamaInstalled, ollamaStatus?.running]);

  // Tanpa Ollama, jangan memaksa setelan yang PASTI gagal.
  //
  // Bawaannya "offline + Ollama", dan di komputer tanpa Ollama itu berarti job
  // pertama berhenti dengan galat sebelum satu klip pun jadi — padahal
  // heuristik ada, gratis, dan tidak butuh apa pun. Diputuskan SEKALI saat
  // status pertama datang; sesudah itu pilihan pengguna yang berlaku.
  //
  // Ini bukan pelanggaran "tanpa fallback" (notes/12): yang dipilih di sini
  // adalah SETELAN AWAL yang terlihat di layar, bukan penggantian mesin
  // diam-diam saat job sudah berjalan.
  const autoPicked = useRef(false);
  useEffect(() => {
    if (autoPicked.current || ollamaStatus === null || !ollamaActive) return;
    autoPicked.current = true;
    if (!ollamaStatus.running) {
      setEngine("heuristic");
      setTranscriptFix(false);
      addLog(t("logNoOllama"));
    }
  }, [ollamaStatus, ollamaActive, setEngine, setTranscriptFix, addLog, t]);



  // "Terpasang" tidak sama dengan "bisa dipakai", dan "bisa membalas sapaan"
  // ternyata juga tidak: berkali-kali di komputer baru Ollama jalan, qwen2.5
  // terpasang, tombol ini hijau — dan job klip tetap berhenti. Karena itu yang
  // dijalankan sekarang DUA TAHAP LLM yang sama persis dengan yang dipakai job
  // (koreksi transkrip + pemilihan momen) atas isi contoh yang dipaku di engine.
  // Hasil tiap tahap masuk log, sebab tahap MANA yang gagal itulah petunjuknya.

  // Unduhan model BERJALAN DI LATAR; halaman ini cuma berlangganan kabarnya.
  //
  // Permintaannya menjawab 202 seketika, lalu kemajuannya datang lewat SSE yang
  // sama dengan pemasangan komponen. Sebelum ini POST-nya ditunggu sampai
  // selesai: satu permintaan HTTP yang diam belasan menit untuk model 5 GB,
  // tanpa persentase — dan kalau halamannya dimuat ulang, unduhannya ikut mati.
  const pullModel = useCallback(async () => {
    setPulling(true);
    setPullPct(-1);
    addLog(t("logPullStart", { model: llmModel }));
    try {
      const res = await fetch(eng(`/api/ollama/pull`), {
        method: "POST", headers: { "content-type": "application/json" },
        body: JSON.stringify({ model: llmModel }),
      });
      // 409 = sudah berjalan. Itu bukan galat: langganan di bawah tetap
      // menampilkan kemajuannya, jadi menekan tombol dua kali tidak merusak apa
      // pun dan tidak perlu dijelaskan ke pengguna.
      if (!res.ok && res.status !== 409) throw new Error((await res.json()).error || "failed");
    } catch (e: any) {
      setPulling(false);
      addLog(t("logPullFailed", { error: e.message }));
    }
  }, [llmModel, addLog, t]);

  // Kemajuan unduhan model DIDENGARKAN terus, bukan hanya sesudah tombolnya
  // ditekan di sesi ini.
  //
  // Unduhannya hidup di engine, bukan di halaman: memuat ulang halaman di
  // tengah unduhan 5 GB tidak boleh membuat tombolnya kembali berkata "Download"
  // seolah tidak ada apa-apa. Engine mengirim cuplikan keadaan saat SSE
  // tersambung, jadi halaman yang baru dibuka langsung tahu apa yang berjalan.
  useEffect(() => {
    if (!ollamaActive) return;
    const es = new EventSource(eng(`/api/requirements/events`));
    es.addEventListener("install", (ev) => {
      const st = JSON.parse((ev as MessageEvent).data);
      if (!String(st.id || "").startsWith("llm-model:")) return;
      if (!st.done) { setPulling(true); setPullPct(st.value); return; }
      setPulling(false);
      if (st.error) addLog(t("logPullFailed", { error: st.error }));
      else { addLog(t("logPullDone", { model: String(st.id).slice(10) })); checkOllama(); }
    });
    return () => es.close();
  }, [ollamaActive, addLog, checkOllama, t]);

  // "downloading… 42%" — persentasenya hanya bila besarnya diketahui.
  const pullLabel = pullPct >= 0
    ? `${t("downloading")} ${Math.round(pullPct * 100)}%`
    : t("downloading");

  return (
    <div className="panel">
      <div className="group">
        <div className="group-title" role="heading" aria-level={3}>{t("groupEngine")}</div>
        <div className="grid3">
          <div className="field"><label>{t("whisperModel")}</label>
            <Select value={model} onChange={setModel} disabled={transcriber !== "whisper"} options={models.map((m) => ({
              value: m.name, label: m.name,
              note: m.downloaded ? m.size : t("modelNotDownloaded"),
            }))} /></div>
        </div>

        {/* Mesin skor memakai pemilih yang SAMA dengan tab kartu berita dan
            pembuat berita (notes/39). Dulu panel ini merakit pilihannya sendiri:
            "Mode" offline/hybrid, kunci Claude, model Claude, mesin offline,
            model lokal — lima kendali untuk satu pertanyaan, dan kunci Claude
            hanya bisa diisi dari sini. Yang tersisa di sini cuma yang memang
            khusus mesin lokal: mengunduh model yang belum ada. */}
        <div style={{ marginTop: 12 }}>
          {/* Mesin AI dipilih SEKALI di Pengaturan untuk semua alat
              (DESIGN-Clipper-Lanjutan §1). Di sini hanya ditampilkan, dengan
              jalan ke tempat mengubahnya. */}
          <p className="ai-line">
            {t("aiInUse")} <b>{engine === "heuristic" ? t("offlineHeuristic") : (engines.find((e) => e.id === engine)?.name || engine)}</b>
            {engine !== "heuristic" && llmModel && <code>{llmModel}</code>}
            <a href="/requirements">{t("aiChangeInSettings")}</a>
          </p>
            {ollamaActive && (
              <div className="field engine-actions">
                <label className="engine-result">
                  {ollamaStatus !== null && !ollamaStatus.running ? (
                    <Warn>{t("ollamaNotDetected")} <code>ollama serve</code>
                      <button className="ghost tiny" onClick={() => checkOllama()}>{t("recheck")}</button>
                    </Warn>
                  ) : ollamaStatus?.running && !selectedOllama ? (
                    <Warn>{t("modelNeedsDownload")}
                      <button className="ghost tiny" onClick={pullModel} disabled={pulling}>
                        {pulling ? pullLabel : <><Download className="ico" aria-hidden="true" /> {t("downloadModel")}</>}
                      </button>
                    </Warn>
                  ) : selectedOllama && !selectedOllama.ready ? (
                    <Warn>{selectedOllama.note}
                      <button className="ghost tiny" onClick={pullModel} disabled={pulling}>
                        {pulling ? pullLabel : <><Download className="ico" aria-hidden="true" /> {t("downloadSelected")}</>}
                      </button>
                    </Warn>
                  ) : pulling ? (
                    <span className="meta">{pullLabel}</span>
                  ) : null}
                </label>
              </div>
            )}
        </div>

        {/* Koreksi transkrip berlaku di kedua mode, jadi ditaruh di luar
            percabangan hybrid/offline. Ia memakai LLM juga saat mesin skornya
            heuristik — peringatannya di sini supaya tidak mengagetkan saat job
            berhenti. */}
        <div className="field" style={{ marginTop: 12 }}>
          <label className="chk">
            <input type="checkbox" checked={transcriptFix}
              onChange={(e) => setTranscriptFix(e.target.checked)} /> {t("transcriptFix")} <Tip text={t("transcriptFixTip")} />
            {transcriptFix && engine === "heuristic" && (
              <Warn>{t("transcriptFixNeedsLLM")}</Warn>
            )}
          </label>
          {transcriptFix && (
            <>
              {/* Daftar istilah menempel di bawah koreksi transkrip karena hanya
                  tahap itu yang memakainya — tanpa centang di atas, isian ini
                  tidak berpengaruh apa pun. */}
              <label htmlFor="terms">{t("terms")}</label>
              <input id="terms" type="text" value={terms}
                placeholder={t("termsPlaceholder")}
                onChange={(e) => setTerms(e.target.value)} />
            </>
          )}
        </div>
      </div>

      <div className="group">
        <div className="group-title" role="heading" aria-level={3}>{t("groupQuality")}</div>
        <div className="grid3">
          <div className="field"><label>{t("resolution")}</label>
            <Select value={resolution} onChange={setResolution} options={[
              { value: "720p", label: "720p" }, { value: "1080p", label: "1080p" },
              { value: "1440p", label: "1440p" },
            ]} /></div>
          <div className="field"><label>{t("quality")}</label>
            <Select value={quality} onChange={setQuality} options={[
              { value: "draft", label: t("qualityDraft") }, { value: "hd", label: t("qualityHd") },
              { value: "max", label: t("qualityMax") },
            ]} /></div>
          <div className="field"><label>{t("fps")} <Tip text={t("fpsTip")} /></label>
            <Select value={String(fps)} onChange={(v) => setFps(Number(v))} options={[
              { value: "0", label: t("fpsSource") }, { value: "24", label: "24" },
              { value: "30", label: "30" }, { value: "60", label: "60" },
            ]} /></div>
        </div>
      </div>

      {!hideClips && <div className="group">
        <div className="group-title" role="heading" aria-level={3}>{t("groupClips")}</div>
        <div className="grid3">
          <div className="field"><label>{t("clipDuration")} <Tip text={t("clipDurationTip")} /></label>
            <Select value={durationPreset} onChange={setDurationPreset} options={[
              { value: "auto", label: t("durationAuto") },
              ...["30s", "60s", "90s", "2 min", "3 min"].map((n, i) => ({
                value: ["30", "60", "90", "120", "180"][i], label: t("durationAbout", { n }),
              })),
            ]} /></div>
          <div className="field"><label>{t("maxClips")} <Tip text={t("maxClipsTip")} /></label>
            <input type="number" min={1} max={50} value={maxClips} onChange={(e) => setMaxClips(Number(e.target.value))} /></div>
          <div className="field"><label>{t("saveClips")} <Tip text={t("saveClipsTip")} /></label>
            <Select value={saveMode} onChange={setSaveMode} options={[
              { value: "burn", label: t("saveBurn") }, { value: "clean", label: t("saveClean") },
              { value: "both", label: t("saveBoth") },
            ]} /></div>
        </div>
      </div>}
    </div>
  );
}
