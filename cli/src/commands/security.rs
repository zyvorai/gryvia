use anyhow::{Context, Result};
use colored::*;
use kube::api::{Api, ApiResource, GroupVersionKind, ListParams, PostParams};
use kube::core::DynamicObject;
use prettytable::{format, Cell, Row, Table};
use serde_json::json;

use crate::client::GryviaClient;

pub enum SecurityAction {
    Alerts {
        severity: Option<String>,
        alert_type: Option<String>,
        namespace: String,
    },
    Status {
        namespace: String,
    },
    PolicyList {
        namespace: String,
        output: String,
    },
    PolicyCreate {
        name: String,
        namespaces: Vec<String>,
        rules: Vec<String>,
        auto_block: bool,
        namespace: String,
    },
}

pub async fn execute(client: &GryviaClient, action: SecurityAction) -> Result<()> {
    match action {
        SecurityAction::Alerts {
            severity,
            alert_type,
            namespace,
        } => {
            execute_alerts(
                client,
                &namespace,
                severity.as_deref(),
                alert_type.as_deref(),
            )
            .await
        }
        SecurityAction::Status { namespace } => execute_status(client, &namespace).await,
        SecurityAction::PolicyList { namespace, output } => {
            execute_policy_list(client, &namespace, &output).await
        }
        SecurityAction::PolicyCreate {
            name,
            namespaces,
            rules,
            auto_block,
            namespace,
        } => {
            execute_policy_create(client, &namespace, &name, &namespaces, &rules, auto_block).await
        }
    }
}

async fn execute_alerts(
    client: &GryviaClient,
    namespace: &str,
    severity: Option<&str>,
    alert_type: Option<&str>,
) -> Result<()> {
    println!("{}", "━━━ Security Alerts ━━━".bold().cyan());
    println!();

    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1",
        "GryviaSecurityPolicy",
    ));
    let api: Api<DynamicObject> = Api::namespaced_with(client.kube_client.clone(), namespace, &ar);

    let policies = match api.list(&ListParams::default()).await {
        Ok(list) => list,
        Err(e) => {
            println!(
                "  {} Could not query security policies: {}",
                "!".yellow().bold(),
                e
            );
            println!();
            return Ok(());
        }
    };

    // Collect all alerts from all policies
    let mut all_alerts: Vec<(String, String, String, String, String)> = Vec::new();

    for policy in &policies.items {
        let status = policy.data.get("status");
        let detection_counts = status
            .and_then(|s| s.get("detectionCounts"))
            .and_then(|v| v.as_object());

        let policy_name = policy.metadata.name.as_deref().unwrap_or("-");

        if let Some(counts) = detection_counts {
            for (det_type, count) in counts {
                let count_val = count.as_i64().unwrap_or(0);
                if count_val == 0 {
                    continue;
                }

                // Apply type filter
                if let Some(filter_type) = alert_type {
                    if det_type != filter_type {
                        continue;
                    }
                }

                let sev = classify_alert_severity(det_type, count_val);

                // Apply severity filter
                if let Some(filter_sev) = severity {
                    if sev != filter_sev {
                        continue;
                    }
                }

                all_alerts.push((
                    sev.to_string(),
                    det_type.to_string(),
                    policy_name.to_string(),
                    format!("{}", count_val),
                    status
                        .and_then(|s| s.get("lastAlert"))
                        .and_then(|v| v.as_str())
                        .unwrap_or("-")
                        .to_string(),
                ));
            }
        }
    }

    if all_alerts.is_empty() {
        println!("  {}", "No security alerts detected.".green());
        println!();
        return Ok(());
    }

    let mut table = Table::new();
    table.set_format(*format::consts::FORMAT_BOX_CHARS);

    table.add_row(Row::new(vec![
        Cell::new("SEVERITY").style_spec("Fb"),
        Cell::new("TYPE").style_spec("Fb"),
        Cell::new("POLICY").style_spec("Fb"),
        Cell::new("COUNT").style_spec("Fb"),
        Cell::new("LAST ALERT").style_spec("Fb"),
    ]));

    for (sev, det_type, policy_name, count, last_alert) in &all_alerts {
        let severity_colored = match sev.as_str() {
            "critical" => sev.red().bold().to_string(),
            "high" => sev.yellow().bold().to_string(),
            "medium" => sev.blue().to_string(),
            "low" => sev.dimmed().to_string(),
            _ => sev.normal().to_string(),
        };

        table.add_row(Row::new(vec![
            Cell::new(&severity_colored),
            Cell::new(det_type),
            Cell::new(policy_name),
            Cell::new(count),
            Cell::new(last_alert),
        ]));
    }

    table.printstd();
    println!();
    println!("  {} Total alerts: {}", "i".cyan().bold(), all_alerts.len());
    println!();

    Ok(())
}

