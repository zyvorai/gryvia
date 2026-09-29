use anyhow::{Context, Result};
use colored::*;
use kube::api::{Api, ApiResource, GroupVersionKind, ListParams, Patch, PatchParams};
use kube::core::DynamicObject;
use prettytable::{format, Cell, Row, Table};
use serde_json::json;

use crate::client::GryviaClient;

pub enum PolicyAction {
    Suggest {
        namespace: String,
    },
    Apply {
        policy_name: String,
        namespace: String,
    },
    List {
        namespace: String,
        output: String,
    },
}

pub async fn execute(client: &GryviaClient, action: PolicyAction) -> Result<()> {
    match action {
        PolicyAction::Suggest { namespace } => suggest_policies(client, &namespace).await,
        PolicyAction::Apply {
            policy_name,
            namespace,
        } => apply_policy(client, &policy_name, &namespace).await,
        PolicyAction::List { namespace, output } => {
            list_policies(client, &namespace, &output).await
        }
    }
}

async fn list_policies(client: &GryviaClient, namespace: &str, output: &str) -> Result<()> {
    println!("{}", "━━━ Flow Policies ━━━".bold().cyan());
    println!();

    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1alpha1",
        "GryviaFlowPolicy",
    ));
    let api: Api<DynamicObject> = Api::namespaced_with(client.kube_client.clone(), namespace, &ar);

    let policies = match api.list(&ListParams::default()).await {
        Ok(list) => list,
        Err(e) => {
            println!(
                "  {} Could not query flow policies: {}",
                "⚠".yellow().bold(),
                e
            );
            println!();
            return Ok(());
        }
    };

    if policies.items.is_empty() {
        println!("  {}", "No flow policies found.".dimmed());
        println!();
        return Ok(());
    }

    match output {
        "json" => {
            println!("{}", serde_json::to_string_pretty(&policies)?);
        }
        "yaml" => {
            println!("{}", serde_yaml::to_string(&policies)?);
        }
        _ => {
            print_policies_table(&policies.items);
        }
    }

    Ok(())
}

fn print_policies_table(policies: &[DynamicObject]) {
    let mut table = Table::new();
    table.set_format(*format::consts::FORMAT_BOX_CHARS);

    table.add_row(Row::new(vec![
        Cell::new("NAME").style_spec("Fb"),
        Cell::new("SOURCE").style_spec("Fb"),
        Cell::new("DESTINATION").style_spec("Fb"),
        Cell::new("PORT").style_spec("Fb"),
        Cell::new("ACTION").style_spec("Fb"),
        Cell::new("INTENT").style_spec("Fb"),
        Cell::new("STATUS").style_spec("Fb"),
        Cell::new("MATCHED FLOWS").style_spec("Fb"),
    ]));

    for policy in policies {
        let name = policy.metadata.name.as_deref().unwrap_or("<unknown>");
        let spec = policy.data.get("spec");
        let status = policy.data.get("status");

        let source = spec
            .and_then(|s| s.get("sourceService"))
            .and_then(|v| v.as_str())
            .unwrap_or("-");
        let destination = spec
            .and_then(|s| s.get("destinationService"))
            .and_then(|v| v.as_str())
            .unwrap_or("-");
        let port = spec
            .and_then(|s| s.get("port"))
            .and_then(|v| v.as_u64())
            .map(|p| p.to_string())
            .unwrap_or_else(|| "-".to_string());
        let action = spec
            .and_then(|s| s.get("action"))
            .and_then(|v| v.as_str())
            .unwrap_or("allow");
        let intent = spec
            .and_then(|s| s.get("intent"))
            .and_then(|v| v.as_str())
            .unwrap_or("-");

        let phase = status
            .and_then(|s| s.get("phase"))
            .and_then(|v| v.as_str())
            .unwrap_or("Pending");
        let matched_flows = status
            .and_then(|s| s.get("matchedFlows"))
            .and_then(|v| v.as_u64())
            .unwrap_or(0);

        let action_colored = match action {
            "allow" | "ALLOW" => action.green().to_string(),
            "deny" | "DENY" | "drop" | "DROP" => action.red().to_string(),
            _ => action.yellow().to_string(),
        };

        let phase_colored = match phase {
            "Enforced" | "Active" => phase.green().to_string(),
            "Pending" => phase.yellow().to_string(),
            "Suggested" => phase.cyan().to_string(),
            _ => phase.normal().to_string(),
        };

        table.add_row(Row::new(vec![
            Cell::new(name),
            Cell::new(source),
            Cell::new(destination),
            Cell::new(&port),
            Cell::new(&action_colored),
            Cell::new(intent),
            Cell::new(&phase_colored),
            Cell::new(&matched_flows.to_string()),
        ]));
    }

    table.printstd();
    println!();
}

