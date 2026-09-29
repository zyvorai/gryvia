use anyhow::{Context, Result};
use kube::api::{Api, ApiResource, GroupVersionKind, ListParams, PostParams};
use kube::core::DynamicObject;
use serde_json::json;
use tokio::time::{sleep, Duration};

use crate::client::GryviaClient;
use crate::commands::flows::verdict_marker;
use crate::display;
use crate::ui::{self, Cell2, Marker};

pub async fn execute(
    client: &GryviaClient,
    service: &str,
    duration: &str,
    level: &str,
    namespace: &str,
    follow: bool,
) -> Result<()> {
    let color = ui::color_enabled();
    for line in trace_header_lines(service, duration, level, namespace, color) {
        println!("{line}");
    }

    // Create trace session via CRD
    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1alpha1",
        "GryviaTraceSession",
    ));
    let api: Api<DynamicObject> = Api::namespaced_with(client.kube_client.clone(), namespace, &ar);

    let session_name = format!("trace-{}-{}", service, chrono::Utc::now().timestamp());
    let trace_obj = serde_json::from_value(json!({
        "apiVersion": "gryvia.io/v1alpha1",
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
                Marker::Ok.paint_with(Marker::Ok.glyph(), color),
                session_name
            );
        }
        Err(e) => {
            display::print_warning(&format!("Could not create trace session: {}", e));
            println!(
                "  {}",
                Marker::Disabled
                    .paint_with("Displaying mock flow data for demonstration...", color)
            );
        }
    }

    println!();
    println!("{}", ui::section("Captured Flows", color));
    println!();

    if follow {
        // Poll for results in follow mode
        loop {
            let flows = fetch_flows(client, namespace, service).await;
            print_flows(&flows, color);

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
                    display::print_info(&format!(
                        "Trace session {} ({})",
                        phase.to_lowercase(),
                        session_name
                    ));
                    break;
                }
            }

            sleep(Duration::from_secs(2)).await;
        }
    } else {
        // One-shot: fetch current flows
        let flows = fetch_flows(client, namespace, service).await;
        if flows.is_empty() {
            println!(
                "{}",
                Marker::Disabled
                    .paint_with("No flows captured yet. Use --follow to stream live.", color)
            );
        } else {
            print_flows(&flows, color);
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

async fn fetch_flows(client: &GryviaClient, namespace: &str, service: &str) -> Vec<FlowEntry> {
    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1alpha1",
        "GryviaFlow",
    ));
    let api: Api<DynamicObject> = Api::namespaced_with(client.kube_client.clone(), namespace, &ar);

    let label_selector = format!("gryvia.io/service={}", service);
    let params = ListParams::default().labels(&label_selector);

    match api.list(&params).await {
        Ok(flows) => flows
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
            .collect(),
        Err(_) => Vec::new(),
    }
}

fn print_flows(flows: &[FlowEntry], color: bool) {
    for line in flow_lines(flows, color) {
        println!("{line}");
    }
}

/// The banner and the trace parameters.
pub fn trace_header_lines(
    service: &str,
    duration: &str,
    level: &str,
    namespace: &str,
    color: bool,
) -> Vec<String> {
    vec![
        ui::header(&format!("Trace: {service}"), color),
        String::new(),
        ui::kv("Service", service, 11, color),
        ui::kv("Duration", duration, 11, color),
        ui::kv("Level", level, 11, color),
        ui::kv("Namespace", namespace, 11, color),
        String::new(),
    ]
}

/// Lines of the captured-flows table.
fn flow_lines(flows: &[FlowEntry], color: bool) -> Vec<String> {
    let rows: Vec<Vec<Cell2>> = flows
        .iter()
        .map(|flow| {
            let marker = verdict_marker(&flow.verdict);
            let verdict = match (marker, flow.policy.as_deref()) {
                (Marker::Error, Some(p)) => format!("{} (policy: {})", flow.verdict, p),
                _ => flow.verdict.clone(),
            };
            vec![
                (flow.timestamp.clone(), None),
                (flow.source.clone(), None),
                (flow.destination.clone(), None),
                (flow.protocol.clone(), None),
                (flow.bytes.clone(), None),
                (flow.latency.clone(), None),
                (verdict, Some(marker)),
            ]
        })
        .collect();
    ui::grid(
        &[
            "TIMESTAMP",
            "SOURCE",
            "DESTINATION",
            "PROTOCOL",
            "BYTES",
            "LATENCY",
            "VERDICT",
        ],
        &rows,
        color,
    )
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::commands::network::strip_ansi;

    fn fixture() -> Vec<FlowEntry> {
        vec![
            FlowEntry {
                timestamp: "10:00:01".into(),
                source: "web".into(),
                destination: "api".into(),
                protocol: "HTTP".into(),
                bytes: "1.2KB".into(),
                latency: "3ms".into(),
                verdict: "FORWARDED".into(),
                policy: None,
            },
            FlowEntry {
                timestamp: "10:00:02".into(),
                source: "web".into(),
                destination: "db".into(),
                protocol: "TCP".into(),
                bytes: "0B".into(),
                latency: "-".into(),
                verdict: "DROP".into(),
                policy: Some("deny-db".into()),
            },
        ]
    }

    #[test]
    fn flows_render_aligned() {
        assert_eq!(
            flow_lines(&fixture(), false),
            vec![
                "TIMESTAMP  SOURCE  DESTINATION  PROTOCOL  BYTES  LATENCY  VERDICT",
                "10:00:01   web     api          HTTP      1.2KB  3ms      FORWARDED",
                "10:00:02   web     db           TCP       0B     -        DROP (policy: deny-db)",
            ]
        );
    }

    #[test]
    fn header_and_color() {
        assert_eq!(
            trace_header_lines("api", "30s", "l4", "prod", false),
            vec![
                "━━━ Trace: api ━━━",
                "",
                "Service    api",
                "Duration   30s",
                "Level      l4",
                "Namespace  prod",
                "",
            ]
        );
        let plain = flow_lines(&fixture(), false);
        let colored = flow_lines(&fixture(), true);
        assert!(colored.iter().any(|l| l.contains('\x1b')));
        let stripped: Vec<String> = colored.iter().map(|l| strip_ansi(l)).collect();
        assert_eq!(stripped, plain);
    }
}
