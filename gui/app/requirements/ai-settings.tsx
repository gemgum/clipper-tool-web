"use client";

import { Fragment, useEffect, useState } from "react";
import { useI18n } from "../i18n";
import { eng } from "../engine";
import { saveAI, useAISettings } from "../ai";
import type { AISettings, AITool } from "../ai";
import { useEngines } from "../engine-picker";
import type { EngineInfo } from "../engine-picker";

// Tab "Mesin AI" (DESIGN-Clipper-Lanjutan §7): mesin yang dipakai SEMUA alat,
// uji koneksi, dan pengecualian per alat. Kunci & alamat tiap mesin tetap
// diisi di daftar "Mesin lain" di bawahnya (EngineSettings).
export default function AISettingsPanel() {
  const { t } = useI18n();
  const { data, setData } = useAISettings();
  const { engines } = useEngines();
  const [editing, setEditing] = useState(false);
  const [test, setTest] = useState<{ ok: boolean; text: string } | null>(null);
  const [testing, setTesting] = useState(false);

  const g = data?.global;
  const info = engines.find((e) => e.id === g?.engine);
  const ready = !!info?.ready;
  const host = (() => { try { return info?.base_url ? new URL(info.base_url).host : ""; } catch { return info?.base_url || ""; } })();

  const runTest = async () => {
    if (!g) return;
    setTesting(true); setTest(null);
    try {
      const res = await fetch(eng("/api/engines/test"), {
        method: "POST", headers: { "content-type": "application/json" },
        body: JSON.stringify({ id: g.engine, model: g.model }),
      });
      const d = await res.json();
      setTest(d.ok ? { ok: true, text: t("setTestOk") } : { ok: false, text: t("setTestFailed", { error: d.error || `HTTP ${res.status}` }) });
    } catch (e: any) {
      setTest({ ok: false, text: t("setTestFailed", { error: e.message }) });
    } finally { setTesting(false); }
  };

  const tools: { id: AITool; name: string }[] = [
    { id: "clips", name: t("tabClips") }, { id: "news", name: t("tabNews") },
    { id: "writer", name: t("tabWriter") }, { id: "captions", name: t("tabCaptions") },
  ];

  return (
    <>
      <section className="card">
        <h2>{t("setCurrentEngine")}</h2>
        <p className="step-hint flush">{t("setCurrentEngineHint")}</p>
        {g && !editing && (
          <div className="ai-current">
            <span className={"set-dot " + (ready ? "ok" : "bad")} aria-hidden="true" />
            <div className="grow">
              <p className="set-name">{info?.name || g.engine} <span className="meta">· {ready ? t("setStatusReady") : (info && !info.has_key ? t("setNoKey") : t("setNotRunning"))}</span></p>
              <p className="ai-sub">{[g.model, host].filter(Boolean).join(" · ")}</p>
            </div>
            <button type="button" className="ai-btn" onClick={runTest} disabled={testing}>{testing ? t("llmTesting") : t("setTest")}</button>
            <button type="button" className="ghost" onClick={() => setEditing(true)}>{t("setChange")}</button>
          </div>
        )}
        {g && editing && (
          <Chooser engines={engines} initial={g} allowHeuristic={false}
            onCancel={() => setEditing(false)}
            onSave={async (c) => { setData(await saveAI("", c.engine, c.model)); setEditing(false); setTest(null); }} />
        )}
        {test && <p className={test.ok ? "set-ok" : "set-err"} role="status">{test.text}</p>}
      </section>

      <section className="card">
        <h2>{t("setExceptions")}</h2>
        <p className="step-hint flush">{t("setExceptionsHint")}</p>
        {data && tools.map((tool) => (
          <Fragment key={tool.id}>
            {tool.id === "clips" ? (
              <ClipsRow name={tool.name} engines={engines} current={data.overrides.clips} transcriber={data.transcriber}
                onSave={async (c, tr) => {
                  await saveAI("clips", c?.engine || "", c?.model || "");
                  setData(await saveAI("transcribe", tr.engine, tr.model));
                }} />
            ) : (
              <ExceptionRow name={tool.name} engines={engines} current={data.overrides[tool.id]}
                allowHeuristic={false}
                onSave={async (c) => setData(await saveAI(tool.id, c?.engine || "", c?.model || ""))} />
            )}
          </Fragment>
        ))}
      </section>
    </>
  );
}

