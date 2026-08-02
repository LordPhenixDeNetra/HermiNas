//! Local HTTP/JSON ingestion endpoint (F-103): accepts one JSON object per
//! request, or newline-delimited JSON objects in a single body, and pushes
//! each raw line onto the same channel the file tailers feed. Always tags
//! lines with `Format::Json` — that's this endpoint's whole contract.

use std::net::SocketAddr;

use axum::extract::State;
use axum::routing::{get, post};
use axum::Router;
use tokio::sync::mpsc;

use crate::parse::Format;

pub type Line = (Format, String);

#[derive(Clone)]
struct HttpState {
    tx: mpsc::Sender<Line>,
}

pub fn build_router(tx: mpsc::Sender<Line>) -> Router {
    Router::new()
        .route("/healthz", get(|| async { "OK" }))
        .route("/ingest", post(ingest))
        .with_state(HttpState { tx })
}

pub async fn serve(addr: SocketAddr, tx: mpsc::Sender<Line>) -> Result<(), crate::AgentError> {
    let listener = tokio::net::TcpListener::bind(addr).await?;
    axum::serve(listener, build_router(tx))
        .await
        .map_err(|e| crate::AgentError::Io(std::io::Error::other(e)))
}

async fn ingest(State(state): State<HttpState>, body: String) -> &'static str {
    for line in body.lines().filter(|l| !l.trim().is_empty()) {
        let _ = state.tx.send((Format::Json, line.to_string())).await;
    }
    "accepted"
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn ingest_forwards_each_line_tagged_as_json() {
        let (tx, mut rx) = mpsc::channel(10);
        let state = HttpState { tx };

        let body = "{\"a\":1}\n{\"b\":2}".to_string();
        let resp = ingest(State(state), body).await;
        assert_eq!(resp, "accepted");

        let (fmt1, line1) = rx.recv().await.unwrap();
        let (fmt2, line2) = rx.recv().await.unwrap();
        assert!(matches!(fmt1, Format::Json));
        assert!(matches!(fmt2, Format::Json));
        assert_eq!(line1, "{\"a\":1}");
        assert_eq!(line2, "{\"b\":2}");
    }

    #[tokio::test]
    async fn ingest_skips_blank_lines() {
        let (tx, mut rx) = mpsc::channel(10);
        let state = HttpState { tx };

        ingest(State(state), "{\"a\":1}\n\n\n".to_string()).await;

        let (_, line) = rx.recv().await.unwrap();
        assert_eq!(line, "{\"a\":1}");
        assert!(rx.try_recv().is_err(), "no second message expected");
    }

    #[tokio::test]
    async fn serve_accepts_a_real_http_post() {
        let (tx, mut rx) = mpsc::channel(10);
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();

        tokio::spawn(async move {
            let _ = axum::serve(listener, build_router(tx)).await;
        });

        let client = reqwest::Client::new();
        let resp = client
            .post(format!("http://{addr}/ingest"))
            .body("{\"x\":1}")
            .send()
            .await
            .unwrap();
        assert!(resp.status().is_success());

        let (_, received) = tokio::time::timeout(std::time::Duration::from_secs(2), rx.recv())
            .await
            .expect("timed out waiting for ingested line")
            .expect("channel closed");
        assert_eq!(received, "{\"x\":1}");
    }
}
