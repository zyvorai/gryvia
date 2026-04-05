use anyhow::{Context, Result};
use kube::api::{Api, PostParams};
use kube::core::DynamicObject;
use serde_yaml;
use std::fs;
use indicatif::{ProgressBar, ProgressStyle};
use tokio::time::{sleep, Duration};

use crate::client::KubeFabricClient;
use crate::display;

pub async fn execute(client: &KubeFabricClient, file: &str, wait: bool, follow_logs: bool) -> Result<()> {
    // Read and parse YAML file
    let contents = fs::read_to_string(file)
        .with_context(|| format!("Failed to read file: {}", file))?;

    let job: DynamicObject = serde_yaml::from_str(&contents)
        .context("Failed to parse YAML file")?;

    let job_name = job.metadata.name.clone().unwrap_or_else(|| "unknown".to_string());

    display::print_info(&format!("Submitting job: {}", job_name));

    // Submit job
    let api: Api<DynamicObject> = Api::namespaced(
        client.kube_client.clone(),
        client.namespace(),
    );

    let _result = api.create(&PostParams::default(), &job).await
        .context("Failed to submit job")?;

    display::print_success(&format!("Job {} submitted successfully", job_name));

    if wait {
        display::print_info("Waiting for job to complete...");
        wait_for_completion(client, &job_name).await?;
    }

    if follow_logs {
        // TODO: Implement log following
        display::print_info("Log following not yet implemented");
    }

    Ok(())
}

async fn wait_for_completion(client: &KubeFabricClient, job_name: &str) -> Result<()> {
    let spinner = ProgressBar::new_spinner();
    spinner.set_style(
        ProgressStyle::default_spinner()
            .template("{spinner:.green} {msg}")
            .unwrap()
    );

    let api: Api<DynamicObject> = Api::namespaced(
        client.kube_client.clone(),
        client.namespace(),
    );

    let timeout = Duration::from_secs(24 * 60 * 60); // 24 hour timeout
    let start = tokio::time::Instant::now();

    loop {
        if start.elapsed() > timeout {
            spinner.finish_with_message(format!("✗ Job {} timed out waiting for completion", job_name));
            return Err(anyhow::anyhow!("Timed out waiting for job {} to complete after 24h", job_name));
        }

        spinner.set_message(format!("Checking status of {}", job_name));

        let job = api.get(job_name).await?;

        if let Some(status) = job.data.get("status") {
            if let Some(phase) = status.get("phase") {
                let phase_str = phase.as_str().unwrap_or("Unknown");

                match phase_str {
                    "Completed" | "Succeeded" => {
                        spinner.finish_with_message(format!("✓ Job {} completed", job_name));
                        return Ok(());
                    }
                    "Failed" => {
                        spinner.finish_with_message(format!("✗ Job {} failed", job_name));
                        let error_msg = status.get("message")
                            .and_then(|m| m.as_str())
                            .unwrap_or("unknown error");
                        display::print_error(&format!("Error: {}", error_msg));
                        return Err(anyhow::anyhow!("Job {} failed: {}", job_name, error_msg));
                    }
                    _ => {
                        spinner.set_message(format!("Job status: {}", phase_str));
                    }
                }
            }
        }

        sleep(Duration::from_secs(5)).await;
    }
}