// Pemilih mesin + model. Model boleh kosong = model bawaan mesin itu.
function Chooser({ engines, initial, allowHeuristic, onSave, onCancel }: {
  engines: EngineInfo[]; initial: { engine: string; model: string }; allowHeuristic: boolean;
  onSave: (c: { engine: string; model: string }) => Promise<void>; onCancel?: () => void;
}) {
  const { t } = useI18n();
  const [engine, setEngine] = useState(initial.engine);
  const [model, setModel] = useState(initial.model);
  const [models, setModels] = useState<string[]>([]);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  useEffect(() => {
    if (!engine || engine === "heuristic") { setModels([]); return; }
    fetch(eng(`/api/engines/${engine}/models`)).then((r) => r.json()).then((d) => setModels(d.models || [])).catch(() => setModels([]));
  }, [engine]);
  const ready = engines.filter((e) => e.ready || e.id === engine);
  const listID = `models-${engine}`;
  return (
    <div className="ai-chooser">
      <div className="field">
        <label>{t("setEngineLabel")}</label>
        <select value={engine} onChange={(e) => { setEngine(e.target.value); setModel(""); }}>
          {ready.map((e) => <option key={e.id} value={e.id}>{e.name}</option>)}
          {allowHeuristic && <option value="heuristic">{t("offlineHeuristic")}</option>}
        </select>
      </div>
      {engine !== "heuristic" && (
        <div className="field">
          <label>{t("setModelLabel")}</label>
          <input list={listID} value={model} placeholder={engines.find((e) => e.id === engine)?.model || ""} onChange={(e) => setModel(e.target.value)} />
          <datalist id={listID}>{models.map((m) => <option key={m} value={m} />)}</datalist>
        </div>
      )}
      <div className="ai-chooser-actions">
        <button type="button" className="primary" disabled={saving || !engine} onClick={async () => {
          setSaving(true); setError("");
          try { await onSave({ engine, model: engine === "heuristic" ? "" : model }); } catch (e: any) { setError(e.message); }
          finally { setSaving(false); }
        }}>{t("save")}</button>
        {onCancel && <button type="button" className="ghost" onClick={onCancel}>{t("cancel")}</button>}
      </div>
      {error && <p className="set-err">{error}</p>}
    </div>
  );
}

function ExceptionRow({ name, engines, current, allowHeuristic, onSave }: {
  name: string; engines: EngineInfo[]; current: { engine: string; model: string } | null; allowHeuristic: boolean;
  onSave: (c: { engine: string; model: string } | null) => Promise<void>;
}) {
  const { t } = useI18n();
  const [open, setOpen] = useState(false);
  const label = current
    ? `${current.engine === "heuristic" ? t("offlineHeuristic") : (engines.find((e) => e.id === current.engine)?.name || current.engine)}${current.model ? " · " + current.model : ""}`
    : t("setFollowMain");
  return (
    <div className="set-row">
      <div className="set-main">
        <p className="set-name">{name}</p>
        <p className="meta">{label}</p>
        {open && (
          <Chooser engines={engines} initial={current || { engine: engines.find((e) => e.ready)?.id || "", model: "" }}
            allowHeuristic={allowHeuristic} onCancel={() => setOpen(false)}
            onSave={async (c) => { await onSave(c); setOpen(false); }} />
        )}
      </div>
      {!open && (
        <div className="set-actions">
          <button type="button" className="ghost" onClick={() => setOpen(true)}>{current ? t("setChange") : t("setUseOther")}</button>
          {current && <button type="button" className="ghost" onClick={() => onSave(null)}>{t("setFollowMainBtn")}</button>}
        </div>
      )}
    </div>
  );
}

