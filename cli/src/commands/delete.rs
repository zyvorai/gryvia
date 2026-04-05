use anyhow::{Context, Result};
use kube::api::{Api, DeleteParams};
use dialoguer::Confirm;

use crate::client::KubeFabricClient;
use crate::types::*;
use crate::display;

pub async fn execute(client: &KubeFabricClient, resource: &str, name: &str, yes: bool) -> Result<()> {
    if !yes {
        let confirm = Confirm::new()
            .with_prompt(format!("Delete {} '{}'?", resource, name))
            .interact()?;

        if !confirm {
            display::print_info("Cancelled");
            return Ok(());
        }
    }

    match resource {
        "job" => delete_job(client, name).await?,
        "quota" => delete_quota(client, name).await?,
        "storage" => {
            display::print_warning("Resource type 'storage' delete is not yet implemented");
        }
        "network" => {
            display::print_warning("Resource type 'network' delete is not yet implemented");
        }
        _ => {
            anyhow::bail!("Unknown resource type: {}. Valid types: job, quota, storage, network", resource);
        }
    }

    Ok(())
}

async fn delete_job(client: &KubeFabricClient, name: &str) -> Result<()> {
    let api: Api<FabricAIJob> = Api::namespaced(
        client.kube_client.clone(),
        client.namespace(),
    );

    api.delete(name, &DeleteParams::default()).await
        .context("Failed to delete job")?;

    display::print_success(&format!("Job {} deleted", name));

    Ok(())
}

async fn delete_quota(client: &KubeFabricClient, name: &str) -> Result<()> {
    let api: Api<FabricQuota> = Api::all(client.kube_client.clone());

    api.delete(name, &DeleteParams::default()).await
        .context("Failed to delete quota")?;

    display::print_success(&format!("Quota {} deleted", name));

    Ok(())
}
