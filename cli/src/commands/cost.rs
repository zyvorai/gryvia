use anyhow::{Context, Result};
use kube::api::{Api, ListParams};
use prettytable::{Table, Row, Cell, format};
use colored::*;

use crate::client::KubeFabricClient;
use crate::types::*;

pub async fn execute(
    client: &KubeFabricClient,
    team: Option<String>,
    period: &str,
    detailed: bool,
) -> Result<()> {
    let _ = detailed; // TODO: implement detailed breakdown

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

    for quota in filtered {
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

    Ok(())
}
