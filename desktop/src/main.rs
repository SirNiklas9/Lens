use std::env;
use tauri::State;

struct AppState {
    api_url: String,
}

fn local_api_url(value: &str) -> Result<String, String> {
    let trimmed = value.trim().trim_end_matches('/');
    let valid = ["http://127.0.0.1", "http://localhost", "http://[::1]"]
        .iter()
        .any(|prefix| trimmed == *prefix || trimmed.starts_with(&format!("{prefix}:")));
    if !valid {
        return Err("Inspector Desktop only accepts a loopback HTTP API URL".into());
    }
    Ok(trimmed.to_string())
}

fn configured_api_url() -> Result<String, String> {
    let mut args = env::args().skip(1);
    while let Some(arg) = args.next() {
        if arg == "--api" {
            return local_api_url(&args.next().ok_or("--api requires a URL")?);
        }
    }
    local_api_url(&env::var("INSPECTOR_API_URL").unwrap_or_else(|_| "http://127.0.0.1:7341".into()))
}

#[tauri::command]
fn api_url(state: State<'_, AppState>) -> String {
    state.api_url.clone()
}

fn main() {
    let configured_url = match configured_api_url() {
        Ok(value) => value,
        Err(error) => {
            eprintln!("Inspector Desktop: {error}");
            std::process::exit(2);
        }
    };
    tauri::Builder::default()
        .manage(AppState {
            api_url: configured_url,
        })
        .invoke_handler(tauri::generate_handler![api_url])
        .run(tauri::generate_context!())
        .expect("Inspector Desktop failed to run");
}
