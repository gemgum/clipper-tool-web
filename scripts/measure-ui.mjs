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
// Yang dicetak: scrollHeight vs clientHeight untuk <html>, .screen-main, dan
// .screen-col, di dua ukuran jendela dan dua tema. Angka "over" > 0 berarti
// kotak itu bergulir. Halaman yang bergulir dicap GAGAL.
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

// Yang diukur. scrollHeight > clientHeight berarti kotak itu MENGGULIR.
const PROBE = `(() => {
  const box = (el) => el ? { c: el.clientHeight, s: el.scrollHeight, over: el.scrollHeight - el.clientHeight } : null;
  const q = (s) => box(document.querySelector(s));
  return JSON.stringify({
    body: box(document.body),
    doc: { c: document.documentElement.clientHeight, s: document.documentElement.scrollHeight },
    main: q('.screen-main'), col: q('.screen-col'), rail: q('.rail'),
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
      await new Promise((r) => setTimeout(r, 1400));
      const m = await send("Runtime.evaluate", { expression: PROBE, returnByValue: true });
      results.push({ theme, page: name, size: label, ...JSON.parse(m.result.value) });
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
console.log("halaman      tema    jendela    body(c/s)      main over  col over");
for (const r of results) {
  const scrolls = r.doc.s > r.doc.c + 1;
  if (scrolls) bad.push(`${r.page} ${r.theme} ${r.size}`);
  console.log(
    `${r.page.padEnd(13)}${r.theme.padEnd(8)}${r.size.padEnd(11)}` +
    `${String(r.doc.c + "/" + r.doc.s).padEnd(15)}` +
    `${String(r.main?.over ?? "-").padEnd(11)}${r.col?.over ?? "-"}` +
    (scrolls ? "   <-- HALAMAN BERGULIR" : "")
  );
}
console.log(bad.length ? `\nGAGAL: ${bad.join(", ")}` : "\nOK: tidak ada halaman yang bergulir.");
