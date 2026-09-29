use anyhow::{Context, Result};
use kube::api::{Api, ApiResource, GroupVersionKind, ListParams, Patch, PatchParams};
use kube::core::DynamicObject;
use serde_json::json;

use crate::client::GryviaClient;
use crate::commands::flows::verdict_marker;
use crate::display;
use crate::ui::{self, Cell2, Marker};

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
    let color = ui::color_enabled();
    println!("{}", ui::header("Flow Policies", color));
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
            display::print_warning(&format!("Could not query flow policies: {}", e));
            println!();
            return Ok(());
        }
    };

    if policies.items.is_empty() {
        println!(
            "{}",
            Marker::Disabled.paint_with("No flow policies found.", color)
        );
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

/// Marker for a policy phase.
fn policy_phase_marker(phase: &str) -> Marker {
    match phase {
        "Enforced" | "Active" => Marker::Ok,
        "Pending" | "Suggested" => Marker::Warn,
        "Error" | "Failed" => Marker::Error,
        _ => Marker::Unknown,
    }
}

/// Lines of the flow policies table.
pub fn policies_lines(policies: &[DynamicObject], color: bool) -> Vec<String> {
    let rows: Vec<Vec<Cell2>> = policies
        .iter()
        .map(|policy| {
            let name = policy.metadata.name.as_deref().unwrap_or("<unknown>");
            let spec = policy.data.get("spec");
            let status = policy.data.get("status");
            let text = |key: &str| -> String {
                spec.and_then(|s| s.get(key))
                    .and_then(|v| v.as_str())
                    .unwrap_or("-")
                    .to_string()
            };
            let port = spec
                .and_then(|s| s.get("port"))
                .and_then(|v| v.as_u64())
                .map(|p| p.to_string())
                .unwrap_or_else(|| "-".to_string());
            let action = spec
                .and_then(|s| s.get("action"))
                .and_then(|v| v.as_str())
                .unwrap_or("allow");
            let phase = status
                .and_then(|s| s.get("phase"))
                .and_then(|v| v.as_str())
                .unwrap_or("Pending");
            let matched_flows = status
                .and_then(|s| s.get("matchedFlows"))
                .and_then(|v| v.as_u64())
                .unwrap_or(0);
            vec![
                (name.to_string(), None),
                (text("sourceService"), None),
                (text("destinationService"), None),
                (port, None),
                (action.to_string(), Some(verdict_marker(action))),
                (text("intent"), None),
                (phase.to_string(), Some(policy_phase_marker(phase))),
                (matched_flows.to_string(), None),
            ]
        })
        .collect();
    ui::grid(
        &[
            "NAME",
            "SOURCE",
            "DESTINATION",
            "PORT",
            "ACTION",
            "INTENT",
            "STATUS",
            "MATCHED FLOWS",
        ],
        &rows,
        color,
    )
}

fn print_policies_table(policies: &[DynamicObject]) {
    for line in policies_lines(policies, ui::color_enabled()) {
        println!("{line}");
    }
    println!();
}

async fn suggest_policies(client: &GryviaClient, namespace: &str) -> Result<()> {
    let color = ui::color_enabled();
    println!("{}", ui::header("Suggested Policies", color));
    println!();
    println!(
        "{}",
        Marker::Disabled.paint_with("Analyzing observed traffic patterns...", color)
    );
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
            display::print_warning(&format!("Could not query policy suggestions: {}", e));
            println!();
            return Ok(());
        }
    };

    if suggestions.items.is_empty() {
        println!(
            "{}",
            Marker::Disabled.paint_with(
                "No policy suggestions available. Traffic analysis may still be in progress.",
                color
            )
        );
        println!();
        return Ok(());
    }

    for line in suggestion_lines(&suggestions.items, color) {
        println!("{line}");
    }

    Ok(())
}

