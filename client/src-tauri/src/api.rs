use serde::{Deserialize, Serialize};
use std::collections::HashMap;

// --- DTOs matching the server JSON (snake_case both sides) ---

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Settings {
    pub server_url: String,
    #[serde(default)]
    pub api_token: String,
    #[serde(default = "default_auto_sync")]
    pub auto_sync: bool,
}

fn default_auto_sync() -> bool {
    true
}

impl Default for Settings {
    fn default() -> Self {
        Self {
            server_url: "http://localhost:8090".into(),
            api_token: String::new(),
            auto_sync: true,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Game {
    pub id: String,
    pub name: String,
    #[serde(default)]
    pub exe_pattern: String,
    #[serde(default)]
    pub save_dir_hint: String,
    #[serde(default)]
    pub notes: String,
    #[serde(default)]
    pub created_at: i64,
    #[serde(default)]
    pub updated_at: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SaveMeta {
    pub game_id: String,
    pub filename: String,
    pub sha256: String,
    pub size: i64,
    pub updated_at: i64,
}

#[derive(Debug, Clone, Serialize, serde::Deserialize)]
pub struct SaveStatus {
    pub filename: String,
    pub state: String, // synced | changed | local_only | remote_only
    #[serde(skip_serializing_if = "Option::is_none")]
    pub local_size: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub local_mtime: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub local_sha256: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub remote_size: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub remote_sha256: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub remote_updated_at: Option<i64>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct RunningMatch {
    pub pid: u32,
    pub name: String,
    pub exe_stem: String,
    pub game_id: Option<String>,
    pub game_name: Option<String>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ProcessEntry {
    pub name: String,
    pub count: u32,
}

/// Per-file known remote state, kept in memory (refreshed from server).
#[derive(Debug, Clone)]
pub struct KnownRemote {
    pub sha256: String,
    pub size: i64,
    pub updated_at: i64,
}

// --- HTTP client wrapper ---

pub struct Api {
    client: reqwest::Client,
    base_url: String, // includes /api
    token: Option<String>,
}

impl Api {
    pub fn from_settings(s: &Settings) -> Result<Self, String> {
        let url = s.server_url.trim().trim_end_matches('/');
        if url.is_empty() {
            return Err("server URL is empty".into());
        }
        let client = reqwest::Client::builder()
            .timeout(std::time::Duration::from_secs(120))
            .build()
            .map_err(|e| e.to_string())?;
        Ok(Self {
            client,
            base_url: format!("{url}/api"),
            token: if s.api_token.is_empty() { None } else { Some(s.api_token.clone()) },
        })
    }

    fn req(&self, method: reqwest::Method, path: &str) -> reqwest::RequestBuilder {
        let mut req = self.client.request(method, format!("{}{}", self.base_url, path));
        if let Some(token) = &self.token {
            req = req.header("Authorization", format!("Bearer {token}"));
        }
        req
    }

    pub async fn health(&self) -> Result<bool, String> {
        match self.req(reqwest::Method::GET, "/health").send().await {
            Ok(resp) => Ok(resp.status().is_success()),
            Err(_) => Ok(false),
        }
    }

    pub async fn list_games(&self) -> Result<Vec<Game>, String> {
        let resp = self.req(reqwest::Method::GET, "/games").send().await.map_err(|e| e.to_string())?;
        if !resp.status().is_success() {
            return Err(format!("server responded with {}", resp.status()));
        }
        Ok(resp.json::<Vec<Game>>().await.map_err(|e| e.to_string())?)
    }

    pub async fn create_game(&self, input: &CreateGameInput) -> Result<Game, String> {
        let resp = self
            .req(reqwest::Method::POST, "/games")
            .json(input)
            .send()
            .await
            .map_err(|e| e.to_string())?;
        let status = resp.status();
        if !status.is_success() {
            return Err(read_error(resp).await.unwrap_or_else(|| format!("server responded with {status}")));
        }
        Ok(resp.json().await.map_err(|e| e.to_string())?)
    }

    pub async fn update_game(&self, id: &str, patch: &GamePatch) -> Result<Game, String> {
        let resp = self
            .req(reqwest::Method::PATCH, &format!("/games/{id}"))
            .json(patch)
            .send()
            .await
            .map_err(|e| e.to_string())?;
        let status = resp.status();
        if !status.is_success() {
            return Err(read_error(resp).await.unwrap_or_else(|| format!("server responded with {status}")));
        }
        Ok(resp.json().await.map_err(|e| e.to_string())?)
    }

    pub async fn delete_game(&self, id: &str) -> Result<(), String> {
        let resp = self
            .req(reqwest::Method::DELETE, &format!("/games/{id}"))
            .send()
            .await
            .map_err(|e| e.to_string())?;
        let status = resp.status();
        if !status.is_success() {
            return Err(read_error(resp).await.unwrap_or_else(|| format!("server responded with {status}")));
        }
        Ok(())
    }

    pub async fn list_saves(&self, game_id: &str) -> Result<Vec<SaveMeta>, String> {
        let resp = self
            .req(reqwest::Method::GET, &format!("/games/{game_id}/saves"))
            .send()
            .await
            .map_err(|e| e.to_string())?;
        if !resp.status().is_success() {
            return Err(format!("server responded with {}", resp.status()));
        }
        Ok(resp.json().await.map_err(|e| e.to_string())?)
    }

    /// Upload a file's bytes, returns the server-side SaveMeta.
    pub async fn upload_save(&self, game_id: &str, filename: &str, data: Vec<u8>) -> Result<SaveMeta, String> {
        let encoded = url_encode(filename);
        let resp = self
            .req(reqwest::Method::PUT, &format!("/games/{game_id}/saves/{encoded}"))
            .header("Content-Type", "application/octet-stream")
            .body(data)
            .send()
            .await
            .map_err(|e| e.to_string())?;
        let status = resp.status();
        if !status.is_success() {
            return Err(read_error(resp).await.unwrap_or_else(|| format!("server responded with {status}")));
        }
        Ok(resp.json().await.map_err(|e| e.to_string())?)
    }

    pub async fn download_save(&self, game_id: &str, filename: &str) -> Result<Vec<u8>, String> {
        let encoded = url_encode(filename);
        let resp = self
            .req(reqwest::Method::GET, &format!("/games/{game_id}/saves/{encoded}"))
            .send()
            .await
            .map_err(|e| e.to_string())?;
        if !resp.status().is_success() {
            return Err(format!("server responded with {}", resp.status()));
        }
        Ok(resp.bytes().await.map_err(|e| e.to_string())?.to_vec())
    }

    /// Convenience: remote state map for a game, empty when offline.
    pub async fn remote_state(&self, game_id: &str) -> Result<HashMap<String, KnownRemote>, String> {
        let saves = self.list_saves(game_id).await?;
        Ok(saves
            .into_iter()
            .map(|s| {
                (
                    s.filename.clone(),
                    KnownRemote {
                        sha256: s.sha256,
                        size: s.size,
                        updated_at: s.updated_at,
                    },
                )
            })
            .collect())
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CreateGameInput {
    pub name: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub exe_pattern: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub save_dir_hint: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub notes: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GamePatch {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub exe_pattern: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub save_dir_hint: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub notes: Option<String>,
}

async fn read_error(resp: reqwest::Response) -> Option<String> {
    let body = resp.text().await.ok()?;
    serde_json::from_str::<serde_json::Value>(&body)
        .ok()
        .and_then(|v| v.get("error")?.as_str().map(str::to_string))
}

/// Minimal percent-encoding for filenames inside a URL path segment.
fn url_encode(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    for byte in s.bytes() {
        match byte {
            b'A'..=b'Z' | b'a'..=b'z' | b'0'..=b'9' | b'-' | b'_' | b'.' | b'~' => out.push(byte as char),
            b'/' => out.push('/'), // subfolders keep their separator
            _ => out.push_str(&format!("%{byte:02X}")),
        }
    }
    out
}
