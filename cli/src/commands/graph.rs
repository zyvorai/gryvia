use anyhow::Result;
use kube::api::{Api, ApiResource, GroupVersionKind, ListParams};
use kube::core::DynamicObject;
use std::collections::{HashMap, HashSet};

use crate::client::GryviaClient;
use crate::commands::flows::verdict_marker;
use crate::display;
use crate::ui::{self, Marker};

pub async fn execute(client: &GryviaClient, namespace: &str, format: &str) -> Result<()> {
    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1alpha1",
        "GryviaFlow",
    ));
    let api: Api<DynamicObject> = Api::namespaced_with(client.kube_client.clone(), namespace, &ar);

    let flows = match api.list(&ListParams::default()).await {
        Ok(list) => list,
        Err(e) => {
            display::print_warning(&format!("Could not query flow resources: {}", e));
            println!("  Showing empty graph.");
            println!();
            return Ok(());
        }
    };

    // Build graph from flows
    let mut services: HashSet<String> = HashSet::new();
    let mut edges: Vec<Edge> = Vec::new();

    for flow in &flows.items {
        let spec = flow.data.get("spec");
        let source = spec
            .and_then(|s| s.get("source"))
            .and_then(|v| v.as_str())
            .unwrap_or("unknown")
            .to_string();
        let destination = spec
            .and_then(|s| s.get("destination"))
            .and_then(|v| v.as_str())
            .unwrap_or("unknown")
            .to_string();
        let latency = spec
            .and_then(|s| s.get("latency"))
            .and_then(|v| v.as_str())
            .unwrap_or("-")
            .to_string();
        let protocol = spec
            .and_then(|s| s.get("protocol"))
            .and_then(|v| v.as_str())
            .unwrap_or("TCP")
            .to_string();
        let verdict = spec
            .and_then(|s| s.get("verdict"))
            .and_then(|v| v.as_str())
            .unwrap_or("FORWARDED")
            .to_string();

        services.insert(source.clone());
        services.insert(destination.clone());

        edges.push(Edge {
            source,
            destination,
            latency,
            protocol,
            verdict,
        });
    }

    match format {
        "json" => {
            print_json_graph(&services, &edges)?;
        }
        _ => {
            for line in ascii_graph_lines(&services, &edges, ui::color_enabled()) {
                println!("{line}");
            }
        }
    }

    Ok(())
}

struct Edge {
    source: String,
    destination: String,
    latency: String,
    protocol: String,
    verdict: String,
}

/// Lines of the ascii service dependency graph.
fn ascii_graph_lines(services: &HashSet<String>, edges: &[Edge], color: bool) -> Vec<String> {
    let mut lines = vec![ui::header("Service Dependency Graph", color), String::new()];

    if services.is_empty() {
        lines.push(Marker::Disabled.paint_with("No services or flows found.", color));
        lines.push(String::new());
        return lines;
    }

    // Group edges by source
    let mut adjacency: HashMap<&str, Vec<&Edge>> = HashMap::new();
    for edge in edges {
        adjacency.entry(&edge.source).or_default().push(edge);
    }

    let mut sorted_sources: Vec<&str> = edges
        .iter()
        .map(|e| e.source.as_str())
        .collect::<HashSet<&str>>()
        .into_iter()
        .collect();
    sorted_sources.sort();

    let mut printed: HashSet<&str> = HashSet::new();
    for source in &sorted_sources {
        if printed.contains(source) {
            continue;
        }
        node_lines(source, &adjacency, &mut printed, color, &mut lines);
    }

    // Services that only appear as destinations (leaf nodes)
    let mut remaining: Vec<&String> = services
        .iter()
        .filter(|s| !printed.contains(s.as_str()))
        .collect();
    remaining.sort();

    for svc in remaining {
        lines.push(format!("  {}", ui::ansi(&format!("[{svc}]"), "1", color)));
        lines.push(format!(
            "    {}",
            Marker::Disabled.paint_with("(leaf) (no outgoing connections)", color)
        ));
        lines.push(String::new());
    }

    lines.push(format!(
        "{} {} services, {} connections",
        ui::ansi("ℹ", "1;36", color),
        services.len(),
        edges.len()
    ));
    lines.push(String::new());
    lines
}

