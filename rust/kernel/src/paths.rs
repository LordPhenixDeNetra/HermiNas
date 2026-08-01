use std::env;
use std::path::{Path, PathBuf};

/// On-disk layout resolved relative to a single root, so the bundle stays
/// relocatable (leçon aNtaerus). Mirrors kernel/paths/paths.go and
/// herminas_kernel.paths (Python), and the bundle structure documented in
/// cahier-des-charges-herminas.md §9.2.
#[derive(Debug, Clone)]
pub struct Layout {
    pub root: PathBuf,
    pub config_dir: PathBuf,
    pub runtime_dir: PathBuf,
    pub models_dir: PathBuf,
    pub data_dir: PathBuf,
    pub logs_dir: PathBuf,
}

impl Layout {
    pub fn resolve(root: impl AsRef<Path>) -> Self {
        let root = root.as_ref().to_path_buf();
        Self {
            config_dir: root.join("config"),
            runtime_dir: root.join("runtime"),
            models_dir: root.join("models"),
            data_dir: root.join("data"),
            logs_dir: root.join("logs"),
            root,
        }
    }

    /// Resolves from HERMINAS_HOME, falling back to the current directory
    /// (dev mode; the bundle launcher always sets HERMINAS_HOME, see M8.1).
    pub fn default_layout() -> Self {
        let root = env::var("HERMINAS_HOME")
            .map(PathBuf::from)
            .unwrap_or_else(|_| env::current_dir().unwrap_or_else(|_| PathBuf::from(".")));
        Self::resolve(root)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn resolve_builds_expected_subpaths() {
        let layout = Layout::resolve("/tmp/herminas");
        assert_eq!(layout.data_dir, PathBuf::from("/tmp/herminas/data"));
        assert_eq!(layout.config_dir, PathBuf::from("/tmp/herminas/config"));
    }
}
