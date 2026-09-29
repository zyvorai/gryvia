use anyhow::Result;
use kube::api::{Api, ApiResource, GroupVersionKind, ListParams};
use kube::core::DynamicObject;

use crate::client::GryviaClient;
use crate::display;
use crate::ui::{self, Cell2, Marker};

pub enum GpuAction {
    Nccl {
        job: String,
        follow: bool,
        namespace: String,
    },
    Memory {
        node: String,
        namespace: String,
    },
    Rdma {
        node: String,
        namespace: String,
    },
    Training {
        job: String,
        namespace: String,
    },
}

pub async fn execute(client: &GryviaClient, action: GpuAction) -> Result<()> {
    match action {
        GpuAction::Nccl {
            job,
            follow,
            namespace,
        } => execute_nccl(client, &job, follow, &namespace).await,
        GpuAction::Memory { node, namespace } => execute_memory(client, &node, &namespace).await,
        GpuAction::Rdma { node, namespace } => execute_rdma(client, &node, &namespace).await,
        GpuAction::Training { job, namespace } => execute_training(client, &job, &namespace).await,
    }
}

async fn execute_nccl(
    client: &GryviaClient,
    job: &str,
    _follow: bool,
    namespace: &str,
) -> Result<()> {
    let color = ui::color_enabled();
    println!(
        "{}",
        ui::header(&format!("NCCL Collective Stats: {job}"), color)
    );
    println!();

    // Query GryviaTrainingInsight for this job
    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1alpha1",
        "GryviaTrainingInsight",
    ));
    let api: Api<DynamicObject> = Api::namespaced_with(client.kube_client.clone(), namespace, &ar);

    let label_selector = format!("gryvia.io/job={}", job);
    let params = ListParams::default().labels(&label_selector);

    let insights = match api.list(&params).await {
        Ok(list) => list,
        Err(e) => {
            display::print_warning(&format!("Could not query training insights: {}", e));
            println!(
                "{}",
                hint(
                    "Ensure a GryviaTrainingInsight CR exists for this job.",
                    color
                )
            );
            println!();
            return Ok(());
        }
    };

    if insights.items.is_empty() {
        // Also try to get by name directly
        match api.get(job).await {
            Ok(insight) => {
                print_lines(nccl_lines(&insight, color));
            }
            Err(_) => {
                println!("{}", hint("No NCCL stats found for this job.", color));
                println!(
                    "{}",
                    hint(
                        "Create a GryviaTrainingInsight CR targeting this job to collect stats.",
                        color
                    )
                );
            }
        }
    } else {
        for insight in &insights.items {
            print_lines(nccl_lines(insight, color));
        }
    }

    println!();
    Ok(())
}

fn print_lines(lines: Vec<String>) {
    for line in lines {
        println!("{line}");
    }
}

/// A hint line: warning glyph and text, for pages whose data source is missing.
fn hint(text: &str, color: bool) -> String {
    format!(
        "{} {}",
        Marker::Warn.paint_with(Marker::Warn.glyph(), color),
        text
    )
}

fn bottleneck_marker(bottleneck: &str) -> Marker {
    match bottleneck {
        "communication" => Marker::Error,
        "compute" | "data_loading" => Marker::Warn,
        _ => Marker::Disabled,
    }
}