fn classify_alert_severity(alert_type: &str, count: i64) -> &'static str {
    match alert_type {
        "escape" | "privesc" => {
            if count > 5 {
                "critical"
            } else {
                "high"
            }
        }
        "mining" | "exfiltration" => {
            if count > 10 {
                "high"
            } else {
                "medium"
            }
        }
        "driver_fim" => "medium",
        _ => "low",
    }
}

async fn execute_status(client: &GryviaClient, namespace: &str) -> Result<()> {
    println!("{}", "━━━ Security Status Overview ━━━".bold().cyan());
    println!();

    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1",
        "GryviaSecurityPolicy",
    ));
    let api: Api<DynamicObject> = Api::namespaced_with(client.kube_client.clone(), namespace, &ar);

    let policies = match api.list(&ListParams::default()).await {
        Ok(list) => list,
        Err(e) => {
            println!(
                "  {} Could not query security policies: {}",
                "!".yellow().bold(),
                e
            );
            return Ok(());
        }
    };

    let total_policies = policies.items.len();
    let mut total_detections = 0;
    let mut total_alerts = 0;
    let mut active_policies = 0;

    for policy in &policies.items {
        let status = policy.data.get("status");
        let phase = status
            .and_then(|s| s.get("phase"))
            .and_then(|v| v.as_str())
            .unwrap_or("Unknown");

        if phase == "Active" {
            active_policies += 1;
        }

        total_detections += status
            .and_then(|s| s.get("activeDetections"))
            .and_then(|v| v.as_i64())
            .unwrap_or(0);

        total_alerts += status
            .and_then(|s| s.get("alertsTriggered"))
            .and_then(|v| v.as_i64())
            .unwrap_or(0);
    }

    println!("{}", "Summary:".bold().underline());
    println!(
        "  Security Policies: {} ({} active)",
        total_policies.to_string().bright_white().bold(),
        active_policies.to_string().green()
    );
    println!(
        "  Detection Rules:   {}",
        total_detections.to_string().bright_white().bold()
    );

    let alerts_display = if total_alerts > 0 {
        total_alerts.to_string().red().bold().to_string()
    } else {
        "0".green().to_string()
    };
    println!("  Total Alerts:      {}", alerts_display);
    println!();

    // Overall security health
    let health = if total_alerts > 50 {
        "CRITICAL".red().bold().to_string()
    } else if total_alerts > 10 {
        "WARNING".yellow().bold().to_string()
    } else if total_alerts > 0 {
        "MONITORING".blue().bold().to_string()
    } else {
        "SECURE".green().bold().to_string()
    };
    println!("  Security Status:   {}", health);
    println!();

    Ok(())
}

