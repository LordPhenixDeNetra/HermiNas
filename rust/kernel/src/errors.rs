use thiserror::Error;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Code {
    InvalidArgument,
    NotFound,
    Unauthorized,
    Internal,
    Unavailable,
    AlreadyExists,
}

/// HermiNas' standard error type for the data plane: a stable code plus a
/// message and an optional source. The `Code` vocabulary mirrors
/// kernel/errors/errors.go (Go) and herminas_kernel.errors.Code (Python).
#[derive(Debug, Error)]
#[error("[{code:?}] {message}")]
pub struct HerminasError {
    pub code: Code,
    pub message: String,
    #[source]
    pub source: Option<Box<dyn std::error::Error + Send + Sync>>,
}

impl HerminasError {
    pub fn new(code: Code, message: impl Into<String>) -> Self {
        Self {
            code,
            message: message.into(),
            source: None,
        }
    }

    pub fn wrap(
        code: Code,
        message: impl Into<String>,
        source: impl std::error::Error + Send + Sync + 'static,
    ) -> Self {
        Self {
            code,
            message: message.into(),
            source: Some(Box::new(source)),
        }
    }
}
