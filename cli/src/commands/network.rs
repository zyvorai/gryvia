use anyhow::Result;
use kube::api::{Api, ApiResource, GroupVersionKind, ListParams};
use kube::core::DynamicObject;
use prettytable::{Table, Row, Cell, format};
use colored::*;

use crate::client::GryviaClient;

pub async fn execute_status(client: &GryviaClient, namespace: &str) -> Result<()> {
    println!("{}", "━━━ Network Health Overview ━━━".bold().cyan());
    println!();

    // Query flows
    let flow_ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1",
        "FabricFlow",
    ));
    let flow_api: Api<DynamicObject> = Api::namespaced_with(
        client.kube_client.clone(),
        namespace,
        &flow_ar,
    );

    let active_flows = match flow_api.list(&ListParams::default()).await {
        Ok(list) => list.items.len(),
        Err(_) => 0,
    };

    // Query policies
    let policy_ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1",
        "FabricFlowPolicy",
    ));
    let policy_api: Api<DynamicObject> = Api::namespaced_with(
        client.kube_client.clone(),
        namespace,
        &policy_ar,
    );

    let (total_policies, enforced_policies) = match policy_api.list(&ListParams::default()).await {
        Ok(list) => {
            let enforced = list
                .items
                .iter()
                .filter(|p| {
                    p.data
                        .get("status")
                        .and_then(|s| s.get("phase"))
                        .and_then(|v| v.as_str())
                        .map(|phase| phase == "Enforced" || phase == "Active")
                        .unwrap_or(false)
                })
                .count();
            (list.items.len(), enforced)
        }
        Err(_) => (0, 0),
    };

    // Query anomalies
    let anomaly_ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1",
        "FabricNetworkAnomaly",
    ));
    let anomaly_api: Api<DynamicObject> = Api::namespaced_with(
        client.kube_client.clone(),
        namespace,
        &anomaly_ar,
    );

    let (total_anomalies, critical_anomalies) =
        match anomaly_api.list(&ListParams::default()).await {
            Ok(list) => {
                let critical = list
                    .items
                    .iter()
                    .filter(|a| {
                        a.data
                            .get("spec")
                            .and_then(|s| s.get("severity"))
                            .and_then(|v| v.as_str())
                            .map(|sev| sev == "critical")
                            .unwrap_or(false)
                    })
                    .count();
                (list.items.len(), critical)
            }
            Err(_) => (0, 0),
        };

    // Query trace sessions
    let trace_ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1",
        "FabricTraceSession",
    ));
    let trace_api: Api<DynamicObject> = Api::namespaced_with(
        client.kube_client.clone(),
        namespace,
        &trace_ar,
    );

    let active_traces = match trace_api.list(&ListParams::default()).await {
        Ok(list) => list
            .items
            .iter()
            .filter(|t| {
                t.data
                    .get("status")
                    .and_then(|s| s.get("phase"))
                    .and_then(|v| v.as_str())
                    .map(|phase| phase == "Running" || phase == "Active")
                    .unwrap_or(false)
            })
            .count(),
        Err(_) => 0,
    };

    // Print summary
    println!("{}", "Overview:".bold().underline());
    println!(
        "  Active Flows:      {}",
        active_flows.to_string().bright_white().bold()
    );
    println!(
        "  Policies:          {} ({} enforced)",
        total_policies.to_string().bright_white().bold(),
        enforced_policies.to_string().green()
    );

    let anomaly_display = if critical_anomalies > 0 {
        format!(
            "{} ({} critical)",
            total_anomalies.to_string().yellow().bold(),
            critical_anomalies.to_string().red().bold()
        )
    } else if total_anomalies > 0 {
        total_anomalies.to_string().yellow().bold().to_string()
    } else {
        "0".green().to_string()
    };
    println!("  Anomalies:         {}", anomaly_display);
    println!(
        "  Active Traces:     {}",
        active_traces.to_string().cyan().bold()
    );
    println!();

    // Network health indicator
    let health = if critical_anomalies > 0 {
        "DEGRADED".red().bold().to_string()
    } else if total_anomalies > 0 {
        "WARNING".yellow().bold().to_string()
    } else {
        "HEALTHY".green().bold().to_string()
    };
    println!("  Network Status:    {}", health);
    println!();

    Ok(())
}

