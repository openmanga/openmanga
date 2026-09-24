use std::{path::PathBuf, process::Command, sync::Mutex, time::Duration};

use notify::{RecursiveMode, Watcher};
use serde_json::{json, Value};
use tauri::Emitter;

/// The sidecar sits next to the app binary (Tauri copies `externalBin` there without the triple).
fn sb_path() -> PathBuf {
    std::env::current_exe().unwrap().with_file_name("sb")
}

/// Runs `sb <args> --json` and returns its one JSON object, or `{code, message}` as the error.
#[tauri::command]
async fn sb(args: Vec<String>, stdin: Option<String>, cwd: Option<String>) -> Result<Value, Value> {
    tauri::async_runtime::spawn_blocking(move || {
        use std::io::Write;
        let mut cmd = Command::new(sb_path());
        cmd.args(&args).arg("--json");
        if let Some(dir) = cwd {
            cmd.current_dir(dir);
        }
        cmd.stdin(std::process::Stdio::piped())
            .stdout(std::process::Stdio::piped())
            .stderr(std::process::Stdio::piped());
        let mut child = cmd.spawn().map_err(|e| json!({"code": "spawn", "message": e.to_string()}))?;
        let input = stdin.unwrap_or_default();
        child.stdin.take().unwrap().write_all(input.as_bytes()).ok();
        let out = child.wait_with_output().map_err(|e| json!({"code": "spawn", "message": e.to_string()}))?;
        let text = String::from_utf8_lossy(&out.stdout);
        let parsed: Value = serde_json::from_str(text.trim()).unwrap_or_else(|_| {
            json!({"error": {"code": "output", "message": format!("{}{}", text, String::from_utf8_lossy(&out.stderr))}})
        });
        match parsed.get("error") {
            Some(err) => Err(err.clone()),
            None if !out.status.success() => Err(json!({"code": "exit", "message": String::from_utf8_lossy(&out.stderr)})),
            None => Ok(parsed),
        }
    })
    .await
    .map_err(|e| json!({"code": "join", "message": e.to_string()}))?
}

struct ProjectWatcher(Mutex<Option<notify::RecommendedWatcher>>);

/// Watches the project folder; emits `project-changed` with the changed paths, batched per 150 ms.
#[tauri::command]
fn watch(app: tauri::AppHandle, state: tauri::State<ProjectWatcher>, dir: Option<String>) -> Result<(), String> {
    let mut slot = state.0.lock().unwrap();
    *slot = None;
    let Some(dir) = dir else { return Ok(()) };
    let (tx, rx) = std::sync::mpsc::channel::<Vec<PathBuf>>();
    let mut watcher = notify::recommended_watcher(move |res: notify::Result<notify::Event>| {
        if let Ok(ev) = res {
            if !ev.kind.is_access() {
                let _ = tx.send(ev.paths);
            }
        }
    })
    .map_err(|e| e.to_string())?;
    watcher.watch(PathBuf::from(&dir).as_path(), RecursiveMode::Recursive).map_err(|e| e.to_string())?;
    std::thread::spawn(move || {
        while let Ok(first) = rx.recv() {
            let mut paths = first;
            while let Ok(more) = rx.recv_timeout(Duration::from_millis(150)) {
                paths.extend(more);
            }
            let list: Vec<String> = paths
                .iter()
                .map(|p| p.to_string_lossy().into_owned())
                .filter(|p| !p.contains("/.history/") && !p.contains(".backup-"))
                .collect();
            if !list.is_empty() {
                let _ = app.emit("project-changed", list);
            }
        }
    });
    *slot = Some(watcher);
    Ok(())
}

/// Dev hook for UI verification: the contents of the file named by `SB_UI_SCRIPT` (unset = none).
#[tauri::command]
fn ui_script() -> Option<String> {
    std::fs::read_to_string(std::env::var("SB_UI_SCRIPT").ok()?).ok()
}

/// Dev hook: prints webview console output to the terminal while `SB_UI_SCRIPT` is set.
#[tauri::command]
fn ui_log(msg: String) {
    if std::env::var("SB_UI_SCRIPT").is_ok() {
        eprintln!("[ui] {msg}");
    }
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_dialog::init())
        .manage(ProjectWatcher(Mutex::new(None)))
        .invoke_handler(tauri::generate_handler![sb, watch, ui_script, ui_log])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
