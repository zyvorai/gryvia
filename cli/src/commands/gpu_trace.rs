use anyhow::Result;
use kube::api::{Api, ApiResource, GroupVersionKind, ListParams};
use kube::core::DynamicObject;
use prettytable::{Table, Row, Cell, format};
use colored::*;

use crate::client::GryviaClient;

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
        GpuAction::Nccl { job, follow, namespace } => {
            execute_nccl(client, &job, follow, &namespace).await
        }
        GpuAction::Memory { node, namespace } => {
            execute_memory(client, &node, &namespace).await
        }
        GpuAction::Rdma { node, namespace } => {
            execute_rdma(client, &node, &namespace).await
        }
        GpuAction::Training { job, namespace } => {
            execute_training(client, &job, &namespace).await
        }
    }
}

async fn execute_nccl(
    client: &GryviaClient,
    job: &str,
    _follow: bool,
    namespace: &str,
) -> Result<()> {
    println!(
        "{}",
        format!("━━━ NCCL Collective Stats: {} ━━━", job).bold().magenta()
    );
    println!();

    // Query GryviaTrainingInsight for this job
    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1",
        "GryviaTrainingInsight",
    ));
    let api: Api<DynamicObject> = Api::namespaced_with(
        client.kube_client.clone(),
        namespace,
        &ar,
    );

    let label_selector = format!("gryvia.io/job={}", job);
    let params = ListParams::default().labels(&label_selector);

    let insights = match api.list(&params).await {
        Ok(list) => list,
        Err(e) => {
            println!(
                "  {} Could not query training insights: {}",
                "!".yellow().bold(),
                e
            );
            println!(
                "  {}",
                "Ensure a GryviaTrainingInsight CR exists for this job.".dimmed()
            );
            println!();
            return Ok(());
        }
    };

    if insights.items.is_empty() {
        // Also try to get by name directly
        match api.get(job).await {
            Ok(insight) => {
                print_nccl_stats(&insight);
            }
            Err(_) => {
                println!("  {}", "No NCCL stats found for this job.".dimmed());
                println!(
                    "  {}",
                    "Create a GryviaTrainingInsight CR targeting this job to collect stats.".dimmed()
                );
            }
        }
    } else {
        for insight in &insights.items {
            print_nccl_stats(insight);
        }
    }

    println!();
    Ok(())
}

fn print_nccl_stats(insight: &DynamicObject) {
    let status = insight.data.get("status");

    // Rank stats table
    let rank_stats = status
        .and_then(|s| s.get("rankStats"))
        .and_then(|v| v.as_array());

    if let Some(ranks) = rank_stats {
        let mut table = Table::new();
        table.set_format(*format::consts::FORMAT_BOX_CHARS);

        table.add_row(Row::new(vec![
            Cell::new("RANK").style_spec("Fb"),
            Cell::new("AVG LATENCY").style_spec("Fb"),
            Cell::new("TOTAL BYTES").style_spec("Fb"),
            Cell::new("STRAGGLER").style_spec("Fb"),
        ]));

        for rank in ranks {
            let rank_id = rank.get("rank").and_then(|v| v.as_i64()).unwrap_or(0);
            let avg_latency = rank.get("avgLatencyNs").and_then(|v| v.as_i64()).unwrap_or(0);
            let total_bytes = rank.get("totalBytes").and_then(|v| v.as_i64()).unwrap_or(0);
            let is_straggler = rank.get("isStraggler").and_then(|v| v.as_bool()).unwrap_or(false);

            let latency_display = format_ns(avg_latency);
            let bytes_display = format_bytes(total_bytes);
            let straggler_display = if is_straggler {
                "YES".red().bold().to_string()
            } else {
                "no".dimmed().to_string()
            };

            table.add_row(Row::new(vec![
                Cell::new(&rank_id.to_string()),
                Cell::new(&latency_display),
                Cell::new(&bytes_display),
                Cell::new(&straggler_display),
            ]));
        }

        table.printstd();
    } else {
        println!("  {}", "No rank statistics available.".dimmed());
    }

    // Communication pattern
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

    println!();
    println!("{}", "Communication Analysis:".bold().underline());
    println!("  Pattern:           {}", comm_pattern.bright_white());
    println!("  Comm/Compute:      {:.2}", ratio);

    let bottleneck_display = match bottleneck {
        "communication" => bottleneck.red().bold().to_string(),
        "compute" => bottleneck.yellow().to_string(),
        "data_loading" => bottleneck.blue().to_string(),
        _ => bottleneck.dimmed().to_string(),
    };
    println!("  Bottleneck:        {}", bottleneck_display);
}

