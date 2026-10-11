"use client";

// Halaman Pengaturan (dulu Requirements) — DESIGN-Clipper-Lanjutan.md §7:
// tiga tab (Mesin AI · Program & model · Lokasi berkas) dan panel Kesiapan
// sistem yang selalu terlihat. Logika pasang/hapus/path/folder tidak berubah.
//
// Halaman Requirements: daftar komponen, statusnya, dan tombol pasang.
//
// Ini yang menggantikan setup.sh bagi pengguna aplikasi. Engine yang tahu apa
// yang ada dan apa yang kurang — halaman ini hanya menampilkan jawabannya dan
// meneruskan tombolnya, supaya "apa yang belum terpasang" tidak pernah jadi
// tebakan.

import { useCallback, useEffect, useRef, useState } from "react";
import PageHeader from "../page-header";
import { useI18n } from "../i18n";
import { eng, engineURL, useWeb } from "../engine";
import Picker from "../picker";
import Alerts from "../alerts";
import EngineSettings from "./engines";
import Warn from "../warn";
import AISettingsPanel from "./ai-settings";
import { useAISettings } from "../ai";
import { useEngines } from "../engine-picker";

type Component = {
  id: string;
  name: string;
  kind: "tool" | "model" | "app";
  required: boolean;
  installed: boolean;
  path: string;
  detail: string;
  size: string;
  installable: boolean;
  hint: string;
  url: string;
  pointable: boolean;
};

type Folders = {
  clips_dir: string;
  cards_dir: string;
  clips_dir_used: string;
  cards_dir_used: string;
};

type Requirements = {
  components: Component[];
  missing: string[];
  data_dir: string;
  models_dir: string;
  tools_dir: string;
  dev: boolean;
};

// Kemajuan pemasangan, apa adanya dari engine.
//
// Halaman ini TIDAK lagi menjalankan unduhannya. Ia hanya menonton: engine yang
// mengunduh, di latar, dan tetap jalan walau jendela ditinggal atau ditutup.
type Install = {
  id: string;
  running: boolean;
  value: number;
  message: string;
  bytes: number;
  total: number;
  error?: string;
  done: boolean;
};