async fn suggest_policies(client: &GryviaClient, namespace: &str) -> Result<()> {
    println!("{}", "━━━ Suggested Policies ━━━".bold().cyan());
    println!();
    println!("  {}", "Analyzing observed traffic patterns...".dimmed());
    println!();

    // Query for suggested policies (those with phase=Suggested)
    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1alpha1",
        "GryviaFlowPolicy",
    ));
    let api: Api<DynamicObject> = Api::namespaced_with(client.kube_client.clone(), namespace, &ar);

    let label_selector = "gryvia.io/suggested=true";
    let params = ListParams::default().labels(label_selector);

    let suggestions = match api.list(&params).await {
        Ok(list) => list,
        Err(e) => {
            println!(
                "  {} Could not query policy suggestions: {}",
                "⚠".yellow().bold(),
                e
            );
            println!();
            return Ok(());
        }
    };

    if suggestions.items.is_empty() {
        println!(
            "  {}",
            "No policy suggestions available. Traffic analysis may still be in progress.".dimmed()
        );
        println!();
        return Ok(());
    }

    for (i, suggestion) in suggestions.items.iter().enumerate() {
        let name = suggestion.metadata.name.as_deref().unwrap_or("<unknown>");
        let spec = suggestion.data.get("spec");

        let source = spec
            .and_then(|s| s.get("sourceService"))
            .and_then(|v| v.as_str())
            .unwrap_or("-");
        let destination = spec
            .and_then(|s| s.get("destinationService"))
            .and_then(|v| v.as_str())
            .unwrap_or("-");
        let port = spec
            .and_then(|s| s.get("port"))
            .and_then(|v| v.as_u64())
            .map(|p| p.to_string())
            .unwrap_or_else(|| "*".to_string());
        let action = spec
            .and_then(|s| s.get("action"))
            .and_then(|v| v.as_str())
            .unwrap_or("allow");
        let intent = spec
            .and_then(|s| s.get("intent"))
            .and_then(|v| v.as_str())
            .unwrap_or("observed traffic pattern");
        let confidence = spec
            .and_then(|s| s.get("confidence"))
            .and_then(|v| v.as_f64())
            .unwrap_or(0.0);

        println!(
            "  {}. {} (confidence: {:.0}%)",
            (i + 1).to_string().bold(),
            name.bright_white(),
            confidence * 100.0
        );
        println!(
            "     {} → {} port:{} action:{} ",
            source.cyan(),
            destination.cyan(),
            port,
            match action {
                "allow" | "ALLOW" => action.green().to_string(),
                "deny" | "DENY" => action.red().to_string(),
                _ => action.yellow().to_string(),
            }
        );
        println!("     Intent: {}", intent.dimmed());
        println!(
            "     Apply: {} policy apply {}",
            "gryvia network".dimmed(),
            name.dimmed()
        );
        println!();
    }

    Ok(())
}

async fn apply_policy(client: &GryviaClient, policy_name: &str, namespace: &str) -> Result<()> {
    println!(
        "{} Applying policy: {}",
        "→".cyan().bold(),
        policy_name.bright_white()
    );

    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1alpha1",
        "GryviaFlowPolicy",
    ));
    let api: Api<DynamicObject> = Api::namespaced_with(client.kube_client.clone(), namespace, &ar);

    // Check policy exists
    let policy = api
        .get(policy_name)
        .await
        .context(format!("Policy '{}' not found", policy_name))?;

    let current_phase = policy
        .data
        .get("status")
        .and_then(|s| s.get("phase"))
        .and_then(|v| v.as_str())
        .unwrap_or("Unknown");

    if current_phase == "Enforced" || current_phase == "Active" {
        println!(
            "  {} Policy '{}' is already enforced.",
            "ℹ".cyan().bold(),
            policy_name
        );
        return Ok(());
    }

    // Patch the policy to set phase to Enforced and remove suggested label
    let patch = json!({
        "status": {
            "phase": "Enforced"
        },
        "metadata": {
            "labels": {
                "gryvia.io/suggested": null
            }
        }
    });

    api.patch(
        policy_name,
        &PatchParams::apply("gryvia-cli"),
        &Patch::Merge(&patch),
    )
    .await
    .context("Failed to apply policy")?;

    println!(
        "  {} Policy '{}' applied and enforced.",
        "✓".green().bold(),
        policy_name
    );
    println!();

    Ok(())
}
