// Jendela aplikasi Clipper.
//
// Sengaja setipis mungkin. Seluruh aplikasi — antarmuka maupun API — dilayani
// engine Go di 127.0.0.1; yang dikerjakan berkas ini hanya tiga hal:
//
//   1. menjalankan engine sebagai proses anak;
//   2. menunggu satu baris "clipper-url: …" di stdout-nya — di situ ada port
//      yang dipilih engine dan kunci sesinya;
//   3. mengarahkan jendela ke alamat itu, dan mematikan engine saat ditutup.
//
// Alasan alamatnya dibaca dari stdout, bukan dari berkas engine.json: shell
// adalah INDUK proses engine, jadi pipa itu pasti miliknya sendiri. Berkas bisa
// saja milik engine lain yang kebetulan sedang jalan.

#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::io::{BufRead, BufReader};
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::time::Duration;

use tauri::{Manager, RunEvent};

/// Awalan baris alamat yang dicetak `clipper serve -shell`.
/// Harus sama dengan api.ShellURLPrefix di engine.
const URL_PREFIX: &str = "clipper-url: ";

/// Proses engine, disimpan supaya bisa dimatikan saat jendela ditutup.
struct Engine(Mutex<Option<Child>>);

impl Engine {
    fn stop(&self) {
        if let Ok(mut guard) = self.lock_inner() {
            if let Some(child) = guard.as_mut() {
                let _ = child.kill();
                let _ = child.wait();
            }
            *guard = None;
        }
    }

    fn lock_inner(&self) -> Result<std::sync::MutexGuard<'_, Option<Child>>, ()> {
        self.0.lock().map_err(|_| ())
    }
}

/// Mencari biner engine.
///
/// Urutannya: timpaan env (untuk mencoba engine yang baru dibangun), lalu
/// folder resource bundel, lalu di sebelah jendela ini, dan terakhir bin/ di
/// dalam checkout — jalur yang dipakai saat `npm run tauri dev`.
fn engine_path(app: &tauri::AppHandle) -> Option<PathBuf> {
    engine_candidates(app).into_iter().find(|p| p.is_file())
}

/// Daftar tempat yang dicoba, dipakai juga untuk pesan galat: pengguna yang
/// aplikasinya diam saja tidak bisa menebak di mana kita mencari.
fn engine_candidates(app: &tauri::AppHandle) -> Vec<PathBuf> {
    let name = if cfg!(windows) { "clipper.exe" } else { "clipper" };
    let mut candidates: Vec<PathBuf> = Vec::new();

    // Timpaan env didahulukan: dipakai untuk mencoba engine yang baru dibangun
    // tanpa mengemas ulang aplikasinya.
    if let Ok(p) = std::env::var("CLIPPER_BIN") {
        candidates.push(PathBuf::from(p));
    }

    if let Ok(dir) = app.path().resource_dir() {
        candidates.push(dir.join("engine").join(name));
        candidates.push(dir.join(name));
    }
    if let Ok(exe) = std::env::current_exe() {
        if let Some(dir) = exe.parent() {
            candidates.push(dir.join("engine").join(name));
            candidates.push(dir.join(name));
        }
    }
    // Checkout sumber: desktop/src-tauri → ../../bin/clipper
    candidates.push(PathBuf::from("../../bin").join(name));
    candidates
}

/// Menjalankan engine dan mengarahkan jendela ke alamat yang dilaporkannya.
fn start_engine(app: &tauri::AppHandle) -> Result<(), String> {
    let exe = engine_path(app).ok_or_else(|| {
        let tried: Vec<String> = engine_candidates(app)
            .iter()
            .map(|p| p.display().to_string())
            .collect();
        format!(
            "The Clipper engine was not found. Looked in:\n{}",
            tried.join("\n")
        )
    })?;

    let mut cmd = Command::new(&exe);
    // stderr ikut ditangkap: di build rilis aplikasi ini tidak punya konsol,
    // jadi pesan galat engine tidak ke mana-mana kecuali kita membacanya.
    cmd.arg("serve")
        .arg("-shell")
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());
    // Tanpa ini, Windows membuka jendela konsol hitam di samping aplikasi.
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        const CREATE_NO_WINDOW: u32 = 0x0800_0000;
        cmd.creation_flags(CREATE_NO_WINDOW);
    }

    let mut child = cmd
        .spawn()
        .map_err(|e| format!("cannot start the engine ({}): {e}", exe.display()))?;
    let stdout = child
        .stdout
        .take()
        .ok_or_else(|| "the engine gave no output to read".to_string())?;
    let stderr = child.stderr.take();

    app.state::<Engine>()
        .lock_inner()
        .map_err(|_| "engine state is poisoned".to_string())?
        .replace(child);

    // Semua keluaran engine dikumpulkan supaya bisa ditampilkan bila gagal.
    let log: Arc<Mutex<Vec<String>>> = Arc::new(Mutex::new(Vec::new()));
    let opened = Arc::new(AtomicBool::new(false));

    if let Some(stderr) = stderr {
        let log = log.clone();
        std::thread::spawn(move || {
            for line in BufReader::new(stderr).lines().map_while(Result::ok) {
                eprintln!("{line}");
                push_line(&log, line);
            }
        });
    }

    let handle = app.clone();
    let stdout_log = log.clone();
    let stdout_opened = opened.clone();
    std::thread::spawn(move || {
        // Dibaca sampai habis, bukan berhenti di baris alamat: kalau pipa ini
        // tidak dikosongkan, engine akan macet begitu penyangganya penuh.
        for line in BufReader::new(stdout).lines().map_while(Result::ok) {
            if !stdout_opened.load(Ordering::SeqCst) {
                if let Some(url) = line.strip_prefix(URL_PREFIX) {
                    stdout_opened.store(true, Ordering::SeqCst);
                    open_window(&handle, url.trim());
                    continue;
                }
            }
            println!("{line}");
            push_line(&stdout_log, line);
        }
        // Pipa habis tanpa alamat = engine mati sebelum siap. Tanpa cabang ini
        // jendelanya menunggu selamanya di halaman "Starting the engine…" —
        // persis kegagalan yang paling membingungkan pengguna.
        if !stdout_opened.load(Ordering::SeqCst) {
            show_error(
                &handle,
                "The engine stopped before it was ready",
                &format!("{}\n\n{}", exe.display(), collected(&stdout_log)),
            );
        }
    });

    // Engine yang hidup tapi tidak pernah melapor (mis. tertahan firewall)
    // tidak menutup pipanya, jadi cabang di atas tidak akan kena. Batas waktu
    // ini yang menangkapnya.
    let timeout_handle = app.clone();
    let timeout_log = log.clone();
    let timeout_opened = opened.clone();
    std::thread::spawn(move || {
        std::thread::sleep(Duration::from_secs(30));
        if !timeout_opened.load(Ordering::SeqCst) {
            show_error(
                &timeout_handle,
                "The engine did not report an address",
                &collected(&timeout_log),
            );
        }
    });

    Ok(())
}

