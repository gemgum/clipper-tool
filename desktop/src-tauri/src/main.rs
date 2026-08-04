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
use std::sync::Mutex;

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
    let name = if cfg!(windows) { "clipper.exe" } else { "clipper" };

    if let Ok(p) = std::env::var("CLIPPER_BIN") {
        let p = PathBuf::from(p);
        if p.is_file() {
            return Some(p);
        }
    }

    let mut candidates: Vec<PathBuf> = Vec::new();
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

    candidates.into_iter().find(|p| p.is_file())
}

/// Menjalankan engine dan mengarahkan jendela ke alamat yang dilaporkannya.
fn start_engine(app: &tauri::AppHandle) -> Result<(), String> {
    let exe = engine_path(app)
        .ok_or_else(|| "the clipper engine was not found next to this app".to_string())?;

    let mut cmd = Command::new(&exe);
    cmd.arg("serve").arg("-shell").stdout(Stdio::piped());
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

    app.state::<Engine>()
        .lock_inner()
        .map_err(|_| "engine state is poisoned".to_string())?
        .replace(child);

    let handle = app.clone();
    std::thread::spawn(move || {
        let mut opened = false;
        // Dibaca sampai habis, bukan berhenti di baris alamat: kalau pipa ini
        // tidak dikosongkan, engine akan macet begitu penyangganya penuh.
        for line in BufReader::new(stdout).lines().map_while(Result::ok) {
            if !opened {
                if let Some(url) = line.strip_prefix(URL_PREFIX) {
                    opened = true;
                    open_window(&handle, url.trim());
                    continue;
                }
            }
            println!("{line}");
        }
    });

    Ok(())
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

fn main() {
    let app = tauri::Builder::default()
        .manage(Engine(Mutex::new(None)))
        .setup(|app| {
            if let Err(e) = start_engine(app.handle()) {
                // Jendela tetap dibuka: halaman awalnya yang menjelaskan apa
                // yang salah, jauh lebih berguna daripada aplikasi yang hilang
                // tanpa sepatah kata.
                eprintln!("{e}");
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
