use colored::*;
use prettytable::{Table, Row, Cell, format};
use crate::types::*;

pub fn print_jobs_table(jobs: &[FabricAIJob]) {
    let mut table = Table::new();
    table.set_format(*format::consts::FORMAT_BOX_CHARS);

    table.add_row(Row::new(vec![
        Cell::new("NAME").style_spec("Fb"),
        Cell::new("FRAMEWORK").style_spec("Fb"),
        Cell::new("GPUS").style_spec("Fb"),
        Cell::new("GPU TYPE").style_spec("Fb"),
        Cell::new("STATUS").style_spec("Fb"),
        Cell::new("AGE").style_spec("Fb"),
    ]));

    let unknown = "<unknown>".to_string();
    for job in jobs {
        let name = job.metadata.name.as_ref().unwrap_or(&unknown);
        let framework = &job.spec.framework;
        let gpus = job.spec.resources.gpu_count.to_string();
        let gpu_type = &job.spec.resources.gpu_type;
        let status = colorize_status(&job.status.phase);
        let age = format_age(job.metadata.creation_timestamp.as_ref());

        table.add_row(Row::new(vec![
            Cell::new(name),
            Cell::new(framework),
            Cell::new(&gpus),
            Cell::new(gpu_type),
            Cell::new(&status),
            Cell::new(&age),
        ]));
    }

    table.printstd();
}

pub fn print_quotas_table(quotas: &[FabricQuota]) {
    let mut table = Table::new();
    table.set_format(*format::consts::FORMAT_BOX_CHARS);

    table.add_row(Row::new(vec![
        Cell::new("TEAM").style_spec("Fb"),
        Cell::new("ALLOCATED").style_spec("Fb"),
        Cell::new("MAX GPUS").style_spec("Fb"),
        Cell::new("RUNNING").style_spec("Fb"),
        Cell::new("QUEUED").style_spec("Fb"),
        Cell::new("BUDGET").style_spec("Fb"),
        Cell::new("STATUS").style_spec("Fb"),
    ]));

    for quota in quotas {
        let team = &quota.spec.team;
        let allocated = quota.status.current_usage.allocated_gpus;
        let max = quota.spec.gpu_quota.max_gpus;
        let allocated_str = format!("{}/{}", allocated, max);
        let running = quota.status.current_usage.running_jobs.to_string();
        let queued = quota.status.current_usage.queued_jobs.to_string();

        let budget = if let Some(ref budget_status) = quota.status.budget_status {
            format!("${:.0}/{:.0}", budget_status.spent_this_month, quota.spec.budget.as_ref().map(|b| b.monthly_budget).unwrap_or(0.0))
        } else {
            "N/A".to_string()
        };

        let status = colorize_status(&quota.status.phase);

        table.add_row(Row::new(vec![
            Cell::new(team),
            Cell::new(&allocated_str),
            Cell::new(&max.to_string()),
            Cell::new(&running),
            Cell::new(&queued),
            Cell::new(&budget),
            Cell::new(&status),
        ]));
    }

    table.printstd();
}

pub fn print_nodes_table(nodes: &[FabricGpuNode]) {
    let mut table = Table::new();
    table.set_format(*format::consts::FORMAT_BOX_CHARS);

    table.add_row(Row::new(vec![
        Cell::new("NODE").style_spec("Fb"),
        Cell::new("GPU TYPE").style_spec("Fb"),
        Cell::new("GPU COUNT").style_spec("Fb"),
        Cell::new("MEMORY").style_spec("Fb"),
        Cell::new("RDMA").style_spec("Fb"),
        Cell::new("STATUS").style_spec("Fb"),
    ]));

    for node in nodes {
        let name = &node.spec.node_name;
        let gpu_type = &node.spec.gpu_type;
        let gpu_count = node.spec.gpu_count.to_string();
        let memory = &node.spec.memory;
        let rdma = if node.spec.rdma_enabled { "✓" } else { "✗" };
        let status = colorize_status(&node.status.phase);

        table.add_row(Row::new(vec![
            Cell::new(name),
            Cell::new(gpu_type),
            Cell::new(&gpu_count),
            Cell::new(memory),
            Cell::new(rdma),
            Cell::new(&status),
        ]));
    }

    table.printstd();
}

pub fn print_cluster_overview(nodes: &[FabricGpuNode], jobs: &[FabricAIJob]) {
    println!("{}", "━━━ GPU CLUSTER OVERVIEW ━━━".bold().cyan());
    println!();

    // Count GPUs by type
    let mut gpu_summary = std::collections::HashMap::new();
    let mut total_gpus = 0;
    let mut allocated_gpus = 0;

    for node in nodes {
        *gpu_summary.entry(&node.spec.gpu_type).or_insert(0) += node.spec.gpu_count;
        total_gpus += node.spec.gpu_count;
    }

    for job in jobs {
        if job.status.phase == "Running" {
            allocated_gpus += job.spec.resources.gpu_count;
        }
    }

    println!("  {} Nodes", nodes.len().to_string().bold());
    println!("  {} Total GPUs", total_gpus.to_string().bold());
    println!("  {} Allocated GPUs", allocated_gpus.to_string().green().bold());
    println!("  {} Available GPUs", (total_gpus.saturating_sub(allocated_gpus)).to_string().yellow().bold());
    println!();

    println!("{}", "GPU Distribution:".bold());
    for (gpu_type, count) in gpu_summary.iter() {
        println!("  • {}: {}", gpu_type.bold(), count);
    }
    println!();

    // Job stats
    let running = jobs.iter().filter(|j| j.status.phase == "Running").count();
    let pending = jobs.iter().filter(|j| j.status.phase == "Pending" || j.status.phase == "Queued").count();
    let completed = jobs.iter().filter(|j| j.status.phase == "Completed").count();
    let failed = jobs.iter().filter(|j| j.status.phase == "Failed").count();

    println!("{}", "Jobs:".bold());
    println!("  • Running: {}", running.to_string().green());
    println!("  • Pending: {}", pending.to_string().yellow());
    println!("  • Completed: {}", completed.to_string().cyan());
    println!("  • Failed: {}", failed.to_string().red());
    println!();
}

pub fn colorize_status(status: &str) -> String {
    match status {
        "Running" | "Active" | "Ready" | "Healthy" => status.green().to_string(),
        "Pending" | "Queued" => status.yellow().to_string(),
        "Failed" | "QuotaExceeded" | "BudgetExceeded" => status.red().to_string(),
        "Completed" => status.cyan().to_string(),
        _ => status.normal().to_string(),
    }
}

fn format_age(timestamp: Option<&k8s_openapi::apimachinery::pkg::apis::meta::v1::Time>) -> String {
    if let Some(ts) = timestamp {
        let age = chrono::Utc::now().signed_duration_since(ts.0);

        if age.num_seconds() < 0 {
            "clock skew".to_string()
        } else if age.num_days() > 0 {
            format!("{}d", age.num_days())
        } else if age.num_hours() > 0 {
            format!("{}h", age.num_hours())
        } else if age.num_minutes() > 0 {
            format!("{}m", age.num_minutes())
        } else {
            format!("{}s", age.num_seconds())
        }
    } else {
        "-".to_string()
    }
}

pub fn print_success(message: &str) {
    println!("{} {}", "✓".green().bold(), message);
}

pub fn print_error(message: &str) {
    eprintln!("{} {}", "✗".red().bold(), message);
}

pub fn print_warning(message: &str) {
    println!("{} {}", "⚠".yellow().bold(), message);
}

pub fn print_info(message: &str) {
    println!("{} {}", "ℹ".cyan().bold(), message);
}
