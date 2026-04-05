use anyhow::{Context, Result};
use kube::api::{Api, ListParams};
use tokio::time::{sleep, Duration};

use crate::client::KubeFabricClient;
use crate::types::*;
use crate::display;

pub async fn execute(client: &KubeFabricClient, detailed: bool, watch: Option<u64>) -> Result<()> {
    if let Some(interval) = watch {
        loop {
            print!("\x1B[2J\x1B[1;1H"); // Clear screen
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

async fn show_cluster_overview(client: &KubeFabricClient, _detailed: bool) -> Result<()> {
    let nodes_api: Api<FabricGpuNode> = Api::all(client.kube_client.clone());
    let jobs_api: Api<FabricAIJob> = Api::namespaced(
        client.kube_client.clone(),
        client.namespace(),
    );

    let nodes = nodes_api.list(&ListParams::default()).await
        .context("Failed to list GPU nodes")?;

    let jobs = jobs_api.list(&ListParams::default()).await
        .context("Failed to list jobs")?;

    display::print_cluster_overview(&nodes.items, &jobs.items);

    Ok(())
}
