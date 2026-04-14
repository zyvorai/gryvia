use anyhow::Result;
use kube::api::{Api, ApiResource, GroupVersionKind, ListParams};
use kube::core::DynamicObject;
use colored::*;
use std::collections::{HashMap, HashSet};

use crate::client::TensorReaperClient;

pub async fn execute(
    client: &TensorReaperClient,
    namespace: &str,
    format: &str,
) -> Result<()> {
    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "tensorreaper.ai",
        "v1",
        "FabricFlow",
    ));
    let api: Api<DynamicObject> = Api::namespaced_with(
        client.kube_client.clone(),
        namespace,
        &ar,
    );

    let flows = match api.list(&ListParams::default()).await {
        Ok(list) => list,
        Err(e) => {
            println!(
                "{} Could not query flow resources: {}",
                "⚠".yellow().bold(),
                e
            );
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
            print_ascii_graph(&services, &edges);
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

fn print_ascii_graph(services: &HashSet<String>, edges: &[Edge]) {
    println!("{}", "━━━ Service Dependency Graph ━━━".bold().cyan());
    println!();

    if services.is_empty() {
        println!("  {}", "No services or flows found.".dimmed());
        println!();
        return;
    }

    // Group edges by source
    let mut adjacency: HashMap<&str, Vec<&Edge>> = HashMap::new();
    for edge in edges {
        adjacency.entry(&edge.source).or_default().push(edge);
    }

    // Collect services that only appear as destinations (leaf nodes)
    let sources: HashSet<&str> = edges.iter().map(|e| e.source.as_str()).collect();
    let mut printed: HashSet<&str> = HashSet::new();

    // Print the graph starting from services that are sources
    let mut sorted_sources: Vec<&str> = sources.iter().copied().collect();
    sorted_sources.sort();

    for source in &sorted_sources {
        if printed.contains(source) {
            continue;
        }
        print_node(source, &adjacency, &mut printed, 0);
    }

    // Print any remaining services not yet printed (isolated nodes)
    let mut remaining: Vec<&String> = services
        .iter()
        .filter(|s| !printed.contains(s.as_str()))
        .collect();
    remaining.sort();

    for svc in remaining {
        println!(
            "  {}",
            format!("[{}]", svc).bright_white()
        );
        println!("    {} (no outgoing connections)", "(leaf)".dimmed());
        println!();
    }

    // Summary
    println!(
        "  {} {} services, {} connections",
        "ℹ".cyan().bold(),
        services.len(),
        edges.len()
    );
    println!();
}

fn print_node<'a>(
    service: &'a str,
    adjacency: &HashMap<&'a str, Vec<&Edge>>,
    printed: &mut HashSet<&'a str>,
    depth: usize,
) {
    if printed.contains(service) {
        let indent = "  ".repeat(depth);
        println!(
            "{}  {} (circular ref)",
            indent,
            format!("[{}]", service).dimmed()
        );
        return;
    }

    printed.insert(service);

    let indent = "  ".repeat(depth);
    println!(
        "{}  {}",
        indent,
        format!("[{}]", service).bright_white().bold()
    );

    if let Some(targets) = adjacency.get(service) {
        for (i, edge) in targets.iter().enumerate() {
            let connector = if i == targets.len() - 1 {
                "└──"
            } else {
                "├──"
            };

            let verdict_symbol = match edge.verdict.as_str() {
                "FORWARDED" | "ALLOW" => "→".green().to_string(),
                "DROP" | "DENIED" => "✗".red().to_string(),
                _ => "?".yellow().to_string(),
            };

            let latency_display = if edge.latency == "-" {
                String::new()
            } else {
                format!(" ({})", edge.latency).dimmed().to_string()
            };

            println!(
                "{}    {} {} {} {}{}",
                indent,
                connector.dimmed(),
                verdict_symbol,
                edge.destination.bright_white(),
                edge.protocol.cyan(),
                latency_display,
            );
        }
    }

    println!();
}

fn print_json_graph(
    services: &HashSet<String>,
    edges: &[Edge],
) -> Result<()> {
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