/// Lines of the NCCL rank table and communication analysis for one training insight.
pub fn nccl_lines(insight: &DynamicObject, color: bool) -> Vec<String> {
    let status = insight.data.get("status");
    let mut lines = Vec::new();

    let rank_stats = status
        .and_then(|s| s.get("rankStats"))
        .and_then(|v| v.as_array());

    if let Some(ranks) = rank_stats {
        let rows: Vec<Vec<Cell2>> = ranks
            .iter()
            .map(|rank| {
                let rank_id = rank.get("rank").and_then(|v| v.as_i64()).unwrap_or(0);
                let avg_latency = rank
                    .get("avgLatencyNs")
                    .and_then(|v| v.as_i64())
                    .unwrap_or(0);
                let total_bytes = rank.get("totalBytes").and_then(|v| v.as_i64()).unwrap_or(0);
                let is_straggler = rank
                    .get("isStraggler")
                    .and_then(|v| v.as_bool())
                    .unwrap_or(false);
                let straggler = if is_straggler {
                    ("YES".to_string(), Some(Marker::Error))
                } else {
                    ("no".to_string(), Some(Marker::Disabled))
                };
                vec![
                    (rank_id.to_string(), None),
                    (format_ns(avg_latency), None),
                    (format_bytes(total_bytes), None),
                    straggler,
                ]
            })
            .collect();
        lines.extend(ui::grid(
            &["RANK", "AVG LATENCY", "TOTAL BYTES", "STRAGGLER"],
            &rows,
            color,
        ));
    } else {
        lines.push(hint("No rank statistics available.", color));
    }

    let comm_pattern = status
        .and_then(|s| s.get("commPattern"))
        .and_then(|v| v.as_str())
        .unwrap_or("-");
    let ratio = status
        .and_then(|s| s.get("commComputeRatio"))
        .and_then(|v| v.as_f64())
        .unwrap_or(0.0);
    let bottleneck = status
        .and_then(|s| s.get("bottleneck"))
        .and_then(|v| v.as_str())
        .unwrap_or("-");

    lines.push(String::new());
    lines.push(ui::section("Communication Analysis", color));
    lines.push(ui::kv("Pattern", comm_pattern, 14, color));
    lines.push(ui::kv("Comm/Compute", &format!("{ratio:.2}"), 14, color));
    lines.push(ui::kv(
        "Bottleneck",
        &bottleneck_marker(bottleneck).paint_with(bottleneck, color),
        14,
        color,
    ));
    lines
}

async fn execute_memory(client: &GryviaClient, node: &str, _namespace: &str) -> Result<()> {
    let color = ui::color_enabled();
    println!(
        "{}",
        ui::header(&format!("GPU Memory Transfer Stats: {node}"), color)
    );
    println!();

    // Query node GPU metrics
    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1alpha1",
        "GryviaGpuNode",
    ));
    let api: Api<DynamicObject> = Api::all_with(client.kube_client.clone(), &ar);

    match api.get(node).await {
        Ok(gpu_node) => {
            print_lines(memory_lines(tracked_gpus(&gpu_node), color));
        }
        Err(_) => {
            println!(
                "{}",
                hint("GPU memory stats not yet available for this node.", color)
            );
            println!(
                "{}",
                hint(
                    "Ensure eBPF collectors are deployed on the target node.",
                    color
                )
            );

            // Show placeholder stats
            println!();
            print_lines(memory_lines(None, color));
        }
    }

    println!();
    Ok(())
}

/// Number of GPUs listed in a node's status, when the list is present.
fn tracked_gpus(node: &DynamicObject) -> Option<usize> {
    node.data
        .get("status")
        .and_then(|s| s.get("gpus"))
        .and_then(|v| v.as_array())
        .map(|g| g.len())
}

/// Lines of the memory transfer table; `gpus` is the tracked GPU count when known.
pub fn memory_lines(gpus: Option<usize>, color: bool) -> Vec<String> {
    let h2d = match gpus {
        Some(n) => format!("{n} GPUs tracked"),
        None => "- GB/s".to_string(),
    };
    let dash = || ("-".to_string(), None);
    let rows: Vec<Vec<Cell2>> = vec![
        vec![
            ("H2D (Host->Device)".to_string(), None),
            (h2d, None),
            dash(),
            dash(),
        ],
        vec![
            ("D2H (Device->Host)".to_string(), None),
            ("- GB/s".to_string(), None),
            dash(),
            dash(),
        ],
        vec![
            ("D2D (Device->Device)".to_string(), None),
            ("- GB/s".to_string(), None),
            dash(),
            dash(),
        ],
    ];
    ui::grid(
        &["DIRECTION", "BANDWIDTH", "COUNT", "AVG LATENCY"],
        &rows,
        color,
    )
}

