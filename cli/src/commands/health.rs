use anyhow::{Context, Result};
use kube::api::{Api, ListParams};
use prettytable::{Table, Row, Cell, format};
use colored::*;

use crate::client::KubeFabricClient;
use crate::types::*;

pub async fn execute(client: &KubeFabricClient, component: &str) -> Result<()> {
    println!("{}", "━━━ Cluster Health Check ━━━".bold().cyan());
    println!();

    match component {
        "all" => {
            check_gpu_health(client).await?;
            check_storage_health(client).await?;
            check_network_health(client).await?;
        }
        "gpu" => check_gpu_health(client).await?,
        "storage" => check_storage_health(client).await?,
        "network" => check_network_health(client).await?,
        _ => {
            anyhow::bail!("Unknown component: {}. Valid components: all, gpu, storage, network", component);
        }
    }

    Ok(())
}

async fn check_gpu_health(client: &KubeFabricClient) -> Result<()> {
    println!("{}", "GPU Nodes:".bold().underline());

    let api: Api<FabricGpuNode> = Api::all(client.kube_client.clone());
    let nodes = api.list(&ListParams::default()).await
        .context("Failed to list GPU nodes")?;

    let mut table = Table::new();
    table.set_format(*format::consts::FORMAT_BOX_CHARS);

    table.add_row(Row::new(vec![
        Cell::new("NODE").style_spec("Fb"),
        Cell::new("STATUS").style_spec("Fb"),
        Cell::new("GPUs").style_spec("Fb"),
    ]));

    for node in nodes.items {
        let name = &node.spec.node_name;
        let node_status = node.status.clone().unwrap_or_default();
        let status = match node_status.phase.as_str() {
            "Healthy" | "Ready" => node_status.phase.green().to_string(),
            "Degraded" => node_status.phase.yellow().to_string(),
            _ => node_status.phase.red().to_string(),
        };
        let gpus = format!("{} x {}", node.spec.gpu_count, node.spec.gpu_type);

        table.add_row(Row::new(vec![
            Cell::new(name),
            Cell::new(&status),
            Cell::new(&gpus),
        ]));
    }

    table.printstd();
    println!();

    Ok(())
}

async fn check_storage_health(_client: &KubeFabricClient) -> Result<()> {
    println!("{}", "Storage:".bold().underline());
    println!("  Storage health check not yet implemented");
    println!();
    Ok(())
}

async fn check_network_health(_client: &KubeFabricClient) -> Result<()> {
    println!("{}", "Network:".bold().underline());
    println!("  Network health check not yet implemented");
    println!();
    Ok(())
}