/// Lines of the numbered policy suggestions.
pub fn suggestion_lines(suggestions: &[DynamicObject], color: bool) -> Vec<String> {
    let mut lines = Vec::new();
    for (i, suggestion) in suggestions.iter().enumerate() {
        let name = suggestion.metadata.name.as_deref().unwrap_or("<unknown>");
        let spec = suggestion.data.get("spec");
        let text = |key: &str, default: &str| -> String {
            spec.and_then(|s| s.get(key))
                .and_then(|v| v.as_str())
                .unwrap_or(default)
                .to_string()
        };
        let port = spec
            .and_then(|s| s.get("port"))
            .and_then(|v| v.as_u64())
            .map(|p| p.to_string())
            .unwrap_or_else(|| "*".to_string());
        let action = text("action", "allow");
        let confidence = spec
            .and_then(|s| s.get("confidence"))
            .and_then(|v| v.as_f64())
            .unwrap_or(0.0);

        lines.push(format!(
            "{} {} (confidence: {:.0}%)",
            ui::ansi(&format!("{}.", i + 1), "1", color),
            name,
            confidence * 100.0
        ));
        lines.push(format!(
            "   {} → {}  port:{}  action:{}",
            text("sourceService", "-"),
            text("destinationService", "-"),
            port,
            verdict_marker(&action).paint_with(&action, color)
        ));
        lines.push(format!(
            "   {}",
            ui::kv(
                "Intent",
                &text("intent", "observed traffic pattern"),
                8,
                color
            )
        ));
        lines.push(format!(
            "   {}",
            ui::kv(
                "Apply",
                &Marker::Disabled.paint_with(&format!("gryvia network policy apply {name}"), color),
                8,
                color
            )
        ));
        lines.push(String::new());
    }
    lines
}

async fn apply_policy(client: &GryviaClient, policy_name: &str, namespace: &str) -> Result<()> {
    let color = ui::color_enabled();
    println!(
        "{} Applying policy: {}",
        ui::ansi("→", "1;36", color),
        policy_name
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
        display::print_info(&format!("Policy '{}' is already enforced.", policy_name));
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
        "{} Policy '{}' applied and enforced.",
        Marker::Ok.paint_with(Marker::Ok.glyph(), color),
        policy_name
    );
    println!();

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::commands::network::strip_ansi;
    use serde_json::json;

    fn policy(name: &str, spec: serde_json::Value, status: serde_json::Value) -> DynamicObject {
        serde_json::from_value(json!({
            "apiVersion": "gryvia.io/v1alpha1",
            "kind": "GryviaFlowPolicy",
            "metadata": {"name": name},
            "spec": spec,
            "status": status,
        }))
        .unwrap()
    }

    fn fixture() -> Vec<DynamicObject> {
        vec![
            policy(
                "web-to-api",
                json!({"sourceService": "web", "destinationService": "api", "port": 8080,
                    "action": "allow", "intent": "frontend traffic", "confidence": 0.9}),
                json!({"phase": "Enforced", "matchedFlows": 120}),
            ),
            policy(
                "block-db",
                json!({"sourceService": "web", "destinationService": "db", "action": "deny"}),
                json!({}),
            ),
        ]
    }

    #[test]
    fn policies_table_is_aligned() {
        assert_eq!(
            policies_lines(&fixture(), false),
            vec![
                "NAME        SOURCE  DESTINATION  PORT  ACTION  INTENT            STATUS    MATCHED FLOWS",
                "web-to-api  web     api          8080  allow   frontend traffic  Enforced  120",
                "block-db    web     db           -     deny    -                 Pending   0",
            ]
        );
    }

    #[test]
    fn suggestions_render() {
        let lines = suggestion_lines(&fixture()[..1], false);
        assert_eq!(
            lines,
            vec![
                "1. web-to-api (confidence: 90%)",
                "   web → api  port:8080  action:allow",
                "   Intent  frontend traffic",
                "   Apply   gryvia network policy apply web-to-api",
                "",
            ]
        );
    }

    #[test]
    fn color_strips_to_plain() {
        let plain = policies_lines(&fixture(), false);
        let colored = policies_lines(&fixture(), true);
        assert!(colored.iter().any(|l| l.contains('\x1b')));
        let stripped: Vec<String> = colored.iter().map(|l| strip_ansi(l)).collect();
        assert_eq!(stripped, plain);
    }
}
