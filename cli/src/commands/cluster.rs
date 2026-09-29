use anyhow::{Context, Result};
use colored::*;
use kube::api::{Api, ListParams};
use prettytable::{format, Cell, Row, Table};
use tokio::time::{sleep, Duration};

use crate::client::GryviaClient;
use crate::display;
use crate::types::*;

pub async fn execute(client: &GryviaClient, detailed: bool, watch: Option<u64>) -> Result<()> {
    if let Some(interval) = watch {
        if interval == 0 {
            anyhow::bail!("Watch interval must be greater than 0");
        }
        loop {
            // Clear screen using ANSI escape sequences. This works on POSIX-compliant
            // terminals but may render as garbage on non-ANSI terminals (e.g. Windows cmd.exe
            // without virtual terminal processing enabled).
            print!("\x1B[2J\x1B[1;1H");
            if let Err(e) = show_cluster_overview(client, detailed).await {
                eprintln!("Error refreshing cluster overview: {}", e);
            }
            sleep(Duration::from_secs(interval)).await;
        }
    } else {
        show_cluster_overview(client, detailed).await?;
    }

    Ok(())
}

async fn show_cluster_overview(client: &GryviaClient, detailed: bool) -> Result<()> {
    let nodes_api: Api<GryviaGpuNode> = Api::all(client.kube_client.clone());
    let jobs_api: Api<GryviaAIJob> = Api::all(client.kube_client.clone());

    let nodes = nodes_api
        .list(&ListParams::default())
        .await
        .context("Failed to list GPU nodes")?;

    let jobs = jobs_api
        .list(&ListParams::default())
        .await
        .context("Failed to list jobs")?;

    display::print_cluster_overview(&nodes.items, &jobs.items);

    if detailed {
        show_detailed_nodes(&nodes.items)?;
        show_detailed_jobs(&jobs.items)?;
    }

    Ok(())
}

fn show_detailed_nodes(nodes: &[GryviaGpuNode]) -> Result<()> {
    println!("{}", "━━━ GPU Node Details ━━━".bold().cyan());
    println!();

    if nodes.is_empty() {
        display::print_info("No GPU nodes registered");
        return Ok(());
    }

    let mut table = Table::new();
    table.set_format(*format::consts::FORMAT_BOX_CHARS);

    table.add_row(Row::new(vec![
        Cell::new("NODE").style_spec("Fb"),
        Cell::new("GPU TYPE").style_spec("Fb"),
        Cell::new("GPUs").style_spec("Fb"),
        Cell::new("MEMORY").style_spec("Fb"),
        Cell::new("RDMA").style_spec("Fb"),
        Cell::new("STATUS").style_spec("Fb"),
    ]));

    for node in nodes {
        let name = &node.spec.node_name;
        let gpu_type = &node.spec.gpu_type;
        let gpu_count = node.spec.gpu_count.to_string();
        let memory = &node.spec.memory;
        let rdma = if node.spec.rdma_enabled {
            "yes".green().to_string()
        } else {
            "no".normal().to_string()
        };
        let node_status = node.status.clone().unwrap_or_default();
        let status = display::colorize_status(&node_status.phase);

        table.add_row(Row::new(vec![
            Cell::new(name),
            Cell::new(gpu_type),
            Cell::new(&gpu_count),
            Cell::new(memory),
            Cell::new(&rdma),
            Cell::new(&status),
        ]));
    }

    table.printstd();
    println!();

    // Per-node GPU details
    for node in nodes {
        let node_status = node.status.clone().unwrap_or_default();
        if node_status.gpus.is_empty() {
            continue;
        }

        println!("  {} GPU Details:", node.spec.node_name.bold());

        let mut gpu_table = Table::new();
        gpu_table.set_format(*format::consts::FORMAT_BOX_CHARS);

        gpu_table.add_row(Row::new(vec![
            Cell::new("INDEX").style_spec("Fb"),
            Cell::new("UUID").style_spec("Fb"),
            Cell::new("TEMP").style_spec("Fb"),
            Cell::new("UTIL%").style_spec("Fb"),
            Cell::new("MEM USED").style_spec("Fb"),
            Cell::new("MEM TOTAL").style_spec("Fb"),
        ]));

        for gpu in &node_status.gpus {
            let temp_str = format!("{}C", gpu.temperature);
            let temp_colored = if gpu.temperature >= 90.0 {
                temp_str.red().to_string()
            } else if gpu.temperature >= 80.0 {
                temp_str.yellow().to_string()
            } else {
                temp_str.green().to_string()
            };

            let util_str = format!("{:.0}%", gpu.utilization);
            let util_colored = if gpu.utilization >= 90.0 {
                util_str.green().to_string()
            } else if gpu.utilization >= 50.0 {
                util_str.yellow().to_string()
            } else {
                util_str.normal().to_string()
            };

            gpu_table.add_row(Row::new(vec![
                Cell::new(&gpu.index.to_string()),
                Cell::new(&gpu.uuid.chars().take(12).collect::<String>()),
                Cell::new(&temp_colored),
                Cell::new(&util_colored),
                Cell::new(&format!("{}MB", gpu.memory_used)),
                Cell::new(&format!("{}MB", gpu.memory_total)),
            ]));
        }

        gpu_table.printstd();
        println!();
    }

    Ok(())
}

fn show_detailed_jobs(jobs: &[GryviaAIJob]) -> Result<()> {
    println!("{}", "━━━ Job Details ━━━".bold().cyan());
    println!();

    let running: Vec<_> = jobs
        .iter()
        .filter(|j| j.status.as_ref().map(|s| s.phase.as_str()) == Some("Running"))
        .collect();

    if running.is_empty() {
        display::print_info("No running jobs");
        return Ok(());
    }

    let mut table = Table::new();
    table.set_format(*format::consts::FORMAT_BOX_CHARS);

    table.add_row(Row::new(vec![
        Cell::new("NAME").style_spec("Fb"),
        Cell::new("FRAMEWORK").style_spec("Fb"),
        Cell::new("GPUs").style_spec("Fb"),
        Cell::new("GPU TYPE").style_spec("Fb"),
        Cell::new("DISTRIBUTED").style_spec("Fb"),
        Cell::new("RUNTIME").style_spec("Fb"),
    ]));

    let unknown = "<unknown>".to_string();
    for job in running {
        let name = job.metadata.name.as_ref().unwrap_or(&unknown);
        let framework = &job.spec.framework;
        let gpus = job.spec.resources.gpu_count.to_string();
        let gpu_type = &job.spec.resources.gpu_type;
        let distributed = if job.spec.distributed.enabled {
            format!(
                "{} ({}x{})",
                job.spec.distributed.strategy,
                job.spec.distributed.nodes,
                job.spec.distributed.gpus_per_node
            )
        } else {
            "no".to_string()
        };
        let status = job.status.clone().unwrap_or_default();
        let runtime = if let Some(start) = status.start_time {
            let duration = chrono::Utc::now().signed_duration_since(start);
            if duration.num_hours() > 0 {
                format!("{}h{}m", duration.num_hours(), duration.num_minutes() % 60)
            } else {
                format!("{}m", duration.num_minutes())
            }
        } else {
            "-".to_string()
        };

        table.add_row(Row::new(vec![
            Cell::new(name),
            Cell::new(framework),
            Cell::new(&gpus),
            Cell::new(gpu_type),
            Cell::new(&distributed),
            Cell::new(&runtime),
        ]));
    }

    table.printstd();
    println!();

    Ok(())
}