async fn execute_rdma(_client: &GryviaClient, node: &str, _namespace: &str) -> Result<()> {
    let color = ui::color_enabled();
    println!("{}", ui::header(&format!("RDMA Stats: {node}"), color));
    println!();

    // RDMA stats would come from the eBPF collector
    println!(
        "{}",
        hint(
            "RDMA statistics require eBPF collector to be deployed on the target node.",
            color
        )
    );
    println!(
        "{}",
        hint(
            "Ensure InfiniBand/RoCE devices are available and monitored.",
            color
        )
    );

    println!();
    Ok(())
}

async fn execute_training(client: &GryviaClient, job: &str, namespace: &str) -> Result<()> {
    let color = ui::color_enabled();
    println!(
        "{}",
        ui::header(&format!("Training Insights: {job}"), color)
    );
    println!();

    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1alpha1",
        "GryviaTrainingInsight",
    ));
    let api: Api<DynamicObject> = Api::namespaced_with(client.kube_client.clone(), namespace, &ar);

    // Try to get insight by job name
    let insight = match api.get(job).await {
        Ok(i) => i,
        Err(_) => {
            // Try listing with label
            let label_selector = format!("gryvia.io/job={}", job);
            let params = ListParams::default().labels(&label_selector);
            match api.list(&params).await {
                Ok(list) if !list.items.is_empty() => list.items[0].clone(),
                _ => {
                    println!(
                        "{}",
                        hint("No training insights found for this job.", color)
                    );
                    println!();
                    return Ok(());
                }
            }
        }
    };

    print_lines(training_lines(&insight, color));
    println!();
    Ok(())
}

/// Lines of the straggler detection and communication analysis sections.
pub fn training_lines(insight: &DynamicObject, color: bool) -> Vec<String> {
    let status = insight.data.get("status");
    let mut lines = Vec::new();

    let stragglers = status
        .and_then(|s| s.get("stragglers"))
        .and_then(|v| v.as_array());

    lines.push(ui::section("Straggler Detection", color));
    match stragglers {
        Some(list) if list.is_empty() => {
            lines.push(format!(
                "{} No stragglers detected.",
                Marker::Ok.paint_with(Marker::Ok.glyph(), color)
            ));
        }
        Some(list) => {
            for straggler in list {
                let rank = straggler.get("rank").and_then(|v| v.as_i64()).unwrap_or(0);
                let factor = straggler
                    .get("slowdownFactor")
                    .and_then(|v| v.as_f64())
                    .unwrap_or(0.0);
                let reason = straggler
                    .get("reason")
                    .and_then(|v| v.as_str())
                    .unwrap_or("-");
                lines.push(format!(
                    "{} Rank {} is {:.1}x slower ({})",
                    Marker::Error.paint_with(Marker::Error.glyph(), color),
                    rank,
                    factor,
                    reason
                ));
            }
        }
        None => lines.push(hint("No straggler data available.", color)),
    }
    lines.push(String::new());

    let comm_pattern = status
        .and_then(|s| s.get("commPattern"))
        .and_then(|v| v.as_str())
        .unwrap_or("-");
    let ratio = status
        .and_then(|s| s.get("commComputeRatio"))
        .and_then(|v| v.as_f64())
        .unwrap_or(0.0);
    let bottleneck = status
        .and_then(|s| s.get("bottleneck"))
        .and_then(|v| v.as_str())
        .unwrap_or("-");
    let last_analysis = status
        .and_then(|s| s.get("lastAnalysis"))
        .and_then(|v| v.as_str())
        .unwrap_or("-");

    let explanation = match bottleneck {
        "communication" => " - Network is the primary bottleneck",
        "compute" => " - GPU compute is the primary bottleneck",
        "data_loading" => " - Data loading is the primary bottleneck",
        _ => "",
    };
    let bottleneck_text = format!(
        "{}{}",
        bottleneck_marker(bottleneck).paint_with(bottleneck, color),
        explanation
    );

    lines.push(ui::section("Communication Analysis", color));
    lines.push(ui::kv("Pattern", comm_pattern, 15, color));
    lines.push(ui::kv("Comm/Compute", &format!("{ratio:.2}"), 15, color));
    lines.push(ui::kv("Bottleneck", &bottleneck_text, 15, color));
    lines.push(ui::kv("Last Analysis", last_analysis, 15, color));
    lines
}

