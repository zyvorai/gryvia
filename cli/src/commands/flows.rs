use anyhow::Result;
use kube::api::{Api, ApiResource, GroupVersionKind, ListParams};
use kube::core::DynamicObject;

use crate::client::GryviaClient;
use crate::display;
use crate::ui::{self, Cell2, Marker};

pub async fn execute(
    client: &GryviaClient,
    service: Option<&str>,
    namespace: &str,
    last: &str,
    output: &str,
) -> Result<()> {
    let color = ui::color_enabled();
    for line in flows_header_lines(service, namespace, last, color) {
        println!("{line}");
    }

    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1alpha1",
        "GryviaFlow",
    ));
    let api: Api<DynamicObject> = Api::namespaced_with(client.kube_client.clone(), namespace, &ar);

    let params = if let Some(svc) = service {
        let label_selector = format!("gryvia.io/service={}", svc);
        ListParams::default().labels(&label_selector)
    } else {
        ListParams::default()
    };

    let flows = match api.list(&params).await {
        Ok(list) => list,
        Err(e) => {
            display::print_warning(&format!("Could not query flow resources: {}", e));
            println!();
            return Ok(());
        }
    };

    if flows.items.is_empty() {
        println!(
            "{}",
            Marker::Disabled.paint_with("No network flows found.", color)
        );
        println!();
        return Ok(());
    }

    match output {
        "json" | "yaml" => {
            crate::output::print_serialized(output, &flows)?;
        }
        _ => {
            print_flows_table(&flows.items);
        }
    }

    Ok(())
}

/// Marker for a flow verdict (`FORWARDED`, `DROP`, ...), case-insensitive.
pub fn verdict_marker(verdict: &str) -> Marker {
    match verdict.to_ascii_uppercase().as_str() {
        "FORWARDED" | "ALLOW" | "ALLOWED" => Marker::Ok,
        "DROP" | "DROPPED" | "DENY" | "DENIED" | "BLOCKED" => Marker::Error,
        _ => Marker::Warn,
    }
}

/// The banner and the query summary above the flows.
pub fn flows_header_lines(
    service: Option<&str>,
    namespace: &str,
    last: &str,
    color: bool,
) -> Vec<String> {
    let mut lines = vec![ui::header("Network Flows", color), String::new()];
    if let Some(svc) = service {
        lines.push(ui::kv("Service", svc, 13, color));
    }
    lines.push(ui::kv("Namespace", namespace, 13, color));
    lines.push(ui::kv("Time window", last, 13, color));
    lines.push(String::new());
    lines
}

/// Lines of the flows table.
pub fn flows_lines(flows: &[DynamicObject], color: bool) -> Vec<String> {
    let text = |spec: Option<&serde_json::Value>, key: &str, default: &str| -> String {
        spec.and_then(|s| s.get(key))
            .and_then(|v| v.as_str())
            .unwrap_or(default)
            .to_string()
    };
    let rows: Vec<Vec<Cell2>> = flows
        .iter()
        .map(|flow| {
            let spec = flow.data.get("spec");
            let port = spec
                .and_then(|s| s.get("port"))
                .and_then(|v| v.as_u64())
                .map(|p| p.to_string())
                .unwrap_or_else(|| "-".to_string());
            let verdict = text(spec, "verdict", "FORWARDED");
            let marker = verdict_marker(&verdict);
            vec![
                (text(spec, "timestamp", "-"), None),
                (text(spec, "source", "-"), None),
                (text(spec, "destination", "-"), None),
                (text(spec, "protocol", "TCP"), None),
                (port, None),
                (text(spec, "bytes", "0B"), None),
                (text(spec, "latency", "-"), None),
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
            "PORT",
            "BYTES",
            "LATENCY",
            "VERDICT",
        ],
        &rows,
        color,
    )
}

fn print_flows_table(flows: &[DynamicObject]) {
    for line in flows_lines(flows, ui::color_enabled()) {
        println!("{line}");
    }
    println!();
    display::print_info(&format!("Total flows: {}", flows.len()));
    println!();
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::commands::network::strip_ansi;
    use serde_json::json;

    fn flow(spec: serde_json::Value) -> DynamicObject {
        serde_json::from_value(json!({
            "apiVersion": "gryvia.io/v1alpha1",
            "kind": "GryviaFlow",
            "metadata": {"name": "f"},
            "spec": spec,
        }))
        .unwrap()
    }

    fn fixture() -> Vec<DynamicObject> {
        vec![
            flow(
                json!({"timestamp": "10:00:01", "source": "web", "destination": "api",
                "protocol": "HTTP", "port": 8080, "bytes": "1.2KB", "latency": "3ms",
                "verdict": "FORWARDED"}),
            ),
            flow(json!({"source": "web", "destination": "db", "verdict": "DROP"})),
        ]
    }

    #[test]
    fn flows_table_is_aligned() {
        assert_eq!(
            flows_lines(&fixture(), false),
            vec![
                "TIMESTAMP  SOURCE  DESTINATION  PROTOCOL  PORT  BYTES  LATENCY  VERDICT",
                "10:00:01   web     api          HTTP      8080  1.2KB  3ms      FORWARDED",
                "-          web     db           TCP       -     0B     -        DROP",
            ]
        );
    }

    #[test]
    fn flows_header_and_color() {
        assert_eq!(
            flows_header_lines(Some("api"), "prod", "5m", false),
            vec![
                "━━━ Network Flows ━━━",
                "",
                "Service      api",
                "Namespace    prod",
                "Time window  5m",
                "",
            ]
        );
        let plain = flows_lines(&fixture(), false);
        let colored = flows_lines(&fixture(), true);
        assert!(colored.iter().any(|l| l.contains('\x1b')));
        let stripped: Vec<String> = colored.iter().map(|l| strip_ansi(l)).collect();
        assert_eq!(stripped, plain);
    }

    #[test]
    fn verdicts() {
        assert_eq!(verdict_marker("FORWARDED"), Marker::Ok);
        assert_eq!(verdict_marker("drop"), Marker::Error);
        assert_eq!(verdict_marker("AUDIT"), Marker::Warn);
    }
}
