use anyhow::{Context, Result};
use colored::*;
use kube::api::{Api, ListParams};

use crate::client::GryviaClient;
use crate::display;
use crate::types::*;

pub async fn execute(client: &GryviaClient, team: Option<String>, budget: bool) -> Result<()> {
    let api: Api<GryviaQuota> = Api::all(client.kube_client.clone());

    let quotas = api
        .list(&ListParams::default())
        .await
        .context("Failed to list quotas")?;

    let filtered: Vec<_> = if let Some(team_name) = team {
        quotas
            .items
            .iter()
            .filter(|q| q.spec.team == team_name)
            .collect()
    } else {
        quotas.items.iter().collect()
    };

    if filtered.is_empty() {
        display::print_warning("No quotas found");
        return Ok(());
    }

    for quota in filtered {
        print_quota_details(quota, budget);
        println!();
    }

    Ok(())
}

fn print_quota_details(quota: &GryviaQuota, show_budget: bool) {
    let team = &quota.spec.team;
    let unknown = "<unknown>".to_string();
    let quota_name = quota.metadata.name.as_ref().unwrap_or(&unknown);

    println!("{} {}", "Team:".bold(), team.cyan().bold());
    println!("{} {}", "Quota:".bold(), quota_name);
    println!();

    // GPU Quota
    println!("{}", "GPU Quota:".bold().underline());
    let quota_status = quota.status.clone().unwrap_or_default();
    let usage = &quota_status.current_usage;
    let spec = &quota.spec.gpu_quota;

    println!(
        "  Allocated GPUs: {}/{} ({}%)",
        usage.allocated_gpus.to_string().yellow(),
        spec.max_gpus,
        if spec.max_gpus > 0 {
            ((usage.allocated_gpus as f64 / spec.max_gpus as f64) * 100.0).round() as i32
        } else {
            0
        }
    );

    println!("  Max GPUs per Job: {}", spec.max_gpus_per_job);
    println!(
        "  Running Jobs: {}/{}",
        usage.running_jobs, spec.max_running_jobs
    );
    println!("  Queued Jobs: {}", usage.queued_jobs);

    if !spec.allowed_gpu_types.is_empty() {
        println!("  Allowed GPU Types: {}", spec.allowed_gpu_types.join(", "));
    }

    println!("  GPU Hours (this month): {:.2}", usage.gpu_hours);

    // Budget Status
    if show_budget {
        if let Some(ref budget_status) = quota_status.budget_status {
            println!();
            println!("{}", "Budget Status:".bold().underline());

            let percent = budget_status.percent_used;
            let color = if percent >= 100.0 {
                "red"
            } else if percent >= 80.0 {
                "yellow"
            } else {
                "green"
            };

            println!("  Spent: ${:.2}", budget_status.spent_this_month);
            println!(
                "  Budget: ${:.2}",
                quota
                    .spec
                    .budget
                    .as_ref()
                    .map(|b| b.monthly_budget)
                    .unwrap_or(0.0)
            );
            println!("  Remaining: ${:.2}", budget_status.remaining_budget);

            let percent_str = format!("{:.1}%", percent);
            let colored_percent = match color {
                "red" => percent_str.red(),
                "yellow" => percent_str.yellow(),
                _ => percent_str.green(),
            };

            println!("  Used: {}", colored_percent);
            println!("  Projected: ${:.2}", budget_status.projected_spend);

            if let Some(ref budget) = quota.spec.budget {
                if budget.hard_limit && percent >= 100.0 {
                    println!(
                        "  {}",
                        "⚠ HARD LIMIT REACHED - New jobs will be blocked"
                            .red()
                            .bold()
                    );
                } else if percent >= budget.alert_threshold * 100.0 {
                    println!(
                        "  {}",
                        format!(
                            "⚠ Alert threshold ({:.0}%) reached",
                            budget.alert_threshold * 100.0
                        )
                        .yellow()
                        .bold()
                    );
                }
            }
        }
    }

    // Status
    println!();
    println!(
        "{} {}",
        "Status:".bold(),
        display::colorize_status(&quota_status.phase)
    );
}
