use std::fs;
use std::path::Path;

use secrecy::SecretString;
use serde::Deserialize;

use crate::errors::{Code, HerminasError};

#[derive(Debug, Deserialize)]
struct RawSettings {
    environment: String,
    http: RawPort,
    grpc: RawPort,
    data_dir: String,
    llm: RawLlm,
}

#[derive(Debug, Deserialize)]
struct RawPort {
    port: u16,
}

#[derive(Debug, Deserialize)]
struct RawLlm {
    provider: String,
    #[serde(default)]
    api_key: String,
}

/// HermiNas' immutable, boot-time configuration for the Rust data plane.
/// Loaded once via `Settings::load`; fields are private with read-only
/// accessors, so nothing can mutate configuration at runtime (leçon
/// aNtaerus: config immuable, 1 seule source).
pub struct Settings {
    environment: String,
    http_port: u16,
    grpc_port: u16,
    data_dir: String,
    llm_provider: String,
    llm_api_key: SecretString,
}

impl Settings {
    pub fn load(path: impl AsRef<Path>) -> Result<Self, HerminasError> {
        let raw_text = fs::read_to_string(&path).map_err(|e| {
            HerminasError::wrap(
                Code::NotFound,
                format!("cannot read settings file {:?}", path.as_ref()),
                e,
            )
        })?;

        let raw: RawSettings = serde_yaml::from_str(&raw_text)
            .map_err(|e| HerminasError::wrap(Code::InvalidArgument, "invalid settings YAML", e))?;

        let api_key = std::env::var("HERMINAS_LLM_API_KEY").unwrap_or(raw.llm.api_key);

        Ok(Settings {
            environment: raw.environment,
            http_port: raw.http.port,
            grpc_port: raw.grpc.port,
            data_dir: raw.data_dir,
            llm_provider: raw.llm.provider,
            llm_api_key: SecretString::new(api_key.into()),
        })
    }

    pub fn environment(&self) -> &str {
        &self.environment
    }
    pub fn http_port(&self) -> u16 {
        self.http_port
    }
    pub fn grpc_port(&self) -> u16 {
        self.grpc_port
    }
    pub fn data_dir(&self) -> &str {
        &self.data_dir
    }
    pub fn llm_provider(&self) -> &str {
        &self.llm_provider
    }
    pub fn llm_api_key(&self) -> &SecretString {
        &self.llm_api_key
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use secrecy::ExposeSecret;

    /// Rust half of the M0.4 anti-leak test (test_secrets_no_leak): the
    /// Debug output of a SecretString must never contain the raw value.
    #[test]
    fn secret_string_debug_does_not_leak() {
        let fake = "sk-supersecrettoken1234567890".to_string();
        let secret = SecretString::new(fake.clone().into());

        let debug_output = format!("{:?}", secret);
        assert!(!debug_output.contains(&fake));
        assert_eq!(secret.expose_secret(), &fake);
    }
}
