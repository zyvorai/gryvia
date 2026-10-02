use anyhow::{Context, Result};
use colored::Colorize;
use futures::{AsyncBufReadExt, StreamExt};
use indicatif::{ProgressBar, ProgressStyle};
use k8s_openapi::api::core::v1::Pod;
use kube::api::{Api, ApiResource, ListParams, LogParams, PostParams};
use kube::core::DynamicObject;
use serde_yaml;
use std::fs;
use std::path::Path;
use tokio::time::{sleep, Duration};

use super::sbatch;
use crate::client::GryviaClient;
use crate::display;

pub async fn execute(
    client: &GryviaClient,
    file: &str,
    wait: bool,
    follow_logs: bool,
) -> Result<()> {
    // Read and parse YAML file
    let contents =
        fs::read_to_string(file).with_context(|| format!("Failed to read file: {}", file))?;

    let job: DynamicObject =
        serde_yaml::from_str(&contents).context("Failed to parse YAML file")?;

    let job_name = job
        .metadata
        .name
        .clone()
        .unwrap_or_else(|| "unknown".to_string());

    display::print_info(&format!("Submitting job: {}", job_name));

    // Submit job
    let ar = ApiResource::from_gvk(&kube::api::GroupVersionKind::gvk(
        "gryvia.io",
        "v1alpha1",
        "GryviaAIJob",
    ));
    let api: Api<DynamicObject> =
        Api::namespaced_with(client.kube_client.clone(), client.namespace(), &ar);

    let _result = api
        .create(&PostParams::default(), &job)
        .await
        .context("Failed to submit job")?;

    display::print_success(&format!("Job {} submitted successfully", job_name));

    if wait {
        display::print_info("Waiting for job to complete...");
        wait_for_completion(client, &job_name).await?;
    }

    if follow_logs {
        display::print_info("Waiting for pods to start...");
        follow_job_logs(client, &job_name).await?;
    }

    Ok(())
}

fn read_sbatch(
    script: &str,
    image: Option<String>,
    namespace: Option<String>,
) -> Result<sbatch::Imported> {
    let contents =
        fs::read_to_string(script).with_context(|| format!("Failed to read file: {}", script))?;
    let stem = Path::new(script)
        .file_stem()
        .and_then(|s| s.to_str())
        .unwrap_or("sbatch-job")
        .to_string();
    let out = sbatch::import(
        &contents,
        &sbatch::Options {
            image,
            default_name: stem,
            namespace,
        },
    )
    .with_context(|| format!("Cannot import {}", script))?;
    for w in &out.warnings {
        // stderr, so the --dry-run YAML on stdout can be piped to kubectl
        eprintln!("{} {}", "⚠".yellow().bold(), w);
    }
    Ok(out)
}

/// `gryvia submit --sbatch SCRIPT --dry-run`: the GryviaAIJobs as YAML on stdout, warnings on stderr.
pub fn print_sbatch(script: &str, image: Option<String>, namespace: Option<String>) -> Result<()> {
    let out = read_sbatch(script, image, namespace)?;
    for job in &out.jobs {
        print!("---\n{}", serde_yaml::to_string(job)?);
    }
    Ok(())
}

/// `gryvia submit --sbatch SCRIPT`: submits one GryviaAIJob (one per array index).
pub async fn execute_sbatch(
    client: &GryviaClient,
    script: &str,
    image: Option<String>,
    wait: bool,
    follow_logs: bool,
) -> Result<()> {
    let out = read_sbatch(script, image, Some(client.namespace().to_string()))?;
    let ar = ApiResource::from_gvk(&kube::api::GroupVersionKind::gvk(
        "gryvia.io",
        "v1alpha1",
        "GryviaAIJob",
    ));
    let api: Api<DynamicObject> =
        Api::namespaced_with(client.kube_client.clone(), client.namespace(), &ar);
    let mut names = Vec::new();
    for job in out.jobs {
        let obj: DynamicObject = serde_json::from_value(job).context("Invalid GryviaAIJob")?;
        let name = obj.metadata.name.clone().unwrap_or_default();
        api.create(&PostParams::default(), &obj)
            .await
            .with_context(|| format!("Failed to submit job {}", name))?;
        display::print_success(&format!("Job {} submitted", name));
        names.push(name);
    }
    if wait {
        for name in &names {
            display::print_info(&format!("Waiting for {} to complete...", name));
            wait_for_completion(client, name).await?;
        }
    }
    if follow_logs {
        if names.len() > 1 {
            display::print_info("Following the logs of the first array job only");
        }
        follow_job_logs(client, &names[0]).await?;
    }
    Ok(())
}

async fn wait_for_completion(client: &GryviaClient, job_name: &str) -> Result<()> {
    let spinner = ProgressBar::new_spinner();
    spinner.set_style(
        ProgressStyle::default_spinner()
            .template("{spinner:.green} {msg}")
            .expect("valid spinner template"),
    );

    let ar = ApiResource::from_gvk(&kube::api::GroupVersionKind::gvk(
        "gryvia.io",
        "v1alpha1",
        "GryviaAIJob",
    ));
    let api: Api<DynamicObject> =
        Api::namespaced_with(client.kube_client.clone(), client.namespace(), &ar);

    let timeout = Duration::from_secs(24 * 60 * 60); // 24 hour timeout
    let start = tokio::time::Instant::now();

    loop {
        if start.elapsed() > timeout {
            spinner.finish_with_message(format!(
                "✗ Job {} timed out waiting for completion",
                job_name
            ));
            return Err(anyhow::anyhow!(
                "Timed out waiting for job {} to complete after 24h",
                job_name
            ));
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
                        let error_msg = status
                            .get("message")
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

async fn follow_job_logs(client: &GryviaClient, job_name: &str) -> Result<()> {
    let pods_api: Api<Pod> = Api::namespaced(client.kube_client.clone(), client.namespace());

    let label_selector = format!("gryvia.io/job={}", job_name);
    let lp = ListParams::default().labels(&label_selector);

    // Wait for pods to appear (up to 5 minutes)
    let timeout = Duration::from_secs(300);
    let start = tokio::time::Instant::now();
    let pod_name;

    loop {
        if start.elapsed() > timeout {
            display::print_warning("Timed out waiting for pods to start");
            return Ok(());
        }

        let pods = pods_api.list(&lp).await?;
        if let Some(pod) = pods.items.first() {
            if let Some(name) = &pod.metadata.name {
                pod_name = name.clone();
                break;
            }
        }

        sleep(Duration::from_secs(3)).await;
    }

    display::print_info(&format!("Following logs from pod: {}", pod_name));

    // Detect container name from the pod spec, defaulting to "trainer"
    let pods = pods_api.list(&lp).await?;
    let container_name = pods
        .items
        .first()
        .and_then(|p| p.spec.as_ref())
        .and_then(|s| s.containers.first())
        .map(|c| c.name.clone())
        .unwrap_or_else(|| "trainer".to_string());

    let log_params = LogParams {
        follow: true,
        container: Some(container_name),
        ..Default::default()
    };

    let stream = pods_api
        .log_stream(&pod_name, &log_params)
        .await
        .context("Failed to start log stream")?;

    let mut lines = stream.lines();
    while let Some(line) = lines.next().await {
        match line {
            Ok(l) => println!("{}", l),
            Err(e) => {
                display::print_error(&format!("Log stream error: {}", e));
                break;
            }
        }
    }

    Ok(())
}
