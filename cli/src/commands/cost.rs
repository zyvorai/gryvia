use anyhow::{Context, Result};
use kube::api::{Api, ListParams};
use prettytable::{Table, Row, Cell, format};
use colored::*;

use crate::client::TensorReaperClient;
use crate::types::*;

pub async fn execute(
    client: &TensorReaperClient,
    team: Option<String>,
    period: &str,
    detailed: bool,
) -> Result<()> {
    // Validate period
    match period {
        "day" | "week" | "month" => {}
        _ => anyhow::bail!("Invalid period: '{}'. Valid periods: day, week, month", period),
    }
    let api: Api<FabricQuota> = Api::all(client.kube_client.clone());

    let quotas = api.list(&ListParams::default()).await
        .context("Failed to list quotas")?;

    let filtered: Vec<_> = if let Some(team_name) = team {
        quotas.items.iter()
            .filter(|q| q.spec.team == team_name)
            .collect()
    } else {
        quotas.items.iter().collect()
    };

    println!("{}", format!("━━━ Cost Analysis ({}) ━━━", period).bold().cyan());
    println!();

    let mut table = Table::new();
    table.set_format(*format::consts::FORMAT_BOX_CHARS);

    table.add_row(Row::new(vec![
        Cell::new("TEAM").style_spec("Fb"),
        Cell::new("SPENT").style_spec("Fb"),
        Cell::new("BUDGET").style_spec("Fb"),
        Cell::new("REMAINING").style_spec("Fb"),
        Cell::new("% USED").style_spec("Fb"),
        Cell::new("PROJECTED").style_spec("Fb"),
    ]));

    let mut total_spent = 0.0;
    let mut total_budget = 0.0;

    for quota in &filtered {
        let quota_status = quota.status.clone().unwrap_or_default();
        if let Some(ref budget_status) = quota_status.budget_status {
            let team = &quota.spec.team;
            let spent = budget_status.spent_this_month;
            let budget = quota.spec.budget.as_ref().map(|b| b.monthly_budget).unwrap_or(0.0);
            let remaining = budget_status.remaining_budget;
            let percent = budget_status.percent_used;
            let projected = budget_status.projected_spend;

            total_spent += spent;
            total_budget += budget;

            let percent_str = format!("{:.1}%", percent);
            let colored_percent = if percent >= 100.0 {
                percent_str.red().to_string()
            } else if percent >= 80.0 {
                percent_str.yellow().to_string()
            } else {
                percent_str.green().to_string()
            };

            table.add_row(Row::new(vec![
                Cell::new(team),
                Cell::new(&format!("${:.2}", spent)),
                Cell::new(&format!("${:.2}", budget)),
                Cell::new(&format!("${:.2}", remaining)),
                Cell::new(&colored_percent),
                Cell::new(&format!("${:.2}", projected)),
            ]));
        }
    }

    table.printstd();

    println!();
    println!("{}", "Summary:".bold());
    println!("  Total Spent: {}", format!("${:.2}", total_spent).yellow());
    println!("  Total Budget: ${:.2}", total_budget);
    println!("  Total Remaining: {}", format!("${:.2}", total_budget - total_spent).green());
    println!("  Overall Usage: {:.1}%", if total_budget > 0.0 { total_spent / total_budget * 100.0 } else { 0.0 });

    if detailed {
        println!();
        show_detailed_breakdown(&filtered).await?;
    }

    Ok(())
}

async fn show_detailed_breakdown(quotas: &[&FabricQuota]) -> Result<()> {
    println!("{}", "━━━ Detailed Breakdown ━━━".bold().cyan());
    println!();

    // GPU hourly rates
    let gpu_rates = [
        ("H100", 3.50),
        ("A100-80G", 2.21),
        ("A100-40G", 1.80),
        ("L40", 1.20),
        ("A10", 0.75),
        ("V100", 0.55),
        ("T4", 0.35),
    ];

    println!("{}", "GPU Pricing (per GPU-hour):".bold().underline());
    let mut price_table = Table::new();
    price_table.set_format(*format::consts::FORMAT_BOX_CHARS);

    price_table.add_row(Row::new(vec![
        Cell::new("GPU TYPE").style_spec("Fb"),
        Cell::new("$/HOUR").style_spec("Fb"),
        Cell::new("$/DAY").style_spec("Fb"),
        Cell::new("$/MONTH").style_spec("Fb"),
    ]));

    for (gpu, rate) in &gpu_rates {
        price_table.add_row(Row::new(vec![
            Cell::new(gpu),
            Cell::new(&format!("${:.2}", rate)),
            Cell::new(&format!("${:.2}", rate * 24.0)),
            Cell::new(&format!("${:.0}", rate * 24.0 * 30.0)),
        ]));
    }

    price_table.printstd();
    println!();

    // Per-team details
    for quota in quotas {
        let quota_status = quota.status.clone().unwrap_or_default();

        println!("{}", format!("Team: {}", quota.spec.team).bold().underline());
        println!();

        // Usage breakdown
        println!("  {}", "Resource Usage:".bold());
        println!("    Allocated GPUs:  {}", quota_status.current_usage.allocated_gpus);
        println!("    Running Jobs:    {}", quota_status.current_usage.running_jobs);
        println!("    Queued Jobs:     {}", quota_status.current_usage.queued_jobs);
        println!("    GPU-Hours (MTD): {:.1}", quota_status.current_usage.gpu_hours);
        println!();

        // Quota limits
        println!("  {}", "Quota Limits:".bold());
        println!("    Max GPUs:          {}", quota.spec.gpu_quota.max_gpus);
        println!("    Max GPUs/Job:      {}", quota.spec.gpu_quota.max_gpus_per_job);
        println!("    Max Running Jobs:  {}", quota.spec.gpu_quota.max_running_jobs);
        if !quota.spec.gpu_quota.allowed_gpu_types.is_empty() {
            println!("    Allowed GPU Types: {}", quota.spec.gpu_quota.allowed_gpu_types.join(", "));
        }
        println!();

        // Budget details
        if let Some(ref budget_spec) = quota.spec.budget {
            println!("  {}", "Budget:".bold());
            println!("    Monthly Budget:    ${:.2}", budget_spec.monthly_budget);
            println!("    Alert Threshold:   {:.0}%", budget_spec.alert_threshold * 100.0);
            println!("    Hard Limit:        {}", if budget_spec.hard_limit { "yes".red() } else { "no".green() });

            if let Some(ref bs) = quota_status.budget_status {
                println!("    Spent This Month:  ${:.2}", bs.spent_this_month);
                println!("    Remaining:         ${:.2}", bs.remaining_budget);
                println!("    Projected Spend:   ${:.2}", bs.projected_spend);

                if bs.projected_spend > budget_spec.monthly_budget {
                    println!("    {}", "WARNING: Projected to exceed budget!".red().bold());
                }
            }
            println!();
        }
    }

    Ok(())
}
