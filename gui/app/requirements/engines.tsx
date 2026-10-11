"use client";

// Bagian "Mesin AI" di halaman setelan (notes/39).
//
// Di SINI kunci API diisi, dan hanya di sini. Tab-tab hanya MEMILIH mesin yang
// sudah siap — kunci disetel sekali, mesin & model dipilih tiap kali bekerja.
//
// SATU form, bukan satu baris per mesin: pilih mesinnya, isiannya muncul di
// bawah. Pilihan terakhir "Tambah mesin baru" membuka form yang sama dalam
// keadaan kosong (nama, kunci, alamat, model bawaan) untuk gateway lain yang
// bicara /chat/completions.

import { useConfirm } from "../confirm";
import { useEffect, useState } from "react";
import { Plug } from "lucide-react";
import { eng } from "../engine";
import { useI18n } from "../i18n";
import { useEngines } from "../engine-picker";
import Select from "../select";

type Result = { ok: boolean; schema: boolean; strict: boolean; error?: string; models?: string[] };
const NEW = "__new";

export default function EngineSettings() {
  const { engines, reload } = useEngines();
  const { t } = useI18n();
  const [ask, confirmEl] = useConfirm();
  const [id, setId] = useState("");
  const [name, setName] = useState("");
  const [key, setKey] = useState("");
  const [base, setBase] = useState("");
  const [model, setModel] = useState("");
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [result, setResult] = useState<Result | null>(null);

  // Mesin pertama terpilih begitu daftarnya datang.
  useEffect(() => { if (!id && engines.length) setId(engines[0].id); }, [engines, id]);
  const e = engines.find((x) => x.id === id);
  const isNew = id === NEW;
  // Isian diisi ulang dari mesin yang dipilih; kunci tidak pernah ditampilkan.
  useEffect(() => {
    setKey(""); setResult(null);
    setName(e?.name || ""); setBase(e?.base_url || ""); setModel(e?.model || "");
  }, [id, e?.name, e?.base_url, e?.model]);

  const local = e?.kind === "local";

  const post = async (path: string, body: object) => {
    const r = await fetch(eng(path), {
      method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body),
    });
    const d = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(d.error || `HTTP ${r.status}`);
    return d;
  };

  const save = async () => {
    setSaving(true); setResult(null);
    try {
      const body: Record<string, string> = { id: isNew ? "" : id, base_url: base, model };
      if (isNew || e?.user) body.name = name;
      // Kunci hanya dikirim bila diketik: field yang tidak dikirim tidak
      // disentuh engine, jadi menyimpan model saja tidak menghapus kunci.
      if (key) body.api_key = key;
      const saved = await post("/api/engines", body);
      setKey("");
      reload();
      if (isNew && saved.id) setId(saved.id);
    } catch (err) {
      setResult({ ok: false, schema: false, strict: false, error: String(err) });
    } finally {
      setSaving(false);
    }
  };

  const remove = async () => {
    if (!e?.user || !(await ask(t("engineRemoveConfirm", { name: e.name }), t("engineRemove")))) return;
    try {
      await post("/api/engines/delete", { id: e.id });
      setId(""); reload();
    } catch (err) {
      setResult({ ok: false, schema: false, strict: false, error: String(err) });
    }
  };

  const test = async () => {
    setTesting(true); setResult(null);
    try {
      setResult(await post("/api/engines/test", { id, model }));
    } catch (err) {
      setResult({ ok: false, schema: false, strict: false, error: String(err) });
    } finally {
      setTesting(false);
    }
  };

  const status = !e ? "" : local
    ? (e.ready ? t("engineReady") : t("engineOffline"))
    : (e.ready ? t("engineReady") : t("engineNoKey"));

  return (
    <div className="engine-form">
      {confirmEl}
      <div className="engine-form-pick">
        <div className="field grow">
          <label>{t("setEngineLabel")}</label>
          <Select value={id} onChange={setId} options={[
            ...engines.map((x) => ({ value: x.id, label: x.name })),
            { value: NEW, label: t("engineAddNew") },
          ]} />
        </div>
        {e && (
          <p className="engine-form-status">
            <span className={"req-dot " + (e.ready ? "on" : "idle")} aria-hidden="true" />
            <span className="meta">{status}</span>
            {e.keys_url && !e.has_key && (
              <a className="meta" href={e.keys_url} target="_blank" rel="noreferrer">{t("engineGetKey")}</a>
            )}
          </p>
        )}
      </div>

      <div className="engine-form-grid">
        {(isNew || e?.user) && (
          <div className="field">
            <label htmlFor="eng-name">{t("engineNameLabel")}</label>
            <input id="eng-name" value={name} placeholder={t("engineNamePlaceholder")} onChange={(ev) => setName(ev.target.value)} />
          </div>
        )}
        {!local && (
          <div className="field">
            <label htmlFor="eng-key">{t("engineKeyLabel")}</label>
            <input id="eng-key" type="password" value={key} autoComplete="off"
              placeholder={e?.has_key ? t("keyPlaceholderStored") : t("engineKeyPlaceholder")}
              onChange={(ev) => setKey(ev.target.value)} />
          </div>
        )}
        <div className="field">
          <label htmlFor="eng-base">{t("engineBaseLabel")}</label>
          <input id="eng-base" value={base} placeholder={local ? t("llmServerAuto") : "https://"}
            onChange={(ev) => setBase(ev.target.value)} />
        </div>
        <div className="field">
          <label htmlFor="eng-model">{t("engineModelLabel")}</label>
          <input id="eng-model" value={model} onChange={(ev) => setModel(ev.target.value)} />
        </div>
      </div>

      <div className="engine-form-actions">
        <button type="button" className="primary" onClick={save}
          disabled={saving || (isNew && (!name.trim() || !base.trim()))}>{isNew ? t("engineAdd") : t("save")}</button>
        {!isNew && (
          <button type="button" className="ghost" onClick={test} disabled={testing || !e?.ready}>
            <Plug className="ico" aria-hidden="true" /> {testing ? t("engineTesting") : t("engineTest")}
          </button>
        )}
        {e?.user && <button type="button" className="ghost" onClick={remove}>{t("engineRemove")}</button>}
        <span className="engine-result">
          {result?.ok && result.strict && <span className="ok">{t("engineOK")}</span>}
          {result?.ok && !result.strict && <span className="meta" title={t("engineNoSchemaTip")}>{t("engineNoSchema")}</span>}
          {result && !result.ok && <span className="bad">{t("engineFailed")}: {result.error}</span>}
        </span>
      </div>
    </div>
  );
}