/// Menyimpan satu baris keluaran engine, dibatasi agar tidak tumbuh selamanya.
fn push_line(log: &Arc<Mutex<Vec<String>>>, line: String) {
    if let Ok(mut v) = log.lock() {
        if v.len() < 200 {
            v.push(line);
        }
    }
}

fn collected(log: &Arc<Mutex<Vec<String>>>) -> String {
    match log.lock() {
        Ok(v) if v.is_empty() => "(the engine printed nothing)".to_string(),
        Ok(v) => v.join("\n"),
        Err(_) => String::new(),
    }
}

/// Mengarahkan jendela ke alamat aplikasi.
fn open_window(app: &tauri::AppHandle, url: &str) {
    let Some(window) = app.get_webview_window("main") else {
        return;
    };
    match tauri::Url::parse(url) {
        Ok(parsed) => {
            if let Err(e) = window.navigate(parsed) {
                eprintln!("cannot open {url}: {e}");
            }
        }
        Err(e) => eprintln!("the engine reported an address we cannot read ({url}): {e}"),
    }
}

/// Menampilkan kegagalan DI DALAM jendela.
///
/// Aplikasi rilis tidak punya konsol, jadi eprintln tidak sampai ke mana pun.
/// Halaman yang menyebut apa yang dicoba dan apa jawabannya adalah satu-satunya
/// cara pengguna tahu harus berbuat apa — dan satu-satunya cara ia bisa
/// menyalin pesannya untuk dilaporkan.
fn show_error(app: &tauri::AppHandle, title: &str, detail: &str) {
    let Some(window) = app.get_webview_window("main") else {
        return;
    };
    let html = format!(
        "<!doctype html><meta charset=\"utf-8\"><title>Clipper</title>\
<body style=\"font:14px/1.6 system-ui;background:#0f1115;color:#e6e9ef;padding:40px\">\
<h1 style=\"font-size:18px;margin:0 0 6px\">{}</h1>\
<p style=\"color:#9aa4b2;margin:0 0 18px\">Clipper could not start. The details below are what the engine reported.</p>\
<pre style=\"background:#1e222b;padding:14px;border-radius:8px;white-space:pre-wrap;word-break:break-word;font-size:12px\">{}</pre>\
</body>",
        escape(title),
        escape(detail)
    );
    // data: URL, bukan berkas sementara: tidak ada yang perlu dibersihkan, dan
    // tidak bergantung pada izin baca berkas milik webview.
    let url = format!("data:text/html;charset=utf-8,{}", encode(&html));
    if let Ok(parsed) = tauri::Url::parse(&url) {
        let _ = window.navigate(parsed);
    }
}

fn escape(s: &str) -> String {
    s.replace('&', "&amp;").replace('<', "&lt;").replace('>', "&gt;")
}

/// Percent-encoding seadanya untuk data: URL — hanya karakter yang jelas aman
/// yang dibiarkan, sisanya dikodekan. Lebih baik terlalu banyak dikodekan
/// daripada URL yang rusak justru saat sedang melaporkan kerusakan lain.
fn encode(s: &str) -> String {
    let mut out = String::with_capacity(s.len() * 2);
    for b in s.as_bytes() {
        match b {
            b'A'..=b'Z' | b'a'..=b'z' | b'0'..=b'9' | b'-' | b'_' | b'.' | b'~' => {
                out.push(*b as char)
            }
            _ => out.push_str(&format!("%{b:02X}")),
        }
    }
    out
}

fn main() {
    let app = tauri::Builder::default()
        .manage(Engine(Mutex::new(None)))
        .setup(|app| {
            if let Err(e) = start_engine(app.handle()) {
                // Jendela tetap dibuka dan menjelaskan sebabnya; aplikasi yang
                // hilang tanpa sepatah kata jauh lebih buruk.
                eprintln!("{e}");
                show_error(app.handle(), "The engine could not be started", &e);
            }
            Ok(())
        })
        .build(tauri::generate_context!())
        .expect("cannot start Clipper");

    app.run(|app, event| {
        if let RunEvent::ExitRequested { .. } | RunEvent::Exit = event {
            app.state::<Engine>().stop();
        }
    });
}
