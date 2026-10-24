// Bukti bahwa jendela tidak bergulir — bukan klaim.
//
// Aturan pertama di CLAUDE.md ("jendela tidak boleh bergulir") pernah dilanggar
// berkali-kali justru karena tidak ada yang mengukurnya; tampilan dinilai dari
// membaca CSS, dan membaca CSS tidak pernah memberi tahu tinggi sebenarnya.
// Berkas ini menutup lubang itu.
//
// Pakai:
//   1) ./bin/clipper serve            (di terminal lain)
//   2) chrome --headless --remote-debugging-port=9333 \
//        --user-data-dir=/tmp/cdp about:blank
//   3) node scripts/measure-ui.mjs http://127.0.0.1:8787 /tmp/shots
//
// Chrome-nya boleh dari mana saja; yang dipakai selama ini:
//   ~/.cache/ms-playwright/chromium-*/chrome-linux64/chrome
// (`npx playwright install chromium`, tanpa root). Biner yang SAMA juga bisa
// dipakai engine untuk merender kartu berita lewat CLIPPER_CHROME.
//
// Yang dicetak (tata letak 24 Oktober 2026, acuan halaman Watermark):
//   scroll = gulir .screen.scroll — HARUS 0 di halaman berkolom (setup-grid /
//            io-grid): yang bergulir hanya kolom kiri/hasil.
//   stage  = atas..bawah kartu pratinjau, sebelum DAN sesudah kolom kiri
//            digulir ke dasar — angka atas yang berubah = pratinjau bergerak.
//   side   = lebih tinggi kartu pratinjau dari kolomnya (> 0 = tidak muat).
// GAGAL bila halaman bergulir, pratinjau bergeser, atau kartunya tidak muat di
// 1240x860. Di 900x600 "side" hanya dilaporkan (jaring pengaman: kolomnya
// bergulir sendiri).
import { mkdirSync, writeFileSync } from "node:fs";

const PORT = 9333;
const BASE = process.argv[2] || "http://127.0.0.1:8787";
const OUT = process.argv[3] || ".";
mkdirSync(OUT, { recursive: true });

const pages = [
  ["clips", "/"],
  ["news", "/news"],
  ["writer", "/writer"],
  ["captions", "/captions"],
  ["watermark", "/watermark"],
  ["requirements", "/requirements"],
  ["history", "/history"],
];
// Jendela terkecil yang diizinkan tauri.conf.json, dan ukuran bawaannya.
// Tinggi VIEWPORT = tinggi jendela dikurangi bilah judul (~95 px terukur).
const sizes = (process.env.SIZES ? JSON.parse(process.env.SIZES) : [["900x600", 900, 505], ["1240x860", 1240, 765]]);
const themes = ["light", "dark"];

const rpc = (ws) => {
  let id = 0;
  const waiting = new Map();
  ws.addEventListener("message", (e) => {
    const m = JSON.parse(e.data);
    if (m.id && waiting.has(m.id)) { waiting.get(m.id)(m); waiting.delete(m.id); }
  });
  return (method, params = {}, sessionId) =>
    new Promise((res) => {
      const n = ++id;
      waiting.set(n, (m) => res(m.result ?? m));
      ws.send(JSON.stringify({ id: n, method, params, sessionId }));
    });
};

const open = async (url) => {
  const r = await fetch(`http://127.0.0.1:${PORT}/json/new?${encodeURIComponent(url)}`, { method: "PUT" });
  return r.json();
};

// Yang diukur — lihat keterangan di kepala berkas.
const PROBE = `(() => {
  const sc = document.querySelector('.screen.scroll');
  const st = document.querySelector('.stage');
  const side = document.querySelector('.setup-side');
  const col = document.querySelector('.setup-main, .io-in');
  const top = () => st ? Math.round(st.getBoundingClientRect().top) : null;
  const t0 = top(); if (col) col.scrollTop = 1e6; const t1 = top(); if (col) col.scrollTop = 0;
  return JSON.stringify({
    scroll: sc ? sc.scrollHeight - sc.clientHeight : 0,
    grid: !!document.querySelector('.setup-grid, .io-grid'),
    t0, t1, bottom: st ? Math.round(st.getBoundingClientRect().bottom) : null,
    side: side ? side.scrollHeight - side.clientHeight : null,
    theme: document.documentElement.dataset.theme || '(none)',
  });
})()`;

const results = [];
for (const theme of themes) {
  for (const [name, path] of pages) {
    for (const [label, w, h] of sizes) {
      const tab = await open(BASE + path);
      const ws = new WebSocket(tab.webSocketDebuggerUrl);
      await new Promise((r) => ws.addEventListener("open", r));
      const send = rpc(ws);
      await send("Page.enable");
      await send("Runtime.enable");
      await send("Emulation.setDeviceMetricsOverride",
        { width: w, height: h, deviceScaleFactor: 1, mobile: false });
      await send("Runtime.evaluate", {
        expression: `localStorage.setItem('clipper.theme','${theme}');document.documentElement.dataset.theme='${theme}'`,
      });
      await send("Page.reload");
      await new Promise((r) => setTimeout(r, 1800));
      const m = await send("Runtime.evaluate", { expression: PROBE, returnByValue: true });
      results.push({ theme, page: name, size: label, h, ...JSON.parse(m.result.value) });
      if (label === "1240x860") {
        const shot = await send("Page.captureScreenshot", { format: "png" });
        writeFileSync(`${OUT}/${name}-${theme}.png`, Buffer.from(shot.data, "base64"));
      }
      ws.close();
      await fetch(`http://127.0.0.1:${PORT}/json/close/${tab.id}`);
    }
  }
}

const bad = [];
console.log("halaman      tema    jendela    scroll stage(atas→sesudah gulir, bawah)  side");
for (const r of results) {
  const moved = r.t0 !== null && r.t0 !== r.t1;
  const strict = r.size === "1240x860";
  const fail = (r.grid && r.scroll > 1) || moved || (strict && r.side > 0);
  if (fail) bad.push(`${r.page} ${r.theme} ${r.size}`);
  const stage = r.t0 === null ? "-" : `${r.t0}→${r.t1}, ${r.bottom}/${r.h}`;
  console.log(
    `${r.page.padEnd(13)}${r.theme.padEnd(8)}${r.size.padEnd(11)}${String(r.scroll).padEnd(6)}` +
    `${stage.padEnd(34)}${r.side ?? "-"}` + (fail ? "   <-- GAGAL" : "")
  );
}
console.log(bad.length ? `\nGAGAL: ${bad.join(", ")}` : "\nOK: halaman tidak bergulir, pratinjau diam dan muat.");
