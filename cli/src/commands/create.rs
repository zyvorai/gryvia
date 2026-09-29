use anyhow::{Context, Result};
use dialoguer::{Input, Select, Confirm};
use kube::api::{Api, PostParams};
use serde_json::json;

use crate::client::GryviaClient;
use crate::display;

pub async fn execute(client: &GryviaClient, resource: &str) -> Result<()> {
    match resource {
        "job" => create_job(client).await?,
        "quota" => create_quota(client).await?,
        _ => {
            anyhow::bail!("Unknown resource type: {}. Valid types: job, quota", resource);
        }
    }

    Ok(())
}

async fn create_job(client: &GryviaClient) -> Result<()> {
    display::print_info("Interactive Job Creation Wizard");
    println!();

    let name: String = Input::new()
        .with_prompt("Job name")
        .validate_with(|input: &String| -> Result<(), &str> {
            if input.is_empty() {
                return Err("Name cannot be empty");
            }
            if input.len() > 253 {
                return Err("Name must be 253 characters or fewer");
            }
            if input.starts_with('-') || input.ends_with('-') {
                return Err("Name must not start or end with a hyphen");
            }
            if !input.chars().all(|c| c.is_ascii_lowercase() || c.is_ascii_digit() || c == '-') {
                return Err("Name must contain only lowercase letters, digits, and hyphens");
            }
            Ok(())
        })
        .interact_text()?;

    let frameworks = vec!["pytorch", "tensorflow", "horovod", "deepspeed", "megatron"];
    let framework_idx = Select::new()
        .with_prompt("Framework")
        .items(&frameworks)
        .default(0)
        .interact()?;
    let framework = frameworks[framework_idx].to_string();

    let image: String = Input::new()
        .with_prompt("Container image")
        .default("nvcr.io/nvidia/pytorch:24.01-py3".to_string())
        .validate_with(|input: &String| -> Result<(), &str> {
            if input.trim().is_empty() {
                Err("Image cannot be empty")
            } else if input.contains(' ') {
                Err("Image cannot contain spaces")
            } else {
                Ok(())
            }
        })
        .interact_text()?;

    let gpu_types = vec!["H100", "A100-80G", "A100-40G", "L40", "A10", "V100", "T4", "any"];
    let gpu_type_idx = Select::new()
        .with_prompt("GPU type")
        .items(&gpu_types)
        .default(0)
        .interact()?;
    let gpu_type = gpu_types[gpu_type_idx].to_string();

    let gpu_count: u32 = Input::new()
        .with_prompt("Number of GPUs")
        .default(1)
        .interact_text()?;

    if gpu_count == 0 {
        anyhow::bail!("GPU count must be at least 1");
    }

    let memory: String = Input::new()
        .with_prompt("Memory (e.g., 64Gi)")
        .default("32Gi".to_string())
        .validate_with(|input: &String| -> Result<(), &str> {
            let trimmed = input.trim();
            if trimmed.is_empty() {
                return Err("Memory cannot be empty");
            }
            // Must start with a digit
            if !trimmed.chars().next().map_or(false, |c| c.is_ascii_digit()) {
                return Err("Memory must start with a number (e.g., 32Gi)");
            }
            Ok(())
        })
        .interact_text()?;

    let cpu: u32 = Input::new()
        .with_prompt("CPUs")
        .default(8)
        .interact_text()?;

    let command: String = Input::new()
        .with_prompt("Command (e.g., python train.py)")
        .validate_with(|input: &String| -> Result<(), &str> {
            if input.trim().is_empty() {
                return Err("Command cannot be empty");
            }
            Ok(())
        })
        .interact_text()?;

    let distributed = Confirm::new()
        .with_prompt("Enable distributed training?")
        .default(false)
        .interact()?;

    let mut dist_config = json!({ "enabled": false });
    if distributed {
        let strategies = vec!["ddp", "fsdp", "deepspeed", "horovod"];
        let strategy_idx = Select::new()
            .with_prompt("Distribution strategy")
            .items(&strategies)
            .default(0)
            .interact()?;

        let num_nodes: u32 = Input::new()
            .with_prompt("Number of nodes")
            .default(1)
            .interact_text()?;

        if num_nodes < 1 {
            anyhow::bail!("Number of nodes must be at least 1");
        }

        let gpus_per_node: u32 = Input::new()
            .with_prompt("GPUs per node")
            .default(gpu_count)
            .interact_text()?;

        dist_config = json!({
            "enabled": true,
            "strategy": strategies[strategy_idx],
            "nodes": num_nodes,
            "gpusPerNode": gpus_per_node,
        });
    }

    // Parse command into parts
    let cmd_parts: Vec<&str> = command.split_whitespace().collect();

    let job_spec = json!({
        "apiVersion": "gryvia.io/v1",
        "kind": "GryviaAIJob",
        "metadata": {
            "name": name,
            "namespace": client.namespace(),
        },
        "spec": {
            "framework": framework,
            "image": image,
            "command": cmd_parts,
            "resources": {
                "gpuType": gpu_type,
                "gpuCount": gpu_count,
                "memory": memory,
                "cpu": cpu,
            },
            "distributed": dist_config,
        }
    });

    println!();
    display::print_info("Job configuration:");
    println!("{}", serde_json::to_string_pretty(&job_spec)?);
    println!();

    let confirm = Confirm::new()
        .with_prompt("Submit this job?")
        .default(true)
        .interact()?;

    if !confirm {
        display::print_info("Cancelled");
        return Ok(());
    }

    let ar = kube::api::ApiResource::from_gvk(
        &kube::api::GroupVersionKind::gvk("gryvia.io", "v1", "GryviaAIJob"),
    );
    let api: Api<kube::core::DynamicObject> = Api::namespaced_with(
        client.kube_client.clone(),
        client.namespace(),
        &ar,
    );

    let job_obj: kube::core::DynamicObject = serde_json::from_value(job_spec)
        .context("Failed to construct job object")?;

    api.create(&PostParams::default(), &job_obj).await
        .context("Failed to create job")?;

    display::print_success(&format!("Job '{}' created successfully", name));

    Ok(())
}

