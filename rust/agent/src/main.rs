use std::time::Duration;

use tokio::sync::mpsc;

use herminas_agent::{backpressure::Backpressure, buffer::wal::Wal, collect, config, parse, ship, AgentError};

#[tokio::main]
async fn main() -> Result<(), AgentError> {
    let config_path = std::env::var("HERMINAS_AGENT_CONFIG").unwrap_or_else(|_| "agent.yaml".to_string());
    let cfg = config::load(&config_path)?;

    let mut wal = Wal::open(&cfg.wal_path)?;
    println!(
        "herminas-agent: WAL opened at {:?} ({} bytes pending from a previous run, if any)",
        cfg.wal_path,
        wal.pending_bytes()
    );

    let (tx, mut rx) = mpsc::channel::<collect::http::Line>(1024);

    // HTTP ingestion endpoint (F-103).
    let http_addr: std::net::SocketAddr = cfg
        .http_addr
        .parse()
        .map_err(|e| AgentError::Config(format!("invalid http_addr {:?}: {e}", cfg.http_addr)))?;
    let http_tx = tx.clone();
    tokio::spawn(async move {
        if let Err(e) = collect::http::serve(http_addr, http_tx).await {
            eprintln!("herminas-agent: http collector stopped: {e}");
        }
    });
    println!("herminas-agent: HTTP ingestion listening on {http_addr}");

    // File tailers (F-102): one polling task per configured source.
    for source in cfg.sources.clone() {
        let tx = tx.clone();
        let format = source.format.clone();
        tokio::spawn(async move {
            let mut tailer = match collect::file::FileTailer::new(&source.path, source.multiline_start.as_deref()) {
                Ok(t) => t,
                Err(e) => {
                    eprintln!("herminas-agent: cannot tail {:?}: {e}", source.path);
                    return;
                }
            };
            // A multiline record only closes once a *following* start-line
            // arrives — otherwise it would sit in FileTailer forever if the
            // file goes idle right after it. After a few empty polls in a
            // row, flush whatever's pending instead of waiting indefinitely.
            let mut idle_polls = 0u32;
            const IDLE_FLUSH_AFTER: u32 = 4; // ~2s at the 500ms poll interval below

            loop {
                match tailer.poll() {
                    Ok(lines) if lines.is_empty() => {
                        idle_polls += 1;
                        if idle_polls >= IDLE_FLUSH_AFTER {
                            if let Some(record) = tailer.flush_pending() {
                                let _ = tx.send((format.clone(), record)).await;
                            }
                            idle_polls = 0;
                        }
                    }
                    Ok(lines) => {
                        idle_polls = 0;
                        for line in lines {
                            let _ = tx.send((format.clone(), line)).await;
                        }
                    }
                    Err(e) => eprintln!("herminas-agent: tail error on {:?}: {e}", source.path),
                }
                tokio::time::sleep(Duration::from_millis(500)).await;
            }
        });
    }
    drop(tx); // the loop below only relies on rx.recv() returning None once every sender is dropped

    // Manual config reload on SIGHUP (M1.1: "rechargement à la demande").
    // Remote reload pushed from the server needs agent.proto's
    // GetAgentConfig RPC (M0.3) and fleet management (M8.3) — not wired
    // yet, so this only reloads and logs; it does not yet restart
    // collectors with the new source list.
    #[cfg(unix)]
    {
        let reload_path = config_path.clone();
        tokio::spawn(async move {
            let mut sighup = match tokio::signal::unix::signal(tokio::signal::unix::SignalKind::hangup()) {
                Ok(s) => s,
                Err(e) => {
                    eprintln!("herminas-agent: cannot install SIGHUP handler: {e}");
                    return;
                }
            };
            loop {
                sighup.recv().await;
                match config::load(&reload_path) {
                    Ok(_) => println!(
                        "herminas-agent: config reloaded from {reload_path} (collectors are not yet restarted dynamically — full hot-reload lands with M8.3)"
                    ),
                    Err(e) => eprintln!("herminas-agent: config reload failed: {e}"),
                }
            }
        });
    }

    let backpressure = Backpressure::new(cfg.backpressure_threshold_bytes);
    let shipper = ship::FileShipper::new(cfg.ship_output_path.clone());
    let mut flush_tick = tokio::time::interval(Duration::from_millis(cfg.flush_interval_ms));

    loop {
        tokio::select! {
            received = rx.recv() => {
                let Some((format, raw_line)) = received else {
                    // All senders dropped (shouldn't normally happen while
                    // the http/file tasks are alive) — keep flushing on a
                    // timer instead of exiting.
                    tokio::time::sleep(Duration::from_secs(1)).await;
                    continue;
                };

                match parse::parse(&raw_line, &format) {
                    Ok(record) => {
                        let encoded = serde_json::to_vec(&record).unwrap_or_default();
                        if let Err(e) = wal.append(&encoded) {
                            eprintln!("herminas-agent: WAL append failed: {e}");
                        }
                    }
                    Err(e) => eprintln!("herminas-agent: dropping unparsable record: {e}"),
                }

                let delay = backpressure.delay_for(wal.pending_bytes());
                if delay > Duration::ZERO {
                    tokio::time::sleep(delay).await;
                }
            }
            _ = flush_tick.tick() => {
                if let Err(e) = drain_and_ship(&mut wal, &shipper, cfg.batch_size) {
                    eprintln!("herminas-agent: ship failed, will retry next tick: {e}");
                }
            }
        }
    }
}

fn drain_and_ship(wal: &mut Wal, shipper: &dyn ship::Shipper, batch_size: usize) -> Result<(), AgentError> {
    let pending = wal.pending()?;
    if pending.is_empty() {
        return Ok(());
    }
    for chunk in pending.chunks(batch_size.max(1)) {
        shipper.ship(chunk)?;
        wal.ack(chunk.last().unwrap().offset)?;
    }
    Ok(())
}