export default function RequirementsPage() {
  const { t } = useI18n();
  // Mode web: komponen & folder server diurus admin lewat setup.sh, bukan dari
  // browser — tombolnya disembunyikan, statusnya tetap terlihat (notes/42).
  const web = useWeb();
  const [req, setReq] = useState<Requirements | null>(null);
  const [error, setError] = useState("");
  const [installs, setInstalls] = useState<Record<string, Install>>({});
  const [picking, setPicking] = useState<Component | null>(null);
  const [folders, setFolders] = useState<Folders | null>(null);
  // Folder mana yang sedang dipilih: "clips" | "cards" | null.
  const [pickingFolder, setPickingFolder] = useState<null | "clips" | "cards">(null);
  const [busy, setBusy] = useState(true);
  // Pesan hasil per komponen (selesai / gagal), hilang saat dicoba lagi.
  const [notes, setNotes] = useState<Record<string, string>>({});
  const abort = useRef<AbortController | null>(null);
  const [tab, setTab] = useState<"ai" | "programs" | "files">("ai");
  const { data: ai } = useAISettings();
  const { engines } = useEngines();

  const load = useCallback(async () => {
    setBusy(true);
    try {
      const res = await fetch(eng(`/api/requirements`));
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "failed");
      setReq(data);
      setError("");
      const fs = await fetch(eng(`/api/settings`)).then((r) => r.json());
      setFolders(fs);
    } catch {
      setError(t("engineUnreachable", { url: engineURL() }));
    } finally {
      setBusy(false);
    }
  }, [t]);

  useEffect(() => {
    load();
    return () => abort.current?.abort();
  }, [load]);

  // install hanya MEMULAI. Kemajuannya datang lewat langganan di bawah, jadi
  // menutup atau meninggalkan halaman ini tidak menghentikan apa pun.
  const install = useCallback(
    async (c: Component) => {
      setNotes((n) => ({ ...n, [c.id]: "" }));
      try {
        const res = await fetch(eng(`/api/requirements/install`), {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ id: c.id }),
        });
        if (!res.ok) {
          const data = await res.json().catch(() => ({}));
          throw new Error(data.error || `HTTP ${res.status}`);
        }
      } catch (e: any) {
        setNotes((n) => ({ ...n, [c.id]: `⚠ ${e.message}` }));
      }
    },
    []
  );

  // Langganan kabar pemasangan. Pesan pertama dari engine berisi keadaan
  // terkini, jadi halaman yang baru dibuka langsung menampilkan unduhan yang
  // sedang berjalan — termasuk yang dimulai sebelum halaman ini dibuka.
  useEffect(() => {
    const es = new EventSource(eng(`/api/requirements/events`));
    es.addEventListener("install", (ev) => {
      const st: Install = JSON.parse((ev as MessageEvent).data);
      setInstalls((prev) => ({ ...prev, [st.id]: st }));
      if (st.done) load();
    });
    return () => es.close();
  }, [load]);

  const remove = useCallback(
    async (c: Component) => {
      try {
        const res = await fetch(eng(`/api/requirements/remove`), {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ id: c.id }),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error);
        setNotes((n) => ({ ...n, [c.id]: t("reqRemoved") }));
        load();
      } catch (e: any) {
        setNotes((n) => ({ ...n, [c.id]: `⚠ ${e.message}` }));
      }
    },
    [load, t]
  );

  // Engine mengirim daftar kosong, bukan null — tapi versi lama tidak, dan satu
  // baris pertahanan di sini lebih murah daripada halaman yang gagal dirender.
  const missing = req?.missing ?? [];

  // Menunjuk program yang sudah ada di komputer, bagi yang tidak mau (atau
  // tidak bisa) mengunduh ulang — dan bagi Chrome/Edge yang memang tidak
  // pernah kita unduh sendiri.
  const setPath = useCallback(
    async (c: Component, path: string) => {
      setPicking(null);
      try {
        const res = await fetch(eng(`/api/requirements/path`), {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ id: c.id, path }),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error);
        setNotes((n) => ({ ...n, [c.id]: t("reqPathSaved") }));
        load();
      } catch (e: any) {
        setNotes((n) => ({ ...n, [c.id]: `⚠ ${e.message}` }));
      }
    },
    [load, t]
  );

  // Menyimpan tempat klip / kartu. Folder kosong = kembali ke folder data.
  const saveFolder = useCallback(
    async (which: "clips" | "cards", dir: string) => {
      setPickingFolder(null);
      try {
        const res = await fetch(eng(`/api/settings/folders`), {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ [which]: dir }),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error);
        setFolders(data);
      } catch (e: any) {
        setError(e.message);
      }
    },
    []
  );

  // Yang kurang DAN bisa dipasang engine — dipakai tombol "pasang semua" di
  // notifikasi. Yang tidak bisa dipasang (Ollama, Chrome) tidak ikut: tombol
  // yang menjanjikan sesuatu yang tidak bisa dikerjakan lebih buruk daripada
  // tidak ada tombol.
  const installable = (req?.components || []).filter((c) => c.required && !c.installed && c.installable);
  const anyInstalling = Object.values(installs).some((st) => st.running);


  const comps = req?.components || [];
  const models = comps.filter((c) => c.kind === "model");
  const modelsIn = models.filter((c) => c.installed);
  const gbUsed = modelsIn.reduce((sum, c) => sum + (parseFloat((c.size || "").replace(/[^0-9.]/g, "")) * (/GB/i.test(c.size) ? 1 : 0.001) || 0), 0);
  const ffmpegOk = comps.filter((c) => c.id === "ffmpeg" || c.id === "ffprobe").every((c) => c.installed);
  const chrome = comps.find((c) => /chrome/i.test(c.id) || /chrome/i.test(c.name));
  const globalAI = ai?.global;
  const aiInfo = engines.find((e) => e.id === globalAI?.engine);
  const aiReady = globalAI?.engine === "heuristic" || !!aiInfo?.ready;
  const ready: { name: string; ok: boolean; status: string }[] = [
    { name: t("setReadyAI"), ok: aiReady, status: aiReady ? t("setStatusReady") : t("setStatusNotReady") },
    { name: t("setReadyFfmpeg"), ok: ffmpegOk, status: ffmpegOk ? t("setStatusReady") : t("setStatusMissing") },
    { name: t("setReadyModels"), ok: modelsIn.length > 0, status: t("setModelsInstalled", { n: modelsIn.length }) },
    { name: t("setReadyChrome"), ok: !!chrome?.installed, status: chrome?.installed ? t("setStatusReady") : t("setStatusMissing") },
  ];

  // Satu baris komponen, sama untuk program dan model (DESIGN §9 ModelRow).
  const row = (c: Component) => {
    const st = installs[c.id];
    const live = st?.running ?? false;
    return (
      <div className="set-row" key={c.id}>
        <span className={"set-dot " + (c.installed ? "ok" : c.required ? "bad" : "idle")} aria-hidden="true" />
        <div className="set-main">
          <p className="set-name">
            {c.name}{c.size && <span className="meta"> · {c.size}</span>}
            {st?.error && <Warn>{st.error}</Warn>}
          </p>
          <p className="meta">{live ? st.message : c.detail}</p>
          {c.installed && c.path && !web && <p className="set-path">{c.path}</p>}
          {!c.installed && !c.installable && c.hint && <p className="meta">{c.hint}</p>}
          {notes[c.id] && <p className="meta">{notes[c.id]}</p>}
          {live && <div className="bar slim"><div style={{ width: `${Math.max(0, st.value) * 100}%` }} /></div>}
        </div>
        <span className={"set-label " + (c.installed ? "ok" : "")}>{c.installed ? t("setInstalled") : c.required ? t("setStatusMissing") : t("setNotInstalled")}</span>
        {!web && (
          <div className="set-actions">
            {!c.installed && c.installable && (
              <button type="button" className="ghost" disabled={live} onClick={() => install(c)}>{live ? t("reqInstalling") : t("setDownload")}</button>
            )}
            {c.pointable && (
              <button type="button" className="ghost" disabled={live} onClick={() => setPicking(c)}>{c.installed ? t("reqPathChange") : t("reqPathPick")}</button>
            )}
            {!c.installed && !c.installable && c.url && (
              <a className="btn-ghost" href={c.url} target="_blank" rel="noreferrer">{t("reqOpenDownload")} ↗</a>
            )}
            {c.installed && c.kind === "model" && (
              <button type="button" className="ghost" disabled={live} onClick={() => remove(c)}>{t("reqRemove")}</button>
            )}
          </div>
        )}
      </div>
    );
  };

  return (
    <main className="screen scroll clips-v2 settings-v2">
      <PageHeader title={t("tabRequirements")} subtitle={t("subRequirements")}>
        <button type="button" className="ghost" disabled={busy} onClick={load}>{busy ? t("loading") : t("reqRefresh")}</button>
      </PageHeader>
      {pickingFolder && (
        <Picker mode="folder"
          start={pickingFolder === "clips" ? folders?.clips_dir_used : folders?.cards_dir_used}
          onPick={(p) => saveFolder(pickingFolder, p)} onClose={() => setPickingFolder(null)} />
      )}
      {picking && (
        <Picker mode="file" title={t("pickerProgramTitle", { name: picking.name })} hint={t("pickerProgramHint")}
          start={picking.path} onPick={(p) => setPath(picking, p)} onClose={() => setPicking(null)} />
      )}
      <Alerts items={[
        error && { kind: "error" as const, text: error },
        missing.length > 0 && { kind: "warn" as const, key: `missing-${missing.join(",")}`,
          text: <>{t("reqMissing", { list: missing.join(", ") })}{" "}
            {installable.length > 0 && !web && (
              <button className="ghost tiny" disabled={anyInstalling} onClick={() => installable.forEach(install)}>
                {anyInstalling ? t("reqInstalling") : t("reqInstallMissing", { n: installable.length })}
              </button>
            )}</> },
      ]} />

      {/* Tiga tab dengan garis bawah aksen (DESIGN §9 SettingsTabs). */}
      <div className="set-tabs" role="tablist" aria-label={t("tabRequirements")}>
        {/* Mode web: tab lokasi berkas dibuang — folder server bukan urusan tim. */}
        {([["ai", t("setTabAI")], ["programs", t("setTabPrograms")], ...(web ? [] : [["files", t("setTabFiles")]])] as [typeof tab, string][]).map(([id, label]) => (
          <button key={id} type="button" role="tab" aria-selected={tab === id} className={tab === id ? "on" : ""}
            onClick={() => setTab(id)}>{label}</button>
        ))}
      </div>

      <div className="set-body">
        <div className="set-content" role="tabpanel">
          {tab === "ai" && (
            <>
              <AISettingsPanel />
              <section className="card">
                <h2>{t("setOtherEngines")}</h2>
                <p className="step-hint flush">{t("setOtherEnginesHint")}</p>
                <EngineSettings />
              </section>
            </>
          )}

          {tab === "programs" && (
            <>
              <section className="card">
                <h2>{t("reqGroupTools")}</h2>
                {comps.filter((c) => c.kind === "tool").map(row)}
              </section>
              <section className="card">
                <div className="card-head">
                  <h2>{t("setSpeechModels")}</h2>
                  <span className="meta">{t("setModelsSummary", { n: modelsIn.length, m: models.length, gb: gbUsed.toFixed(1) })}</span>
                </div>
                <p className="step-hint flush">{t("setSpeechModelsHint")}</p>
                {models.map(row)}
              </section>
              {!web && comps.some((c) => c.kind === "app") && (
                <details className="card adv">
                  <summary>{t("setLocalAIGuide")}</summary>
                  <p className="meta adv-hint">{t("setLocalAIGuideHint")}</p>
                  {comps.filter((c) => c.kind === "app").map(row)}
                </details>
              )}
            </>
          )}

          {tab === "files" && req && !web && (
            <section className="card">
              <h2>{t("setFilesTitle")}</h2>
              {([
                ["clips", t("reqFolderClips"), folders?.clips_dir_used, folders?.clips_dir],
                ["cards", t("reqFolderCards"), folders?.cards_dir_used, folders?.cards_dir],
              ] as const).map(([key, label, used, custom]) => (
                <div className="set-row" key={key}>
                  <div className="set-main">
                    <p className="set-name">{label}</p>
                    <p className="set-path">{used || "–"}</p>
                    {!custom && <p className="meta">{t("reqFolderDefault")}</p>}
                  </div>
                  {!web && (
                    <div className="set-actions">
                      <button type="button" className="ghost" onClick={() => setPickingFolder(key)}>{t("reqFolderChange")}</button>
                      {custom && <button type="button" className="ghost" onClick={() => saveFolder(key, "")}>{t("reqFolderReset")}</button>}
                    </div>
                  )}
                </div>
              ))}
              {([[t("reqWhereModels"), req.models_dir], [t("reqWhereTools"), req.tools_dir], [t("reqWhereData"), req.data_dir]] as const).map(([label, p]) => (
                <div className="set-row" key={label}>
                  <div className="set-main"><p className="set-name">{label}</p><p className="set-path">{p}</p></div>
                </div>
              ))}
              {req.dev && <p className="meta">{t("reqDevNote")}</p>}
            </section>
          )}
        </div>

        {/* Kesiapan sistem: empat baris yang selalu terlihat (§7). */}
        <aside className="card set-ready">
          <h2>{t("setReadyTitle")}</h2>
          {ready.map((r) => (
            <div className="set-ready-row" key={r.name}>
              <span className={"set-dot " + (r.ok ? "ok" : "bad")} aria-hidden="true" />
              <span className="grow">{r.name}</span>
              <span className={"set-label " + (r.ok ? "ok" : "bad")}>{r.status}</span>
            </div>
          ))}
          <p className="note">{t("setReadyNote")}</p>
        </aside>
      </div>
    </main>
  );
}