pub async fn execute_anomalies(
    client: &GryviaClient,
    namespace: &str,
    service: Option<&str>,
    severity: Option<&str>,
    output: &str,
) -> Result<()> {
    println!("{}", "━━━ Network Anomalies ━━━".bold().cyan());
    println!();

    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1",
        "FabricNetworkAnomaly",
    ));
    let api: Api<DynamicObject> = Api::namespaced_with(
        client.kube_client.clone(),
        namespace,
        &ar,
    );

    let mut params = ListParams::default();
    if let Some(svc) = service {
        let label_selector = format!("gryvia.io/service={}", svc);
        params = params.labels(&label_selector);
    }

    let anomalies = match api.list(&params).await {
        Ok(list) => list,
        Err(e) => {
            println!(
                "  {} Could not query anomalies: {}",
                "⚠".yellow().bold(),
                e
            );
            println!();
            return Ok(());
        }
    };

    // Filter by severity if specified
    let filtered: Vec<&DynamicObject> = anomalies
        .items
        .iter()
        .filter(|a| {
            if let Some(sev) = severity {
                a.data
                    .get("spec")
                    .and_then(|s| s.get("severity"))
                    .and_then(|v| v.as_str())
                    .map(|actual_sev| actual_sev == sev)
                    .unwrap_or(false)
            } else {
                true
            }
        })
        .collect();

    if filtered.is_empty() {
        println!("  {}", "No anomalies detected.".green());
        println!();
        return Ok(());
    }

    match output {
        "json" => {
            let items: Vec<&serde_json::Value> =
                filtered.iter().map(|a| &a.data).collect();
            println!("{}", serde_json::to_string_pretty(&items)?);
        }
        _ => {
            print_anomalies_table(&filtered);
        }
    }

    Ok(())
}

fn print_anomalies_table(anomalies: &[&DynamicObject]) {
    let mut table = Table::new();
    table.set_format(*format::consts::FORMAT_BOX_CHARS);

    table.add_row(Row::new(vec![
        Cell::new("SEVERITY").style_spec("Fb"),
        Cell::new("SERVICE").style_spec("Fb"),
        Cell::new("TYPE").style_spec("Fb"),
        Cell::new("DESCRIPTION").style_spec("Fb"),
        Cell::new("DETECTED AT").style_spec("Fb"),
    ]));

    for anomaly in anomalies {
        let spec = anomaly.data.get("spec");

        let severity = spec
            .and_then(|s| s.get("severity"))
            .and_then(|v| v.as_str())
            .unwrap_or("unknown");
        let svc = spec
            .and_then(|s| s.get("service"))
            .and_then(|v| v.as_str())
            .unwrap_or("-");
        let anomaly_type = spec
            .and_then(|s| s.get("type"))
            .and_then(|v| v.as_str())
            .unwrap_or("-");
        let description = spec
            .and_then(|s| s.get("description"))
            .and_then(|v| v.as_str())
            .unwrap_or("-");
        let detected_at = spec
            .and_then(|s| s.get("detectedAt"))
            .and_then(|v| v.as_str())
            .unwrap_or("-");

        let severity_colored = match severity {
            "critical" => severity.red().bold().to_string(),
            "high" => severity.red().to_string(),
            "medium" | "warning" => severity.yellow().to_string(),
            "low" | "info" => severity.cyan().to_string(),
            _ => severity.normal().to_string(),
        };

        table.add_row(Row::new(vec![
            Cell::new(&severity_colored),
            Cell::new(svc),
            Cell::new(anomaly_type),
            Cell::new(description),
            Cell::new(detected_at),
        ]));
    }

    table.printstd();
    println!();
    println!(
        "  {} Total anomalies: {}",
        "ℹ".cyan().bold(),
        anomalies.len()
    );
    println!();
}
