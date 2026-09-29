use anyhow::{Context, Result};
use colored::*;
use kube::api::{Api, ListParams};
use prettytable::{format, Cell, Row, Table};
use tokio::time::{sleep, Duration};

use crate::client::GryviaClient;
use crate::display;
use crate::types::*;

pub async fn execute(
    client: &GryviaClient,
    name: Option<String>,
    watch: Option<u64>,
) -> Result<()> {
    if let Some(interval) = watch {
        if interval == 0 {
            anyhow::bail!("Watch interval must be greater than 0");
        }
        loop {
            print!("\x1B[2J\x1B[1;1H"); // Clear screen
            if let Err(e) = show_queue(client, &name).await {
                eprintln!("Error refreshing queue: {}", e);
            }
            sleep(Duration::from_secs(interval)).await;
        }
    } else {
        show_queue(client, &name).await?;
    }

    Ok(())
}

async fn show_queue(client: &GryviaClient, name: &Option<String>) -> Result<()> {
    let api: Api<GryviaAIJob> = Api::all(client.kube_client.clone());

    let jobs = api
        .list(&ListParams::default())
        .await
        .context("Failed to list jobs")?;

    // Filter to pending/queued/scheduling jobs
    let queued: Vec<_> = jobs
        .items
        .iter()
        .filter(|j| {
            let phase = j.status.as_ref().map(|s| s.phase.as_str()).unwrap_or("");
            matches!(phase, "Pending" | "Queued" | "Scheduling")
        })
        .filter(|j| {
            if let Some(ref filter) = name {
                j.metadata
                    .name
                    .as_deref()
                    .unwrap_or("")
                    .contains(filter.as_str())
            } else {
                true
            }
        })
        .collect();

    let running: Vec<_> = jobs
        .items
        .iter()
        .filter(|j| j.status.as_ref().map(|s| s.phase.as_str()) == Some("Running"))
        .collect();

    println!("{}", "━━━ Job Queue ━━━".bold().cyan());
    println!();

    // Summary
    println!("{}", "Queue Summary:".bold());
    println!(
        "  Queued/Pending: {}",
        queued.len().to_string().yellow().bold()
    );
    println!(
        "  Running:        {}",
        running.len().to_string().green().bold()
    );
    println!();

    if queued.is_empty() {
        display::print_info("No jobs in queue");
        return Ok(());
    }

    // Queue table
    println!("{}", "Queued Jobs:".bold().underline());

    let mut table = Table::new();
    table.set_format(*format::consts::FORMAT_BOX_CHARS);

    table.add_row(Row::new(vec![
        Cell::new("POSITION").style_spec("Fb"),
        Cell::new("NAME").style_spec("Fb"),
        Cell::new("FRAMEWORK").style_spec("Fb"),
        Cell::new("GPUs").style_spec("Fb"),
        Cell::new("GPU TYPE").style_spec("Fb"),
        Cell::new("STATUS").style_spec("Fb"),
        Cell::new("WAITING").style_spec("Fb"),
    ]));

    let unknown = "<unknown>".to_string();
    for (i, job) in queued.iter().enumerate() {
        let name = job.metadata.name.as_ref().unwrap_or(&unknown);
        let framework = &job.spec.framework;
        let gpus = job.spec.resources.gpu_count.to_string();
        let gpu_type = &job.spec.resources.gpu_type;
        let job_status = job.status.clone().unwrap_or_default();
        let status = display::colorize_status(&job_status.phase);

        let waiting = if let Some(ref ts) = job.metadata.creation_timestamp {
            let age = display::age_since(ts);
            if age.num_hours() > 0 {
                format!("{}h{}m", age.num_hours(), age.num_minutes() % 60)
            } else if age.num_minutes() > 0 {
                format!("{}m", age.num_minutes())
            } else {
                format!("{}s", age.num_seconds())
            }
        } else {
            "-".to_string()
        };

        table.add_row(Row::new(vec![
            Cell::new(&format!("#{}", i + 1)),
            Cell::new(name),
            Cell::new(framework),
            Cell::new(&gpus),
            Cell::new(gpu_type),
            Cell::new(&status),
            Cell::new(&waiting),
        ]));
    }

    table.printstd();
    println!();

    // Running jobs summary
    if !running.is_empty() {
        println!("{}", "Running Jobs:".bold().underline());

        let mut table = Table::new();
        table.set_format(*format::consts::FORMAT_BOX_CHARS);

        table.add_row(Row::new(vec![
            Cell::new("NAME").style_spec("Fb"),
            Cell::new("GPUs").style_spec("Fb"),
            Cell::new("GPU TYPE").style_spec("Fb"),
            Cell::new("RUNNING FOR").style_spec("Fb"),
        ]));

        for job in &running {
            let name = job.metadata.name.as_ref().unwrap_or(&unknown);
            let gpus = job.spec.resources.gpu_count.to_string();
            let gpu_type = &job.spec.resources.gpu_type;
            let job_status = job.status.clone().unwrap_or_default();

            let running_for = if let Some(start) = job_status.start_time {
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
                Cell::new(&gpus),
                Cell::new(gpu_type),
                Cell::new(&running_for),
            ]));
        }

        table.printstd();
        println!();
    }

    // GPU utilization
    let total_queued_gpus: u32 = queued.iter().map(|j| j.spec.resources.gpu_count).sum();
    let total_running_gpus: u32 = running.iter().map(|j| j.spec.resources.gpu_count).sum();

    println!("{}", "GPU Demand:".bold());
    println!(
        "  Running:  {} GPUs",
        total_running_gpus.to_string().green()
    );
    println!(
        "  Queued:   {} GPUs",
        total_queued_gpus.to_string().yellow()
    );
    println!(
        "  Total:    {} GPUs",
        (total_running_gpus + total_queued_gpus).to_string().bold()
    );

    Ok(())
}