fn format_ns(ns: i64) -> String {
    if ns >= 1_000_000_000 {
        format!("{:.2}s", ns as f64 / 1_000_000_000.0)
    } else if ns >= 1_000_000 {
        format!("{:.2}ms", ns as f64 / 1_000_000.0)
    } else if ns >= 1_000 {
        format!("{:.2}us", ns as f64 / 1_000.0)
    } else {
        format!("{}ns", ns)
    }
}

fn format_bytes(bytes: i64) -> String {
    if bytes >= 1_073_741_824 {
        format!("{:.2} GB", bytes as f64 / 1_073_741_824.0)
    } else if bytes >= 1_048_576 {
        format!("{:.2} MB", bytes as f64 / 1_048_576.0)
    } else if bytes >= 1024 {
        format!("{:.2} KB", bytes as f64 / 1024.0)
    } else {
        format!("{} B", bytes)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::commands::network::strip_ansi;
    use serde_json::json;

    fn insight(status: serde_json::Value) -> DynamicObject {
        serde_json::from_value(json!({
            "apiVersion": "gryvia.io/v1alpha1",
            "kind": "GryviaTrainingInsight",
            "metadata": {"name": "job"},
            "status": status,
        }))
        .unwrap()
    }

    fn nccl() -> DynamicObject {
        insight(json!({
            "rankStats": [
                {"rank": 0, "avgLatencyNs": 1_500_000, "totalBytes": 2_147_483_648i64, "isStraggler": false},
                {"rank": 1, "avgLatencyNs": 9_000_000, "totalBytes": 1024, "isStraggler": true},
            ],
            "commPattern": "ring",
            "commComputeRatio": 0.4567,
            "bottleneck": "communication",
        }))
    }

    #[test]
    fn nccl_renders() {
        assert_eq!(
            nccl_lines(&nccl(), false),
            vec![
                "RANK  AVG LATENCY  TOTAL BYTES  STRAGGLER",
                "0     1.50ms       2.00 GB      no",
                "1     9.00ms       1.00 KB      YES",
                "",
                "Communication Analysis",
                "Pattern       ring",
                "Comm/Compute  0.46",
                "Bottleneck    communication",
            ]
        );
    }

    #[test]
    fn nccl_without_ranks_hints() {
        let lines = nccl_lines(&insight(json!({})), false);
        assert_eq!(lines[0], "! No rank statistics available.");
    }

    #[test]
    fn memory_table_renders() {
        assert_eq!(
            memory_lines(Some(8), false),
            vec![
                "DIRECTION             BANDWIDTH       COUNT  AVG LATENCY",
                "H2D (Host->Device)    8 GPUs tracked  -      -",
                "D2H (Device->Host)    - GB/s          -      -",
                "D2D (Device->Device)  - GB/s          -      -",
            ]
        );
    }

    #[test]
    fn training_renders() {
        let i = insight(json!({
            "stragglers": [{"rank": 3, "slowdownFactor": 2.25, "reason": "thermal throttling"}],
            "commPattern": "tree",
            "commComputeRatio": 1.0,
            "bottleneck": "compute",
            "lastAnalysis": "2026-09-29T10:00:00Z",
        }));
        assert_eq!(
            training_lines(&i, false),
            vec![
                "Straggler Detection",
                "✗ Rank 3 is 2.2x slower (thermal throttling)",
                "",
                "Communication Analysis",
                "Pattern        tree",
                "Comm/Compute   1.00",
                "Bottleneck     compute - GPU compute is the primary bottleneck",
                "Last Analysis  2026-09-29T10:00:00Z",
            ]
        );
    }

    #[test]
    fn color_strips_to_plain() {
        for f in [
            nccl_lines as fn(&DynamicObject, bool) -> Vec<String>,
            training_lines,
        ] {
            let plain = f(&nccl(), false);
            let colored = f(&nccl(), true);
            assert!(colored.iter().any(|l| l.contains('\x1b')));
            let stripped: Vec<String> = colored.iter().map(|l| strip_ansi(l)).collect();
            assert_eq!(stripped, plain);
        }
    }
}
