use anyhow::{Context, Result};
use kube::api::{Api, ListParams};
use serde_json;

use crate::client::KubeFabricClient;
use crate::types::*;
use crate::display;

pub async fn execute(client: &KubeFabricClient, resource: &str, all_namespaces: bool, output: &str) -> Result<()> {
    match resource {
        "jobs" | "job" => list_jobs(client, all_namespaces, output).await?,
        "quotas" | "quota" => list_quotas(client, output).await?,
        "nodes" | "node" => list_nodes(client, output).await?,
        _ => {
            display::print_error(&format!("Unknown resource type: {}", resource));
            return Ok(());
        }
    }

    Ok(())
}

async fn list_jobs(client: &KubeFabricClient, _all_namespaces: bool, output: &str) -> Result<()> {
    let api: Api<FabricAIJob> = Api::namespaced(
        client.kube_client.clone(),
        client.namespace(),
    );

    let jobs = api.list(&ListParams::default()).await
        .context("Failed to list jobs")?;

    match output {
        "json" => {
            println!("{}", serde_json::to_string_pretty(&jobs)?);
        }
        "yaml" => {
            println!("{}", serde_yaml::to_string(&jobs)?);
        }
        _ => {
            display::print_jobs_table(&jobs.items);
        }
    }

    Ok(())
}

async fn list_quotas(client: &KubeFabricClient, output: &str) -> Result<()> {
    let api: Api<FabricQuota> = Api::all(client.kube_client.clone());

    let quotas = api.list(&ListParams::default()).await
        .context("Failed to list quotas")?;

    match output {
        "json" => {
            println!("{}", serde_json::to_string_pretty(&quotas)?);
        }
        "yaml" => {
            println!("{}", serde_yaml::to_string(&quotas)?);
        }
        _ => {
            display::print_quotas_table(&quotas.items);
        }
    }

    Ok(())
}

async fn list_nodes(client: &KubeFabricClient, output: &str) -> Result<()> {
    let api: Api<FabricGpuNode> = Api::all(client.kube_client.clone());

    let nodes = api.list(&ListParams::default()).await
        .context("Failed to list GPU nodes")?;

    match output {
        "json" => {
            println!("{}", serde_json::to_string_pretty(&nodes)?);
        }
        "yaml" => {
            println!("{}", serde_yaml::to_string(&nodes)?);
        }
        _ => {
            display::print_nodes_table(&nodes.items);
        }
    }

    Ok(())
}
