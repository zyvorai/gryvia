use anyhow::{Context, Result};
use colored::*;
use kube::api::{Api, ApiResource, GroupVersionKind, ListParams};
use kube::core::DynamicObject;
use prettytable::{format, Cell, Row, Table};

use crate::client::GryviaClient;
use crate::types::*;

pub async fn execute(client: &GryviaClient, component: &str) -> Result<()> {
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
            anyhow::bail!(
                "Unknown component: {}. Valid components: all, gpu, storage, network",
                component
            );
        }
    }

    Ok(())
}

async fn check_gpu_health(client: &GryviaClient) -> Result<()> {
    println!("{}", "GPU Nodes:".bold().underline());

    let api: Api<GryviaGpuNode> = Api::all(client.kube_client.clone());
    let nodes = api
        .list(&ListParams::default())
        .await
        .context("Failed to list GPU nodes")?;

    if nodes.items.is_empty() {
        println!("  No GPU nodes registered");
        println!();
        return Ok(());
    }

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

async fn check_storage_health(client: &GryviaClient) -> Result<()> {
    println!("{}", "Storage:".bold().underline());

    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk("gryvia.io", "v1", "GryviaStorage"));
    let api: Api<DynamicObject> = Api::all_with(client.kube_client.clone(), &ar);

    let storages = match api.list(&ListParams::default()).await {
        Ok(list) => list,
        Err(e) => {
            // CRD might not be installed
            println!("  Could not query storage resources: {}", e);
            println!();
            return Ok(());
        }
    };

    if storages.items.is_empty() {
        println!("  No storage resources configured");
        println!();
        return Ok(());
    }

    let mut table = Table::new();
    table.set_format(*format::consts::FORMAT_BOX_CHARS);

    table.add_row(Row::new(vec![
        Cell::new("NAME").style_spec("Fb"),
        Cell::new("BACKEND").style_spec("Fb"),
        Cell::new("CAPACITY").style_spec("Fb"),
        Cell::new("RDMA").style_spec("Fb"),
        Cell::new("CSI").style_spec("Fb"),
        Cell::new("STATUS").style_spec("Fb"),
    ]));

    for storage in &storages.items {
        let name = storage.metadata.name.as_deref().unwrap_or("<unknown>");
        let spec = storage.data.get("spec");
        let status = storage.data.get("status");

        let backend = spec
            .and_then(|s| s.get("backend"))
            .and_then(|v| v.as_str())
            .unwrap_or("-");
        let capacity = spec
            .and_then(|s| s.get("capacity"))
            .and_then(|v| v.as_str())
            .unwrap_or("-");
        let rdma = spec
            .and_then(|s| s.get("rdma"))
            .and_then(|v| v.as_bool())
            .unwrap_or(false);
        let rdma_str = if rdma {
            "yes".green().to_string()
        } else {
            "no".normal().to_string()
        };

        let csi_installed = status
            .and_then(|s| s.get("csiDriverInstalled"))
            .and_then(|v| v.as_bool())
            .unwrap_or(false);
        let csi_str = if csi_installed {
            "installed".green().to_string()
        } else {
            "missing".red().to_string()
        };

        let phase = status
            .and_then(|s| s.get("phase"))
            .and_then(|v| v.as_str())
            .unwrap_or("Unknown");
        let phase_colored = match phase {
            "Ready" => phase.green().to_string(),
            "Configuring" | "Pending" => phase.yellow().to_string(),
            "Degraded" => phase.yellow().to_string(),
            _ => phase.red().to_string(),
        };

        table.add_row(Row::new(vec![
            Cell::new(name),
            Cell::new(backend),
            Cell::new(capacity),
            Cell::new(&rdma_str),
            Cell::new(&csi_str),
            Cell::new(&phase_colored),
        ]));
    }

    table.printstd();
    println!();

    Ok(())
}

async fn check_network_health(client: &GryviaClient) -> Result<()> {
    println!("{}", "Network:".bold().underline());

    let ar = ApiResource::from_gvk(&GroupVersionKind::gvk("gryvia.io", "v1", "GryviaNetwork"));
    let api: Api<DynamicObject> = Api::all_with(client.kube_client.clone(), &ar);

    let networks = match api.list(&ListParams::default()).await {
        Ok(list) => list,
        Err(e) => {
            println!("  Could not query network resources: {}", e);
            println!();
            return Ok(());
        }
    };

    if networks.items.is_empty() {
        println!("  No network resources configured");
        println!();
        return Ok(());
    }

    let mut table = Table::new();
    table.set_format(*format::consts::FORMAT_BOX_CHARS);

    table.add_row(Row::new(vec![
        Cell::new("NAME").style_spec("Fb"),
        Cell::new("TYPE").style_spec("Fb"),
        Cell::new("NODES").style_spec("Fb"),
        Cell::new("STATUS").style_spec("Fb"),
    ]));

    for network in &networks.items {
        let name = network.metadata.name.as_deref().unwrap_or("<unknown>");
        let spec = network.data.get("spec");
        let status = network.data.get("status");

        let network_type = spec
            .and_then(|s| s.get("networkType"))
            .and_then(|v| v.as_str())
            .unwrap_or("-");

        let configured_nodes = status
            .and_then(|s| s.get("configuredNodes"))
            .and_then(|v| v.as_i64())
            .unwrap_or(0);
        let total_nodes = status
            .and_then(|s| s.get("totalNodes"))
            .and_then(|v| v.as_i64())
            .unwrap_or(0);
        let nodes_str = format!("{}/{}", configured_nodes, total_nodes);

        let phase = status
            .and_then(|s| s.get("phase"))
            .and_then(|v| v.as_str())
            .unwrap_or("Unknown");
        let phase_colored = match phase {
            "Ready" | "Configured" => phase.green().to_string(),
            "Configuring" | "Pending" => phase.yellow().to_string(),
            "Degraded" | "PartiallyConfigured" => phase.yellow().to_string(),
            _ => phase.red().to_string(),
        };

        table.add_row(Row::new(vec![
            Cell::new(name),
            Cell::new(network_type),
            Cell::new(&nodes_str),
            Cell::new(&phase_colored),
        ]));
    }

    table.printstd();
    println!();

    Ok(())
}
