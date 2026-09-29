use anyhow::{Context, Result};
use futures::{AsyncBufReadExt, StreamExt};
use k8s_openapi::api::core::v1::Pod;
use kube::api::{Api, ListParams, LogParams};
use kube::api::{ApiResource, GroupVersionKind};
use kube::core::DynamicObject;

use crate::client::GryviaClient;
use crate::display;

pub async fn execute(
    client: &GryviaClient,
    job: &str,
    follow: bool,
    tail: usize,
    replica: Option<usize>,
) -> Result<()> {
    // Validate job name to prevent label selector injection
    if job.contains('=') || job.contains(',') || job.contains('!') {
        anyhow::bail!(
            "Invalid job name '{}': contains reserved label selector characters",
            job
        );
    }

    let pods_api: Api<Pod> = Api::namespaced(client.kube_client.clone(), client.namespace());

    let label_selector = format!("gryvia.io/job={}", job);
    let lp = ListParams::default().labels(&label_selector);

    let pods = pods_api
        .list(&lp)
        .await
        .context("Failed to list pods for job")?;

    if pods.items.is_empty() {
        let ar = ApiResource::from_gvk(&GroupVersionKind::gvk(
            "gryvia.io",
            "v1alpha1",
            "GryviaAIJob",
        ));
        let jobs_api: Api<DynamicObject> =
            Api::namespaced_with(client.kube_client.clone(), client.namespace(), &ar);
        let _ = jobs_api
            .get(job)
            .await
            .with_context(|| format!("Job '{}' not found", job))?;

        display::print_warning(&format!(
            "No pods found for job '{}' - job may still be scheduling",
            job
        ));
        return Ok(());
    }

    let target_pod = if let Some(idx) = replica {
        let pod_name = format!("{}-training-{}", job, idx);
        pods.items
            .iter()
            .find(|p| p.metadata.name.as_deref() == Some(&pod_name))
            .with_context(|| format!("Replica {} not found for job '{}'", idx, job))?
    } else {
        &pods.items[0]
    };

    let pod_name = target_pod.metadata.name.as_deref().unwrap_or("unknown");
    display::print_info(&format!("Streaming logs from pod: {}", pod_name));

    // Detect container name from the pod spec, defaulting to "trainer"
    let container_name = target_pod
        .spec
        .as_ref()
        .and_then(|s| s.containers.first())
        .map(|c| c.name.clone())
        .unwrap_or_else(|| "trainer".to_string());

    let log_params = LogParams {
        follow,
        tail_lines: Some(tail as i64),
        container: Some(container_name),
        ..Default::default()
    };

    if follow {
        let stream = pods_api
            .log_stream(pod_name, &log_params)
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
    } else {
        let logs = pods_api
            .logs(pod_name, &log_params)
            .await
            .context("Failed to get pod logs")?;
        print!("{}", logs);
    }

    Ok(())
}
