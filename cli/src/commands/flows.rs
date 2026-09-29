use anyhow::Result;
use colored::*;
use kube::api::{Api, ApiResource, GroupVersionKind, ListParams};
use kube::core::DynamicObject;
use prettytable::{format, Cell, Row, Table};

use crate::client::GryviaClient;

pub async fn execute(
    client: &GryviaClient,
    service: Option<&str>,
    namespace: &str,
    last: &str,
    output: &str,
) -> Result<()> {
    println!("{}", "━━━ Network Flows ━━━".bold().cyan());
    println!();

    if let Some(svc) = service {
        println!("  {} {}", "Service:".bold(), svc);
    }
    println!("  {} {}", "Namespace:".bold(), namespace);
    println!("  {} {}", "Time window:".bold(), last);
    println!();

    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk("gryvia.io", "v1", "GryviaFlow"));
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
            println!(
                "  {} Could not query flow resources: {}",
                "⚠".yellow().bold(),
                e
            );
            println!();
            return Ok(());
        }
    };

    if flows.items.is_empty() {
        println!("  {}", "No network flows found.".dimmed());
        println!();
        return Ok(());
    }

    match output {
        "json" => {
            println!("{}", serde_json::to_string_pretty(&flows)?);
        }
        _ => {
            print_flows_table(&flows.items);
        }
    }

    Ok(())
}

fn print_flows_table(flows: &[DynamicObject]) {
    let mut table = Table::new();
    table.set_format(*format::consts::FORMAT_BOX_CHARS);

    table.add_row(Row::new(vec![
        Cell::new("TIMESTAMP").style_spec("Fb"),
        Cell::new("SOURCE").style_spec("Fb"),
        Cell::new("DESTINATION").style_spec("Fb"),
        Cell::new("PROTOCOL").style_spec("Fb"),
        Cell::new("PORT").style_spec("Fb"),
        Cell::new("BYTES").style_spec("Fb"),
        Cell::new("LATENCY").style_spec("Fb"),
        Cell::new("VERDICT").style_spec("Fb"),
    ]));

    for flow in flows {
        let spec = flow.data.get("spec");

        let timestamp = spec
            .and_then(|s| s.get("timestamp"))
            .and_then(|v| v.as_str())
            .unwrap_or("-");
        let source = spec
            .and_then(|s| s.get("source"))
            .and_then(|v| v.as_str())
            .unwrap_or("-");
        let destination = spec
            .and_then(|s| s.get("destination"))
            .and_then(|v| v.as_str())
            .unwrap_or("-");
        let protocol = spec
            .and_then(|s| s.get("protocol"))
            .and_then(|v| v.as_str())
            .unwrap_or("TCP");
        let port = spec
            .and_then(|s| s.get("port"))
            .and_then(|v| v.as_u64())
            .map(|p| p.to_string())
            .unwrap_or_else(|| "-".to_string());
        let bytes = spec
            .and_then(|s| s.get("bytes"))
            .and_then(|v| v.as_str())
            .unwrap_or("0B");
        let latency = spec
            .and_then(|s| s.get("latency"))
            .and_then(|v| v.as_str())
            .unwrap_or("-");
        let verdict = spec
            .and_then(|s| s.get("verdict"))
            .and_then(|v| v.as_str())
            .unwrap_or("FORWARDED");

        let verdict_colored = match verdict {
            "FORWARDED" | "ALLOW" => verdict.green().to_string(),
            "DROP" | "DENIED" => verdict.red().to_string(),
            _ => verdict.yellow().to_string(),
        };

        table.add_row(Row::new(vec![
            Cell::new(timestamp),
            Cell::new(source),
            Cell::new(destination),
            Cell::new(protocol),
            Cell::new(&port),
            Cell::new(bytes),
            Cell::new(latency),
            Cell::new(&verdict_colored),
        ]));
    }

    table.printstd();
    println!();
    println!("  {} Total flows: {}", "ℹ".cyan().bold(), flows.len());
    println!();
}
