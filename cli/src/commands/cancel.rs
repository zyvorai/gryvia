use anyhow::{Context, Result};
use kube::api::{Api, DeleteParams};
use dialoguer::Confirm;

use crate::client::KubeFabricClient;
use crate::types::*;
use crate::display;

pub async fn execute(client: &KubeFabricClient, jobs: &[String], yes: bool) -> Result<()> {
    if jobs.is_empty() {
        display::print_error("No jobs specified");
        return Ok(());
    }

    if !yes {
        let job_list = jobs.join(", ");
        let confirm = Confirm::new()
            .with_prompt(format!("Cancel {} job(s): {}?", jobs.len(), job_list))
            .interact()?;

        if !confirm {
            display::print_info("Cancelled");
            return Ok(());
        }
    }

    let api: Api<FabricAIJob> = Api::namespaced(
        client.kube_client.clone(),
        client.namespace(),
    );

    for job_name in jobs {
        match api.delete(job_name, &DeleteParams::default()).await {
            Ok(_) => display::print_success(&format!("Job {} cancelled", job_name)),
            Err(e) => display::print_error(&format!("Failed to cancel job {}: {}", job_name, e)),
        }
    }

    Ok(())
}
