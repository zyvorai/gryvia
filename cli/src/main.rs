mod commands;
mod client;
mod display;
mod types;

use clap::{Parser, Subcommand};
use anyhow::Result;
use tracing_subscriber;

#[derive(Parser)]
#[command(name = "kubefabric")]
#[command(about = "KubeFabric CLI - Manage GPU clusters for AI workloads", long_about = None)]
#[command(version)]
struct Cli {
    #[command(subcommand)]
    command: Commands,

    /// Kubernetes context to use
    #[arg(long, global = true)]
    context: Option<String>,

    /// Namespace to use
    #[arg(short, long, global = true)]
    namespace: Option<String>,

    /// Enable verbose logging
    #[arg(short, long, global = true)]
    verbose: bool,
}

#[derive(Subcommand)]
enum Commands {
    /// Submit an AI training or inference job
    Submit {
        /// Path to job YAML file
        #[arg(short, long)]
        file: String,

        /// Wait for job to complete
        #[arg(short, long)]
        wait: bool,

        /// Follow logs after submission
        #[arg(long)]
        logs: bool,
    },

    /// List jobs, quotas, or nodes
    List {
        /// Resource type to list (jobs, quotas, nodes, storage, networks)
        resource: String,

        /// Show all namespaces
        #[arg(short, long)]
        all_namespaces: bool,

        /// Output format (table, json, yaml)
        #[arg(short, long, default_value = "table")]
        output: String,
    },

    /// Get detailed information about a resource
    Get {
        /// Resource type (job, quota, node, storage, network)
        resource: String,

        /// Resource name
        name: String,

        /// Output format (yaml, json)
        #[arg(short, long, default_value = "yaml")]
        output: String,
    },

    /// Delete a resource
    Delete {
        /// Resource type (job, quota, storage, network)
        resource: String,

        /// Resource name
        name: String,

        /// Skip confirmation
        #[arg(short, long)]
        yes: bool,
    },

    /// Show job status and logs
    Status {
        /// Job name
        job: String,

        /// Follow logs
        #[arg(short, long)]
        follow: bool,
    },

    /// View and follow job logs
    Logs {
        /// Job name
        job: String,

        /// Follow logs
        #[arg(short, long)]
        follow: bool,

        /// Number of lines to show from the end
        #[arg(long, default_value = "100")]
        tail: usize,

        /// Show logs for specific pod replica
        #[arg(long)]
        replica: Option<usize>,
    },

    /// Cancel a running or queued job
    Cancel {
        /// Job name(s) to cancel
        jobs: Vec<String>,

        /// Skip confirmation
        #[arg(short, long)]
        yes: bool,
    },

    /// Show GPU cluster overview
    Cluster {
        /// Show detailed node information
        #[arg(short, long)]
        detailed: bool,

        /// Refresh interval in seconds (watch mode)
        #[arg(short, long)]
        watch: Option<u64>,
    },

    /// Show GPU quota status for teams
    Quota {
        /// Team name (shows all teams if not specified)
        team: Option<String>,

        /// Show budget details
        #[arg(short, long)]
        budget: bool,
    },

    /// Show cost analysis and spending
    Cost {
        /// Team name (shows all teams if not specified)
        team: Option<String>,

        /// Time period (day, week, month)
        #[arg(short, long, default_value = "month")]
        period: String,

        /// Show detailed breakdown
        #[arg(short, long)]
        detailed: bool,
    },

    /// Show queue status
    Queue {
        /// Queue name (shows all queues if not specified)
        name: Option<String>,

        /// Watch mode with refresh interval
        #[arg(short, long)]
        watch: Option<u64>,
    },

    /// Interactive job creation wizard
    Create {
        /// Resource type (job, quota)
        resource: String,
    },

    /// Validate a job YAML file
    Validate {
        /// Path to job YAML file
        file: String,
    },

    /// Show cluster health and diagnostics
    Health {
        /// Check specific component (gpu, storage, network, all)
        #[arg(default_value = "all")]
        component: String,
    },
}

#[tokio::main]
async fn main() -> Result<()> {
    let cli = Cli::parse();

    // Setup logging
    let log_level = if cli.verbose { "debug" } else { "info" };
    tracing_subscriber::fmt()
        .with_env_filter(log_level)
        .init();

    // Create Kubernetes client
    let client = client::KubeFabricClient::new(cli.context, cli.namespace).await?;

    // Execute command
    match cli.command {
        Commands::Submit { file, wait, logs } => {
            commands::submit::execute(&client, &file, wait, logs).await?;
        }
        Commands::List { resource, all_namespaces, output } => {
            commands::list::execute(&client, &resource, all_namespaces, &output).await?;
        }
        Commands::Get { resource, name, output } => {
            commands::get::execute(&client, &resource, &name, &output).await?;
        }
        Commands::Delete { resource, name, yes } => {
            commands::delete::execute(&client, &resource, &name, yes).await?;
        }
        Commands::Status { job, follow } => {
            commands::status::execute(&client, &job, follow).await?;
        }
        Commands::Logs { job, follow, tail, replica } => {
            commands::logs::execute(&client, &job, follow, tail, replica).await?;
        }
        Commands::Cancel { jobs, yes } => {
            commands::cancel::execute(&client, &jobs, yes).await?;
        }
        Commands::Cluster { detailed, watch } => {
            commands::cluster::execute(&client, detailed, watch).await?;
        }
        Commands::Quota { team, budget } => {
            commands::quota::execute(&client, team, budget).await?;
        }
        Commands::Cost { team, period, detailed } => {
            commands::cost::execute(&client, team, &period, detailed).await?;
        }
        Commands::Queue { name, watch } => {
            commands::queue::execute(&client, name, watch).await?;
        }
        Commands::Create { resource } => {
            commands::create::execute(&client, &resource).await?;
        }
        Commands::Validate { file } => {
            commands::validate::execute(&file).await?;
        }
        Commands::Health { component } => {
            commands::health::execute(&client, &component).await?;
        }
    }

    Ok(())
}
