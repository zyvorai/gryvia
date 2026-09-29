use anyhow::{Context, Result};
use colored::*;
use kube::api::Api;
use tokio::time::{sleep, Duration};

use crate::client::GryviaClient;
use crate::types::*;

pub async fn execute(client: &GryviaClient, job: &str, follow: bool) -> Result<()> {
    if follow {
        loop {
            // Clear screen using ANSI escape sequences. This works on POSIX-compliant
            // terminals but may render as garbage on non-ANSI terminals (e.g. Windows cmd.exe
            // without virtual terminal processing enabled).
            print!("\x1B[2J\x1B[1;1H");
            let api: Api<GryviaAIJob> =
                Api::namespaced(client.kube_client.clone(), client.namespace());

            let job_obj = api.get(job).await.context("Failed to get job")?;

            print_job_status(&job_obj);

            let status = job_obj.status.clone().unwrap_or_default();
            match status.phase.as_str() {
                "Completed" | "Succeeded" | "Failed" => {
                    break;
                }
                _ => {}
            }

            sleep(Duration::from_secs(5)).await;
        }
    } else {
        let api: Api<GryviaAIJob> = Api::namespaced(client.kube_client.clone(), client.namespace());

        let job_obj = api.get(job).await.context("Failed to get job")?;

        print_job_status(&job_obj);
    }

    Ok(())
}

fn print_job_status(job: &GryviaAIJob) {
    let unknown = "<unknown>".to_string();
    let name = job.metadata.name.as_ref().unwrap_or(&unknown);

    println!("{}", format!("━━━ Job: {} ━━━", name).bold().cyan());
    println!();

    println!("{} {}", "Framework:".bold(), job.spec.framework);
    println!("{} {}", "Image:".bold(), job.spec.image);
    println!();

    println!("{}", "Resources:".bold().underline());
    println!("  GPU Type: {}", job.spec.resources.gpu_type);
    println!("  GPU Count: {}", job.spec.resources.gpu_count);
    println!("  Memory: {}", job.spec.resources.memory);
    println!("  CPU: {}", job.spec.resources.cpu);
    println!();

    if job.spec.distributed.enabled {
        println!("{}", "Distributed Training:".bold().underline());
        println!("  Strategy: {}", job.spec.distributed.strategy);
        println!("  Nodes: {}", job.spec.distributed.nodes);
        println!("  GPUs/Node: {}", job.spec.distributed.gpus_per_node);
        println!();
    }

    let status = job.status.clone().unwrap_or_default();

    println!("{}", "Status:".bold().underline());
    let phase = &status.phase;
    let colored_phase = match phase.as_str() {
        "Running" => phase.green(),
        "Completed" => phase.cyan(),
        "Failed" => phase.red(),
        "Pending" | "Queued" => phase.yellow(),
        _ => phase.normal(),
    };
    println!("  Phase: {}", colored_phase.bold());

    if !status.message.is_empty() {
        println!("  Message: {}", status.message);
    }

    if let Some(start) = status.start_time {
        println!("  Started: {}", start);

        if let Some(completion) = status.completion_time {
            println!("  Completed: {}", completion);
            let duration = completion.signed_duration_since(start);
            println!(
                "  Duration: {}h {}m {}s",
                duration.num_hours(),
                duration.num_minutes() % 60,
                duration.num_seconds() % 60
            );
        } else if phase == "Running" {
            let duration = chrono::Utc::now().signed_duration_since(start);
            println!(
                "  Running for: {}h {}m {}s",
                duration.num_hours(),
                duration.num_minutes() % 60,
                duration.num_seconds() % 60
            );
        }
    }

    println!();
}
