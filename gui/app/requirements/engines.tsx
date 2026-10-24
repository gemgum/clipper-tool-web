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
import { post } from "../engine";
import { useI18n } from "../i18n";
import { hasAudioUse, useEngines } from "../engine-picker";
import { Group, Row } from "../tab-card";
import Select from "../select";

type Result = {
  ok: boolean; schema: boolean; strict: boolean; error?: string; models?: string[];
  mode?: string; heard?: string; expected?: string;
};
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
  // Kegunaan: "chat", "transcribe", atau "" (keduanya). Mesin baru = chat.
  const [use, setUse] = useState("chat");
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
    setUse(e ? (e.use ?? "chat") : "chat");
  }, [id, e?.name, e?.base_url, e?.model, e?.use]);

  const local = e?.kind === "local";
  // Pilihan kegunaan hanya untuk mesin yang punya endpoint audio (OpenAI-compatible).
  const showUse = isNew || (!!e && hasAudioUse(e));

  const save = async () => {
    setSaving(true); setResult(null);
    try {
      const body: Record<string, string> = { id: isNew ? "" : id, base_url: base, model };
      if (showUse) body.use = use;
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

  // Hanya kuncinya: alamat & model tetap, jadi mesin pabrik pun bisa "dicabut".
  const removeKey = async () => {
    if (!e || !(await ask(t("engineRemoveKeyConfirm", { name: e.name }), t("engineRemoveKey")))) return;
    try {
      await post("/api/engines", { id: e.id, api_key: "" });
      setKey(""); reload();
    } catch (err) {
      setResult({ ok: false, schema: false, strict: false, error: String(err) });
    }
  };

  // Yang diuji = yang TERSIMPAN, lewat jalur job yang sama (engine). Isian yang
  // belum disimpan tidak ikut: hasil Test harus sama dengan hasil job.
  const test = async (mode: "chat" | "transcribe") => {
    setTesting(true); setResult(null);
    try {
      setResult(await post("/api/engines/test", { id, model, mode }));
    } catch (err) {
      setResult({ ok: false, schema: false, strict: false, error: String(err) });
    } finally {
      setTesting(false);
    }
  };

  const dirty = !!e && (!!key || base !== (e.base_url || "") || model !== (e.model || "")
    || (showUse && use !== (e.use ?? "chat")) || (!!e.user && name !== e.name));
  const canTest = !!e && (e.ready || local) && !dirty;

  const status = !e ? "" : local
    ? (e.ready ? t("engineReady") : t("engineOffline"))
    : (e.ready ? t("engineReady") : t("engineNoKey"));

  return (
    <div className="engine-form">
      {confirmEl}
      {/* Baris label | kendali — resep kartu setelan (tab-card.tsx). */}
      <Group>
        <Row label={t("setEngineLabel")}>
          <Select value={id} onChange={setId} ariaLabel={t("setEngineLabel")} options={[
            ...engines.map((x) => ({ value: x.id, label: x.name })),
            { value: NEW, label: t("engineAddNew") },
          ]} />
          {e && (
            <span className="engine-form-status">
              <span className={"req-dot " + (e.ready ? "on" : "idle")} aria-hidden="true" />
              <span className="meta">{status}</span>
              {e.keys_url && !e.has_key && (
                <a className="meta" href={e.keys_url} target="_blank" rel="noreferrer">{t("engineGetKey")}</a>
              )}
            </span>
          )}
        </Row>
        {(isNew || e?.user) && (
          <Row label={t("engineNameLabel")}>
            <input id="eng-name" value={name} aria-label={t("engineNameLabel")} placeholder={t("engineNamePlaceholder")} onChange={(ev) => setName(ev.target.value)} />
          </Row>
        )}
        {!local && (
          <Row label={t("engineKeyLabel")}>
            <input id="eng-key" type="password" value={key} autoComplete="off" aria-label={t("engineKeyLabel")}
              placeholder={e?.has_key ? t("keyPlaceholderStored") : t("engineKeyPlaceholder")}
              onChange={(ev) => setKey(ev.target.value)} />
          </Row>
        )}
        <Row label={t("engineBaseLabel")}>
          <input id="eng-base" value={base} aria-label={t("engineBaseLabel")} placeholder={local ? t("llmServerAuto") : "https://api.example.com/v1"}
            onChange={(ev) => setBase(ev.target.value)} />
        </Row>
        {showUse && (
          <Row label={t("engineUseLabel")}>
            <Select id="eng-use" value={use} onChange={setUse} ariaLabel={t("engineUseLabel")} options={[
              { value: "chat", label: t("engineUseChat") },
              { value: "transcribe", label: t("engineUseTranscribe") },
              { value: "", label: t("engineUseBoth") },
            ]} />
          </Row>
        )}
        <Row label={t("engineModelLabel")}>
          <input id="eng-model" value={model} aria-label={t("engineModelLabel")} onChange={(ev) => setModel(ev.target.value)}
            placeholder={use === "transcribe" ? "whisper-1" : ""} />
        </Row>
      </Group>

      <div className="engine-form-actions">
        <button type="button" className="primary" onClick={save}
          disabled={saving || (isNew && (!name.trim() || !base.trim()))}>{isNew ? t("engineAdd") : t("save")}</button>
        {!isNew && (["chat", "transcribe"] as const)
          .filter((m) => m === "chat" ? (!showUse || use !== "transcribe") : showUse && use !== "chat")
          .map((m) => (
            <button key={m} type="button" className="ghost" onClick={() => test(m)} disabled={testing || !canTest}
              title={dirty ? t("engineSaveFirst") : m === "transcribe" ? t("engineTestAudioTip") : undefined}>
              <Plug className="ico" aria-hidden="true" />{" "}
              {testing ? t("engineTesting") : showUse && use === "" ? t(m === "chat" ? "engineTestChat" : "engineTestAudio")
                : m === "transcribe" ? t("engineTestAudio") : t("engineTest")}
            </button>
          ))}
        {e?.has_key && !local && <button type="button" className="ghost" onClick={removeKey}>{t("engineRemoveKey")}</button>}
        {e?.user && <button type="button" className="ghost" onClick={remove}>{t("engineRemove")}</button>}
        <span className="engine-result">
          {result?.ok && result.mode === "transcribe" && (
            <span className="ok" title={t("engineHeardExpected", { text: result.expected || "" })}>
              {t("engineHeard", { text: result.heard || "" })}
            </span>
          )}
          {result?.ok && result.mode !== "transcribe" && result.strict && <span className="ok">{t("engineOK")}</span>}
          {result?.ok && result.mode !== "transcribe" && !result.strict && <span className="meta" title={t("engineNoSchemaTip")}>{t("engineNoSchema")}</span>}
          {result && !result.ok && <span className="bad">{t("engineFailed")}: {result.error}</span>}
        </span>
      </div>
    </div>
  );
}