async fn create_quota(client: &GryviaClient) -> Result<()> {
    display::print_info("Interactive Quota Creation Wizard");
    println!();

    let team: String = Input::new()
        .with_prompt("Team name")
        .validate_with(|input: &String| -> Result<(), &str> {
            let trimmed = input.trim();
            if trimmed.is_empty() {
                return Err("Team name cannot be empty");
            }
            if !trimmed.chars().all(|c| c.is_alphanumeric() || c == '-' || c == '_') {
                return Err("Team name must only contain alphanumeric characters, hyphens, or underscores");
            }
            Ok(())
        })
        .interact_text()?;

    let namespaces: String = Input::new()
        .with_prompt("Namespaces (comma-separated)")
        .default(format!("{}-ns", team))
        .interact_text()?;

    let namespace_list: Vec<String> = namespaces.split(',')
        .map(|s| s.trim().to_string())
        .collect();

    let max_gpus: u32 = Input::new()
        .with_prompt("Maximum GPUs for team")
        .default(8)
        .interact_text()?;

    let max_gpus_per_job: u32 = Input::new()
        .with_prompt("Maximum GPUs per job")
        .default(max_gpus)
        .interact_text()?;

    let max_running_jobs: u32 = Input::new()
        .with_prompt("Maximum running jobs")
        .default(10)
        .interact_text()?;

    let has_budget = Confirm::new()
        .with_prompt("Set a monthly budget?")
        .default(false)
        .interact()?;

    let mut budget = serde_json::Value::Null;
    if has_budget {
        let monthly_budget: f64 = Input::new()
            .with_prompt("Monthly budget (USD)")
            .default(10000.0)
            .interact_text()?;

        let alert_threshold: f64 = Input::new()
            .with_prompt("Alert threshold (%)")
            .default(80.0)
            .interact_text()?;

        let hard_limit = Confirm::new()
            .with_prompt("Enforce hard budget limit (stop jobs when exceeded)?")
            .default(false)
            .interact()?;

        budget = json!({
            "monthlyBudget": monthly_budget,
            "alertThreshold": alert_threshold / 100.0,
            "hardLimit": hard_limit,
        });
    }

    let quota_name = format!("{}-quota", team);
    let mut quota_spec = json!({
        "apiVersion": "gryvia.io/v1",
        "kind": "GryviaQuota",
        "metadata": {
            "name": quota_name,
        },
        "spec": {
            "team": team,
            "namespaces": namespace_list,
            "gpuQuota": {
                "maxGPUs": max_gpus,
                "maxGPUsPerJob": max_gpus_per_job,
                "maxRunningJobs": max_running_jobs,
            },
        }
    });

    if !budget.is_null() {
        quota_spec["spec"]["budget"] = budget;
    }

    println!();
    display::print_info("Quota configuration:");
    println!("{}", serde_json::to_string_pretty(&quota_spec)?);
    println!();

    let confirm = Confirm::new()
        .with_prompt("Create this quota?")
        .default(true)
        .interact()?;

    if !confirm {
        display::print_info("Cancelled");
        return Ok(());
    }

    let ar = kube::api::ApiResource::from_gvk(
        &kube::api::GroupVersionKind::gvk("gryvia.io", "v1", "GryviaQuota"),
    );
    let api: Api<kube::core::DynamicObject> = Api::all_with(
        client.kube_client.clone(),
        &ar,
    );

    let quota_obj: kube::core::DynamicObject = serde_json::from_value(quota_spec)
        .context("Failed to construct quota object")?;

    api.create(&PostParams::default(), &quota_obj).await
        .context("Failed to create quota")?;

    display::print_success(&format!("Quota '{}' created successfully", quota_name));

    Ok(())
}