async fn execute_memory(
    client: &GryviaClient,
    node: &str,
    _namespace: &str,
) -> Result<()> {
    println!(
        "{}",
        format!("━━━ GPU Memory Transfer Stats: {} ━━━", node).bold().magenta()
    );
    println!();

    // Query node GPU metrics
    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1",
        "GryviaGpuNode",
    ));
    let api: Api<DynamicObject> = Api::all_with(client.kube_client.clone(), &ar);

    match api.get(node).await {
        Ok(gpu_node) => {
            print_gpu_memory_stats(&gpu_node);
        }
        Err(_) => {
            println!("  {}", "GPU memory stats not yet available for this node.".dimmed());
            println!("  {}", "Ensure eBPF collectors are deployed on the target node.".dimmed());

            // Show placeholder stats
            println!();
            let mut table = Table::new();
            table.set_format(*format::consts::FORMAT_BOX_CHARS);

            table.add_row(Row::new(vec![
                Cell::new("DIRECTION").style_spec("Fb"),
                Cell::new("BANDWIDTH").style_spec("Fb"),
                Cell::new("COUNT").style_spec("Fb"),
                Cell::new("AVG LATENCY").style_spec("Fb"),
            ]));

            table.add_row(Row::new(vec![
                Cell::new("H2D (Host->Device)"),
                Cell::new("- GB/s"),
                Cell::new("-"),
                Cell::new("-"),
            ]));
            table.add_row(Row::new(vec![
                Cell::new("D2H (Device->Host)"),
                Cell::new("- GB/s"),
                Cell::new("-"),
                Cell::new("-"),
            ]));
            table.add_row(Row::new(vec![
                Cell::new("D2D (Device->Device)"),
                Cell::new("- GB/s"),
                Cell::new("-"),
                Cell::new("-"),
            ]));

            table.printstd();
        }
    }

    println!();
    Ok(())
}

fn print_gpu_memory_stats(node: &DynamicObject) {
    let status = node.data.get("status");

    let mut table = Table::new();
    table.set_format(*format::consts::FORMAT_BOX_CHARS);

    table.add_row(Row::new(vec![
        Cell::new("DIRECTION").style_spec("Fb"),
        Cell::new("BANDWIDTH").style_spec("Fb"),
        Cell::new("COUNT").style_spec("Fb"),
        Cell::new("AVG LATENCY").style_spec("Fb"),
    ]));

    let gpus = status
        .and_then(|s| s.get("gpus"))
        .and_then(|v| v.as_array());

    if let Some(gpu_list) = gpus {
        let total_gpus = gpu_list.len();
        // Show aggregate memory stats
        table.add_row(Row::new(vec![
            Cell::new("H2D (Host->Device)"),
            Cell::new(&format!("{} GPUs tracked", total_gpus)),
            Cell::new("-"),
            Cell::new("-"),
        ]));
    } else {
        table.add_row(Row::new(vec![
            Cell::new("H2D (Host->Device)"),
            Cell::new("- GB/s"),
            Cell::new("-"),
            Cell::new("-"),
        ]));
    }

    table.add_row(Row::new(vec![
        Cell::new("D2H (Device->Host)"),
        Cell::new("- GB/s"),
        Cell::new("-"),
        Cell::new("-"),
    ]));
    table.add_row(Row::new(vec![
        Cell::new("D2D (Device->Device)"),
        Cell::new("- GB/s"),
        Cell::new("-"),
        Cell::new("-"),
    ]));

    table.printstd();
}

