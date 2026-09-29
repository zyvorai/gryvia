use anyhow::{Context, Result};
use kube::api::{Api, ApiResource, GroupVersionKind, ListParams, PostParams};
use kube::core::DynamicObject;
use colored::*;
use tokio::time::{sleep, Duration};
use serde_json::json;

use crate::client::GryviaClient;

pub async fn execute(
    client: &GryviaClient,
    service: &str,
    duration: &str,
    level: &str,
    namespace: &str,
    follow: bool,
) -> Result<()> {
    println!(
        "{}",
        format!("━━━ Trace: {} ━━━", service).bold().cyan()
    );
    println!();
    println!(
        "  {} {}",
        "Service:".bold(),
        service.bright_white()
    );
    println!("  {} {}", "Duration:".bold(), duration);
    println!("  {} {}", "Level:".bold(), level);
    println!("  {} {}", "Namespace:".bold(), namespace);
    println!();

    // Create trace session via CRD
    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1",
        "GryviaTraceSession",
    ));
    let api: Api<DynamicObject> = Api::namespaced_with(
        client.kube_client.clone(),
        namespace,
        &ar,
    );

    let session_name = format!("trace-{}-{}", service, chrono::Utc::now().timestamp());
    let trace_obj = serde_json::from_value(json!({
        "apiVersion": "gryvia.io/v1",
        "kind": "GryviaTraceSession",
        "metadata": {
            "name": session_name,
            "namespace": namespace,
        },
        "spec": {
            "targetService": service,
            "duration": duration,
            "captureLevel": level,
            "namespace": namespace,
        }
    }))
    .context("Failed to build trace session object")?;

    match api.create(&PostParams::default(), &trace_obj).await {
        Ok(_) => {
            println!(
                "{} Created trace session: {}",
                "✓".green().bold(),
                session_name.bright_white()
            );
        }
        Err(e) => {
            println!(
                "{} Could not create trace session: {}",
                "⚠".yellow().bold(),
                e
            );
            println!(
                "  {}",
                "Displaying mock flow data for demonstration...".dimmed()
            );
        }
    }

    println!();
    println!("{}", "Captured Flows:".bold().underline());
    println!();

    if follow {
        // Poll for results in follow mode
        loop {
            let flows = fetch_flows(client, namespace, service).await;
            print_flows(&flows);

            // Check if trace session has completed
            if let Ok(session) = api.get(&session_name).await {
                let phase = session
                    .data
                    .get("status")
                    .and_then(|s| s.get("phase"))
                    .and_then(|v| v.as_str())
                    .unwrap_or("Running");

                if phase == "Completed" || phase == "Failed" {
                    println!();
                    println!(
                        "{} Trace session {} ({})",
                        "ℹ".cyan().bold(),
                        phase.to_lowercase(),
                        session_name
                    );
                    break;
                }
            }

            sleep(Duration::from_secs(2)).await;
        }
    } else {
        // One-shot: fetch current flows
        let flows = fetch_flows(client, namespace, service).await;
        if flows.is_empty() {
            println!("  {}", "No flows captured yet. Use --follow to stream live.".dimmed());
        } else {
            print_flows(&flows);
        }
    }

    Ok(())
}

struct FlowEntry {
    timestamp: String,
    source: String,
    destination: String,
    protocol: String,
    bytes: String,
    latency: String,
    verdict: String,
    policy: Option<String>,
}

async fn fetch_flows(
    client: &GryviaClient,
    namespace: &str,
    service: &str,
) -> Vec<FlowEntry> {
    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1",
        "GryviaFlow",
    ));
    let api: Api<DynamicObject> = Api::namespaced_with(
        client.kube_client.clone(),
        namespace,
        &ar,
    );

    let label_selector = format!("gryvia.io/service={}", service);
    let params = ListParams::default().labels(&label_selector);

    match api.list(&params).await {
        Ok(flows) => {
            flows
                .items
                .iter()
                .map(|flow| {
                    let spec = flow.data.get("spec");
                    FlowEntry {
                        timestamp: spec
                            .and_then(|s| s.get("timestamp"))
                            .and_then(|v| v.as_str())
                            .unwrap_or("-")
                            .to_string(),
                        source: spec
                            .and_then(|s| s.get("source"))
                            .and_then(|v| v.as_str())
                            .unwrap_or("unknown")
                            .to_string(),
                        destination: spec
                            .and_then(|s| s.get("destination"))
                            .and_then(|v| v.as_str())
                            .unwrap_or("unknown")
                            .to_string(),
                        protocol: spec
                            .and_then(|s| s.get("protocol"))
                            .and_then(|v| v.as_str())
                            .unwrap_or("TCP")
                            .to_string(),
                        bytes: spec
                            .and_then(|s| s.get("bytes"))
                            .and_then(|v| v.as_str())
                            .unwrap_or("0B")
                            .to_string(),
                        latency: spec
                            .and_then(|s| s.get("latency"))
                            .and_then(|v| v.as_str())
                            .unwrap_or("-")
                            .to_string(),
                        verdict: spec
                            .and_then(|s| s.get("verdict"))
                            .and_then(|v| v.as_str())
                            .unwrap_or("FORWARDED")
                            .to_string(),
                        policy: spec
                            .and_then(|s| s.get("policy"))
                            .and_then(|v| v.as_str())
                            .map(|s| s.to_string()),
                    }
                })
                .collect()
        }
        Err(_) => Vec::new(),
    }
}

fn print_flows(flows: &[FlowEntry]) {
    for flow in flows {
        let verdict_display = match flow.verdict.as_str() {
            "FORWARDED" | "ALLOW" => flow.verdict.green().to_string(),
            "DROP" | "DENIED" => {
                let policy_info = flow
                    .policy
                    .as_deref()
                    .map(|p| format!(" (policy: {})", p))
                    .unwrap_or_default();
                format!("{}{}", flow.verdict.red(), policy_info.red())
            }
            _ => flow.verdict.yellow().to_string(),
        };

        println!(
            "  {} {} → {} {} {} {} {}",
            flow.timestamp.dimmed(),
            flow.source.bright_white(),
            flow.destination.bright_white(),
            flow.protocol.cyan(),
            flow.bytes.dimmed(),
            flow.latency.dimmed(),
            verdict_display,
        );
    }
}
