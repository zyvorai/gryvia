use anyhow::Result;
use kube::api::{Api, Patch, PatchParams};
use dialoguer::Confirm;

use crate::client::KubeFabricClient;
use crate::types::*;
use crate::display;

pub async fn execute(client: &KubeFabricClient, jobs: &[String], yes: bool) -> Result<()> {
    if jobs.is_empty() {
        return Err(anyhow::anyhow!("No jobs specified"));
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

    let mut failures = Vec::new();

    for job_name in jobs {
        // Patch job status to Cancelled instead of deleting
        let patch = serde_json::json!({
            "status": {
                "phase": "Cancelled",
                "message": "Cancelled by user"
            }
        });

        match api.patch_status(
            job_name,
            &PatchParams::apply("kubefabric-cli"),
            &Patch::Merge(&patch),
        ).await {
            Ok(_) => display::print_success(&format!("Job {} cancelled", job_name)),
            Err(e) => {
                display::print_error(&format!("Failed to cancel job {}: {}", job_name, e));
                failures.push(job_name.clone());
            }
        }
    }

    if !failures.is_empty() {
        return Err(anyhow::anyhow!("Failed to cancel {} job(s): {}", failures.len(), failures.join(", ")));
    }

    Ok(())
}