async fn execute_policy_list(client: &GryviaClient, namespace: &str, output: &str) -> Result<()> {
    println!("{}", "━━━ Security Policies ━━━".bold().cyan());
    println!();

    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1",
        "GryviaSecurityPolicy",
    ));
    let api: Api<DynamicObject> = Api::namespaced_with(client.kube_client.clone(), namespace, &ar);

    let policies = match api.list(&ListParams::default()).await {
        Ok(list) => list,
        Err(e) => {
            println!(
                "  {} Could not query security policies: {}",
                "!".yellow().bold(),
                e
            );
            return Ok(());
        }
    };

    if policies.items.is_empty() {
        println!("  {}", "No security policies found.".dimmed());
        println!();
        return Ok(());
    }

    match output {
        "json" => {
            let items: Vec<&serde_json::Value> = policies.items.iter().map(|p| &p.data).collect();
            println!("{}", serde_json::to_string_pretty(&items)?);
        }
        _ => {
            let mut table = Table::new();
            table.set_format(*format::consts::FORMAT_BOX_CHARS);

            table.add_row(Row::new(vec![
                Cell::new("NAME").style_spec("Fb"),
                Cell::new("PHASE").style_spec("Fb"),
                Cell::new("NAMESPACES").style_spec("Fb"),
                Cell::new("RULES").style_spec("Fb"),
                Cell::new("ALERTS").style_spec("Fb"),
                Cell::new("AUTO-BLOCK").style_spec("Fb"),
            ]));

            for policy in &policies.items {
                let name = policy.metadata.name.as_deref().unwrap_or("-");
                let spec = policy.data.get("spec");
                let status = policy.data.get("status");

                let phase = status
                    .and_then(|s| s.get("phase"))
                    .and_then(|v| v.as_str())
                    .unwrap_or("Unknown");

                let namespaces = spec
                    .and_then(|s| s.get("targetNamespaces"))
                    .and_then(|v| v.as_array())
                    .map(|arr| {
                        arr.iter()
                            .filter_map(|v| v.as_str())
                            .collect::<Vec<_>>()
                            .join(", ")
                    })
                    .unwrap_or_else(|| "-".to_string());

                let rules = status
                    .and_then(|s| s.get("activeDetections"))
                    .and_then(|v| v.as_i64())
                    .unwrap_or(0);

                let alerts = status
                    .and_then(|s| s.get("alertsTriggered"))
                    .and_then(|v| v.as_i64())
                    .unwrap_or(0);

                let auto_block = spec
                    .and_then(|s| s.get("autoBlock"))
                    .and_then(|v| v.as_bool())
                    .unwrap_or(false);

                let phase_colored = match phase {
                    "Active" => phase.green().to_string(),
                    "Error" => phase.red().to_string(),
                    _ => phase.yellow().to_string(),
                };

                let auto_block_display = if auto_block {
                    "enabled".green().to_string()
                } else {
                    "disabled".dimmed().to_string()
                };

                table.add_row(Row::new(vec![
                    Cell::new(name),
                    Cell::new(&phase_colored),
                    Cell::new(&namespaces),
                    Cell::new(&rules.to_string()),
                    Cell::new(&alerts.to_string()),
                    Cell::new(&auto_block_display),
                ]));
            }

            table.printstd();
        }
    }

    println!();
    Ok(())
}

async fn execute_policy_create(
    client: &GryviaClient,
    namespace: &str,
    name: &str,
    target_namespaces: &[String],
    rules: &[String],
    auto_block: bool,
) -> Result<()> {
    println!("{}", "━━━ Create Security Policy ━━━".bold().cyan());
    println!();

    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1",
        "GryviaSecurityPolicy",
    ));
    let api: Api<DynamicObject> = Api::namespaced_with(client.kube_client.clone(), namespace, &ar);

    let detection_rules: Vec<serde_json::Value> = rules
        .iter()
        .map(|r| {
            json!({
                "type": r,
                "enabled": true,
                "sensitivity": "medium"
            })
        })
        .collect();

    let policy_obj = serde_json::from_value(json!({
        "apiVersion": "gryvia.io/v1",
        "kind": "GryviaSecurityPolicy",
        "metadata": {
            "name": name,
            "namespace": namespace,
        },
        "spec": {
            "targetNamespaces": target_namespaces,
            "detectionRules": detection_rules,
            "autoBlock": auto_block,
        }
    }))
    .context("Failed to build security policy object")?;

    match api.create(&PostParams::default(), &policy_obj).await {
        Ok(_) => {
            println!(
                "{} Created security policy: {}",
                "OK".green().bold(),
                name.bright_white()
            );
            println!("  Namespaces: {}", target_namespaces.join(", "));
            println!("  Rules: {}", rules.join(", "));
            println!(
                "  Auto-block: {}",
                if auto_block {
                    "enabled".green()
                } else {
                    "disabled".dimmed()
                }
            );
        }
        Err(e) => {
            println!(
                "{} Failed to create security policy: {}",
                "ERROR".red().bold(),
                e
            );
        }
    }

    println!();
    Ok(())
}