async fn execute_rdma(
    _client: &GryviaClient,
    node: &str,
    _namespace: &str,
) -> Result<()> {
    println!(
        "{}",
        format!("━━━ RDMA Stats: {} ━━━", node).bold().magenta()
    );
    println!();

    let mut table = Table::new();
    table.set_format(*format::consts::FORMAT_BOX_CHARS);

    table.add_row(Row::new(vec![
        Cell::new("DEVICE").style_spec("Fb"),
        Cell::new("TX BYTES").style_spec("Fb"),
        Cell::new("RX BYTES").style_spec("Fb"),
        Cell::new("TX PACKETS").style_spec("Fb"),
        Cell::new("RX PACKETS").style_spec("Fb"),
        Cell::new("RETRIES").style_spec("Fb"),
    ]));

    // RDMA stats would come from the eBPF collector
    println!("  {}", "RDMA statistics require eBPF collector to be deployed on the target node.".dimmed());
    println!("  {}", "Ensure InfiniBand/RoCE devices are available and monitored.".dimmed());

    println!();
    Ok(())
}

async fn execute_training(
    client: &GryviaClient,
    job: &str,
    namespace: &str,
) -> Result<()> {
    println!(
        "{}",
        format!("━━━ Training Insights: {} ━━━", job).bold().magenta()
    );
    println!();

    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
        "gryvia.io",
        "v1",
        "GryviaTrainingInsight",
    ));
    let api: Api<DynamicObject> = Api::namespaced_with(
        client.kube_client.clone(),
        namespace,
        &ar,
    );

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
                    println!("  {}", "No training insights found for this job.".dimmed());
                    println!();
                    return Ok(());
                }
            }
        }
    };

    let status = insight.data.get("status");

    // Straggler detection
    let stragglers = status
        .and_then(|s| s.get("stragglers"))
        .and_then(|v| v.as_array());

    println!("{}", "Straggler Detection:".bold().underline());
    if let Some(strag_list) = stragglers {
        if strag_list.is_empty() {
            println!("  {}", "No stragglers detected.".green());
        } else {
            for straggler in strag_list {
                let rank = straggler.get("rank").and_then(|v| v.as_i64()).unwrap_or(0);
                let factor = straggler.get("slowdownFactor").and_then(|v| v.as_f64()).unwrap_or(0.0);
                let reason = straggler.get("reason").and_then(|v| v.as_str()).unwrap_or("-");
                println!(
                    "  {} Rank {} is {:.1}x slower ({})",
                    "!".red().bold(),
                    rank.to_string().bright_white(),
                    factor,
                    reason
                );
            }
        }
    } else {
        println!("  {}", "No straggler data available.".dimmed());
    }
    println!();

    // Communication pattern
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

    println!("{}", "Communication Analysis:".bold().underline());
    println!("  Pattern:           {}", comm_pattern.bright_white());
    println!("  Comm/Compute:      {:.2}", ratio);

    let bottleneck_display = match bottleneck {
        "communication" => format!("{} - Network is the primary bottleneck", bottleneck.red().bold()),
        "compute" => format!("{} - GPU compute is the primary bottleneck", bottleneck.yellow()),
        "data_loading" => format!("{} - Data loading is the primary bottleneck", bottleneck.blue()),
        _ => bottleneck.dimmed().to_string(),
    };
    println!("  Bottleneck:        {}", bottleneck_display);

    let last_analysis = status
        .and_then(|s| s.get("lastAnalysis"))
        .and_then(|v| v.as_str())
        .unwrap_or("-");
    println!("  Last Analysis:     {}", last_analysis.dimmed());

    println!();
    Ok(())
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