// Baris Video clips: mesin skor + transkripsi dalam SATU form (permintaan
// pemilik 9 Oktober 2026) — keduanya setelan alat yang sama, dua tombol Save
// untuk satu alat cuma membuat orang menekan yang salah.
//
// Transkripsi: whisper di mesin ini, Google AI Studio, atau mesin
// OpenAI-compatible (notes/43). Kuncinya TIDAK diisi di sini: semua kunci di
// satu form, Engines & keys (AI Studio = mesin "Google AI Studio (Gemini)").
function ClipsRow({ name, engines, current, transcriber, onSave }: {
  name: string; engines: EngineInfo[]; current: { engine: string; model: string } | null;
  transcriber: AISettings["transcriber"];
  onSave: (c: { engine: string; model: string } | null, tr: { engine: string; model: string }) => Promise<void>;
}) {
  const { t } = useI18n();
  const [open, setOpen] = useState(false);
  // "" = ikut mesin utama.
  const [engine, setEngine] = useState(current?.engine || "");
  const [model, setModel] = useState(current?.model || "");
  const [models, setModels] = useState<string[]>([]);
  const [tEngine, setTEngine] = useState(transcriber.engine);
  const [tModel, setTModel] = useState(transcriber.model);
  const [tModels, setTModels] = useState<string[]>([]);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  const fetchModels = (id: string, set: (m: string[]) => void) =>
    fetch(eng(`/api/engines/${id}/models`)).then((r) => r.json()).then((d) => set(d.models || [])).catch(() => set([]));
  useEffect(() => {
    if (!open || !engine || engine === "heuristic") { setModels([]); return; }
    fetchModels(engine, setModels);
  }, [open, engine]);
  useEffect(() => {
    if (!open || tEngine === "whisper") { setTModels([]); return; }
    fetchModels(tEngine === "aistudio" ? "gemini" : tEngine, setTModels);
  }, [open, tEngine]);

  const ready = engines.filter((e) => e.ready || e.id === engine);
  // Gemini lewat pilihan AI Studio, sebab jalur OpenAI-nya tanpa endpoint audio.
  const audio = engines.filter((e) => e.kind === "openai" && e.id !== "gemini" && (e.ready || e.id === tEngine));
  const nameOf = (id: string) => engines.find((e) => e.id === id)?.name || id;
  const tName = (id: string) => id === "whisper" ? t("transcriberWhisper") : id === "aistudio" ? t("transcriberAIStudio") : nameOf(id);
  const engineLabel = current
    ? `${current.engine === "heuristic" ? t("offlineHeuristic") : nameOf(current.engine)}${current.model ? " · " + current.model : ""}`
    : t("setFollowMain");
  const trLabel = transcriber.engine === "whisper" ? tName("whisper")
    : `${tName(transcriber.engine)} · ${transcriber.model || "whisper-1"}${transcriber.key_set ? "" : " · " + (transcriber.engine === "aistudio" ? t("aiStudioNoKey") : t("setNoKey"))}`;

  const begin = () => {
    setEngine(current?.engine || ""); setModel(current?.model || "");
    setTEngine(transcriber.engine); setTModel(transcriber.model); setError(""); setOpen(true);
  };
  const save = async () => {
    setSaving(true); setError("");
    try {
      await onSave(engine ? { engine, model: engine === "heuristic" ? "" : model } : null, { engine: tEngine, model: tModel });
      setOpen(false);
    } catch (e: any) { setError(e.message); }
    finally { setSaving(false); }
  };

  return (
    <div className="set-row">
      <div className="set-main">
        <p className="set-name">{name}</p>
        <p className="meta">{engineLabel}</p>
        <p className={transcriber.key_set ? "meta" : "set-err"} title={t("transcriberTip")}>{t("transcriber")}: {trLabel}</p>
        {open && (
          <>
            <div className="ai-chooser">
              <div className="field">
                <label>{t("setEngineLabel")}</label>
                <select value={engine} onChange={(e) => { setEngine(e.target.value); setModel(""); }}>
                  <option value="">{t("setFollowMain")}</option>
                  {ready.map((e) => <option key={e.id} value={e.id}>{e.name}</option>)}
                  <option value="heuristic">{t("offlineHeuristic")}</option>
                </select>
              </div>
              <div className="field">
                <label>{t("setModelLabel")}</label>
                <input list="models-clips" value={model} disabled={!engine || engine === "heuristic"}
                  placeholder={engines.find((e) => e.id === engine)?.model || ""} onChange={(e) => setModel(e.target.value)} />
                <datalist id="models-clips">{models.map((m) => <option key={m} value={m} />)}</datalist>
              </div>
            </div>
            <div className="ai-chooser">
              <div className="field">
                <label title={t("transcriberTip")}>{t("transcriber")}</label>
                <select value={tEngine} onChange={(e) => { setTEngine(e.target.value); setTModel(""); }}>
                  <option value="whisper">{t("transcriberWhisper")}</option>
                  <option value="aistudio">{t("transcriberAIStudio")}</option>
                  {audio.map((e) => <option key={e.id} value={e.id}>{e.name}</option>)}
                </select>
              </div>
              <div className="field">
                <label>{t("setModelLabel")}</label>
                <input list="models-transcribe" value={tModel} disabled={tEngine === "whisper"}
                  placeholder={tEngine === "aistudio" ? "gemini-3.5-flash" : tEngine === "whisper" ? "" : "whisper-1"}
                  title={tEngine === "aistudio" || tEngine === "whisper" ? undefined : t("transcribeModelTip")}
                  onChange={(e) => setTModel(e.target.value)} />
                <datalist id="models-transcribe">{tModels.map((m) => <option key={m} value={m} />)}</datalist>
              </div>
            </div>
            <div className="ai-chooser-actions end">
              <button type="button" className="primary" disabled={saving} onClick={save}>{t("save")}</button>
              <button type="button" className="ghost" onClick={() => setOpen(false)}>{t("cancel")}</button>
            </div>
            {error && <p className="set-err">{error}</p>}
          </>
        )}
      </div>
      {!open && (
        <div className="set-actions">
          <button type="button" className="ghost" onClick={begin}>{t("setChange")}</button>
        </div>
      )}
    </div>
  );
}