fn node_lines<'a>(
    service: &'a str,
    adjacency: &HashMap<&'a str, Vec<&Edge>>,
    printed: &mut HashSet<&'a str>,
    color: bool,
    lines: &mut Vec<String>,
) {
    if printed.contains(service) {
        lines.push(format!(
            "  {} (circular ref)",
            Marker::Disabled.paint_with(&format!("[{service}]"), color)
        ));
        return;
    }
    printed.insert(service);
    lines.push(format!(
        "  {}",
        ui::ansi(&format!("[{service}]"), "1", color)
    ));

    if let Some(targets) = adjacency.get(service) {
        for (i, edge) in targets.iter().enumerate() {
            let connector = if i == targets.len() - 1 {
                "└──"
            } else {
                "├──"
            };
            let marker = verdict_marker(&edge.verdict);
            let symbol = match marker {
                Marker::Ok => "→",
                Marker::Error => "✗",
                _ => "?",
            };
            let latency = if edge.latency == "-" {
                String::new()
            } else {
                format!(
                    " {}",
                    Marker::Disabled.paint_with(&format!("({})", edge.latency), color)
                )
            };
            lines.push(format!(
                "    {} {} {} {}{}",
                Marker::Disabled.paint_with(connector, color),
                marker.paint_with(symbol, color),
                edge.destination,
                edge.protocol,
                latency,
            ));
        }
    }
    lines.push(String::new());
}

fn print_json_graph(services: &HashSet<String>, edges: &[Edge]) -> Result<()> {
    let nodes: Vec<serde_json::Value> = services
        .iter()
        .map(|s| {
            serde_json::json!({
                "id": s,
                "label": s,
            })
        })
        .collect();

    let edge_values: Vec<serde_json::Value> = edges
        .iter()
        .map(|e| {
            serde_json::json!({
                "source": e.source,
                "target": e.destination,
                "protocol": e.protocol,
                "latency": e.latency,
                "verdict": e.verdict,
            })
        })
        .collect();

    let graph = serde_json::json!({
        "nodes": nodes,
        "edges": edge_values,
    });

    println!("{}", serde_json::to_string_pretty(&graph)?);

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::commands::network::strip_ansi;

    fn edge(src: &str, dst: &str, latency: &str, verdict: &str) -> Edge {
        Edge {
            source: src.into(),
            destination: dst.into(),
            latency: latency.into(),
            protocol: "TCP".into(),
            verdict: verdict.into(),
        }
    }

    fn fixture() -> (HashSet<String>, Vec<Edge>) {
        let edges = vec![
            edge("web", "api", "3ms", "FORWARDED"),
            edge("web", "db", "-", "DROP"),
            edge("api", "db", "1ms", "ALLOW"),
        ];
        let services = ["web", "api", "db"].iter().map(|s| s.to_string()).collect();
        (services, edges)
    }

    #[test]
    fn ascii_graph_renders() {
        let (services, edges) = fixture();
        assert_eq!(
            ascii_graph_lines(&services, &edges, false),
            vec![
                "━━━ Service Dependency Graph ━━━",
                "",
                "  [api]",
                "    └── → db TCP (1ms)",
                "",
                "  [web]",
                "    ├── → api TCP (3ms)",
                "    └── ✗ db TCP",
                "",
                "  [db]",
                "    (leaf) (no outgoing connections)",
                "",
                "ℹ 3 services, 3 connections",
                "",
            ]
        );
    }

    #[test]
    fn ascii_graph_color_strips_to_plain() {
        let (services, edges) = fixture();
        let plain = ascii_graph_lines(&services, &edges, false);
        let colored = ascii_graph_lines(&services, &edges, true);
        assert!(colored.iter().any(|l| l.contains('\x1b')));
        let stripped: Vec<String> = colored.iter().map(|l| strip_ansi(l)).collect();
        assert_eq!(stripped, plain);
    }

    #[test]
    fn empty_graph_says_so() {
        let lines = ascii_graph_lines(&HashSet::new(), &[], false);
        assert_eq!(lines[2], "No services or flows found.");
    }
}
