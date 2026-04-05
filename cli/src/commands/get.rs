use anyhow::{Context, Result};
use kube::api::Api;

use crate::client::KubeFabricClient;
use crate::types::*;
use crate::display;

pub async fn execute(client: &KubeFabricClient, resource: &str, name: &str, output: &str) -> Result<()> {
    match resource {
        "job" => get_job(client, name, output).await?,
        "quota" => get_quota(client, name, output).await?,
        "node" => get_node(client, name, output).await?,
        _ => {
            anyhow::bail!("Unknown resource type: {}. Valid types: job, quota, node", resource);
        }
    }

    Ok(())
}

async fn get_job(client: &KubeFabricClient, name: &str, output: &str) -> Result<()> {
    let api: Api<FabricAIJob> = Api::namespaced(
        client.kube_client.clone(),
        client.namespace(),
    );

    let job = api.get(name).await
        .context("Failed to get job")?;

    match output {
        "json" => {
            println!("{}", serde_json::to_string_pretty(&job)?);
        }
        _ => {
            println!("{}", serde_yaml::to_string(&job)?);
        }
    }

    Ok(())
}

async fn get_quota(client: &KubeFabricClient, name: &str, output: &str) -> Result<()> {
    let api: Api<FabricQuota> = Api::all(client.kube_client.clone());

    let quota = api.get(name).await
        .context("Failed to get quota")?;

    match output {
        "json" => {
            println!("{}", serde_json::to_string_pretty(&quota)?);
        }
        _ => {
            println!("{}", serde_yaml::to_string(&quota)?);
        }
    }

    Ok(())
}

async fn get_node(client: &KubeFabricClient, name: &str, output: &str) -> Result<()> {
    let api: Api<FabricGpuNode> = Api::all(client.kube_client.clone());

    let node = api.get(name).await
        .context("Failed to get GPU node")?;

    match output {
        "json" => {
            println!("{}", serde_json::to_string_pretty(&node)?);
        }
        _ => {
            println!("{}", serde_yaml::to_string(&node)?);
        }
    }

    Ok(())
}
