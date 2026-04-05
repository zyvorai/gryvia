use anyhow::{Context, Result};
use kube::api::Api;
use colored::*;

use crate::client::KubeFabricClient;
use crate::types::*;

pub async fn execute(client: &KubeFabricClient, job: &str, _follow: bool) -> Result<()> {
    let api: Api<FabricAIJob> = Api::namespaced(
        client.kube_client.clone(),
        client.namespace(),
    );

    let job_obj = api.get(job).await
        .context("Failed to get job")?;

    print_job_status(&job_obj);

    Ok(())
}

fn print_job_status(job: &FabricAIJob) {
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
        println!("  World Size: {}", job.spec.distributed.world_size);
        println!();
    }

    println!("{}", "Status:".bold().underline());
    let phase = &job.status.phase;
    let colored_phase = match phase.as_str() {
        "Running" => phase.green(),
        "Completed" => phase.cyan(),
        "Failed" => phase.red(),
        "Pending" | "Queued" => phase.yellow(),
        _ => phase.normal(),
    };
    println!("  Phase: {}", colored_phase.bold());

    if !job.status.message.is_empty() {
        println!("  Message: {}", job.status.message);
    }

    if let Some(start) = job.status.start_time {
        println!("  Started: {}", start);

        if let Some(completion) = job.status.completion_time {
            println!("  Completed: {}", completion);
            let duration = completion.signed_duration_since(start);
            println!("  Duration: {}h {}m {}s",
                duration.num_hours(),
                duration.num_minutes() % 60,
                duration.num_seconds() % 60
            );
        } else if phase == "Running" {
            let duration = chrono::Utc::now().signed_duration_since(start);
            println!("  Running for: {}h {}m {}s",
                duration.num_hours(),
                duration.num_minutes() % 60,
                duration.num_seconds() % 60
            );
        }
    }

    println!();
}
