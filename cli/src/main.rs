mod client;
mod commands;
mod display;
mod output;
mod platform;
mod types;
mod ui;

use anyhow::Result;
use clap::builder::styling::{AnsiColor, Effects};
use clap::builder::Styles;
use clap::{ColorChoice, CommandFactory, FromArgMatches, Parser, Subcommand};
use clap_complete::Shell;
use commands::usage::{GroupBy, UsageFormat};
use output::{OutputFormat, StructuredFormat};

/// An "Examples:" block for a command's help, colored like the rest of the help.
fn examples(lines: &[&str]) -> clap::builder::StyledStr {
    use clap::builder::styling::Style;
    let head: Style = AnsiColor::Cyan.on_default().effects(Effects::BOLD);
    let cmd: Style = AnsiColor::Green.on_default();
    let mut out = format!("{head}Examples:{head:#}\n");
    for line in lines {
        out.push_str(&format!("  {cmd}{line}{cmd:#}\n"));
    }
    out.into()
}

/// Help colors: cyan headings, green commands and flags, yellow placeholders.
const STYLES: Styles = Styles::styled()
    .header(AnsiColor::Cyan.on_default().effects(Effects::BOLD))
    .usage(AnsiColor::Cyan.on_default().effects(Effects::BOLD))
    .literal(AnsiColor::Green.on_default().effects(Effects::BOLD))
    .placeholder(AnsiColor::Yellow.on_default())
    .error(AnsiColor::Red.on_default().effects(Effects::BOLD))
    .valid(AnsiColor::Green.on_default())
    .invalid(AnsiColor::Yellow.on_default());

#[derive(Parser)]
#[command(name = "gryvia")]
#[command(about = "Gryvia CLI - Manage GPU clusters for AI workloads", long_about = None)]
#[command(version, styles = STYLES)]
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

    /// Disable colored output (also honours NO_COLOR)
    #[arg(long, global = true)]
    no_color: bool,
}

#[derive(Subcommand)]
enum Commands {
    /// Submit an AI training or inference job
    #[command(after_help = examples(&["gryvia submit --file job.yaml", "gryvia submit --file job.yaml --wait --logs"]))]
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
    #[command(visible_alias = "ls")]
    #[command(after_help = examples(&["gryvia list jobs", "gryvia list jobs --all-namespaces -o json", "gryvia list nodes"]))]
    List {
        /// Resource type to list (jobs, quotas, nodes)
        resource: String,

        /// Show all namespaces
        #[arg(short, long)]
        all_namespaces: bool,

        /// Output format
        #[arg(short, long, value_enum, default_value_t = OutputFormat::Table, env = "GRYVIA_OUTPUT")]
        output: OutputFormat,
    },

    /// Get detailed information about a resource
    #[command(visible_alias = "describe")]
    #[command(after_help = examples(&["gryvia get job my-job", "gryvia get node gpu-node-01 -o json"]))]
    Get {
        /// Resource type (job, quota, node, storage, network)
        resource: String,

        /// Resource name
        name: String,

        /// Output format
        #[arg(short, long, value_enum, default_value_t = StructuredFormat::Yaml)]
        output: StructuredFormat,
    },

    /// Delete a resource
    #[command(visible_alias = "rm")]
    #[command(after_help = examples(&["gryvia delete job my-job --yes"]))]
    Delete {
        /// Resource type (job, quota, storage, network)
        resource: String,

        /// Resource name
        name: String,

        /// Skip confirmation
        #[arg(short, long)]
        yes: bool,
    },

    /// Show platform status, or the status of one job
    ///
    /// With no argument: a report of the whole platform (operators, gateway, dashboard, GPU nodes, add-ons,
    /// collectors), workload readiness and a per-node table, like `cilium status`. Exits 1 when a required
    /// component is failing. With a job name: that job's details.
    #[command(after_help = examples(&["gryvia status", "gryvia status --brief", "gryvia status --wait --wait-timeout 5m", "gryvia status --node gpu-node-01", "gryvia status -o json", "gryvia status my-job --follow"]))]
    Status {
        /// Job name (omit to show the platform)
        job: Option<String>,

        /// Follow logs (job status)
        #[arg(short, long, requires = "job")]
        follow: bool,

        /// Only this node in the per-node report
        #[arg(long, conflicts_with = "job")]
        node: Option<String>,

        /// Print only OK, or the failing components (exit code 1 on failure)
        #[arg(long, conflicts_with = "job")]
        brief: bool,

        /// Wait until the platform is OK (for scripts and CI)
        #[arg(long, conflicts_with = "job")]
        wait: bool,

        /// How long to wait, like 90s or 5m
        #[arg(long, default_value = "5m", requires = "wait")]
        wait_timeout: String,

        /// Output format
        #[arg(short, long, value_enum, default_value_t = OutputFormat::Table, conflicts_with = "job")]
        output: OutputFormat,

        /// Namespace the platform is installed in
        #[arg(long, default_value = "gryvia-system", conflicts_with = "job")]
        platform_namespace: String,
    },

    /// View and follow job logs
    #[command(after_help = examples(&["gryvia logs my-job --follow", "gryvia logs my-job --tail 100"]))]
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
    #[command(after_help = examples(&["gryvia cancel my-job", "gryvia cancel job-a job-b --yes"]))]
    Cancel {
        /// Job name(s) to cancel
        jobs: Vec<String>,

        /// Skip confirmation
        #[arg(short, long)]
        yes: bool,
    },

    /// Show GPU cluster overview
    #[command(after_help = examples(&["gryvia cluster", "gryvia cluster --detailed", "gryvia cluster -o json"]))]
    Cluster {
        /// Show detailed node information
        #[arg(short, long)]
        detailed: bool,

        /// Refresh interval in seconds (watch mode)
        #[arg(short, long)]
        watch: Option<u64>,

        /// Output format
        #[arg(short, long, value_enum, default_value_t = OutputFormat::Table, env = "GRYVIA_OUTPUT")]
        output: OutputFormat,
    },

    /// Show GPU quota status for teams
    #[command(after_help = examples(&["gryvia quota", "gryvia quota ml-research --budget", "gryvia quota -o yaml"]))]
    Quota {
        /// Team name (shows all teams if not specified)
        team: Option<String>,

        /// Show budget details
        #[arg(short, long)]
        budget: bool,

        /// Output format
        #[arg(short, long, value_enum, default_value_t = OutputFormat::Table, env = "GRYVIA_OUTPUT")]
        output: OutputFormat,
    },

    /// Show cost analysis and spending
    #[command(after_help = examples(&["gryvia cost --period month", "gryvia cost ml-research --detailed"]))]
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
    #[command(after_help = examples(&["gryvia queue", "gryvia queue --watch 5", "gryvia queue -o json"]))]
    Queue {
        /// Queue name (shows all queues if not specified)
        name: Option<String>,

        /// Watch mode with refresh interval
        #[arg(short, long)]
        watch: Option<u64>,

        /// Output format
        #[arg(short, long, value_enum, default_value_t = OutputFormat::Table, env = "GRYVIA_OUTPUT")]
        output: OutputFormat,
    },

    /// List the GPU SKUs on offer, with hourly rates
    ///
    /// Read-only. Shows every GryviaGpuSku: GPU type, GPUs per unit, rate per hour, spot discount and
    /// whether it is enabled. Rates are what usage metering multiplies GPU hours by.
    #[command(visible_alias = "skus")]
    #[command(after_help = examples(&["gryvia catalog", "gryvia skus -o json"]))]
    Catalog {
        /// Output format
        #[arg(short, long, value_enum, default_value_t = OutputFormat::Table, env = "GRYVIA_OUTPUT")]
        output: OutputFormat,
    },

    /// Manage tenants: list, get, create, delete
    ///
    /// A tenant is a GryviaTenant object; its workloads run in the namespace `tenant-<name>`.
    #[command(after_help = examples(&["gryvia tenant list", "gryvia tenant get acme", "gryvia tenant create acme --display-name Acme --allowed-sku h100-8x --max-gpus 16 --isolated true", "gryvia tenant delete acme --yes"]))]
    Tenant {
        #[command(subcommand)]
        action: TenantCommands,
    },

    /// Show metered GPU usage and estimated cost
    ///
    /// Aggregates GryviaUsageRecord objects: GPU hours, cost and distinct jobs per tenant, SKU or day. With --network it reports measured egress bytes per tenant, peer class, zone class or day from GryviaNetworkUsageRecord objects (egress only; estimates).
    /// Records are filtered on their start time; a date-only --to covers that whole day. Costs are
    /// estimates from job run time multiplied by the SKU rate, not invoices.
    #[command(after_help = examples(&["gryvia usage", "gryvia usage --tenant acme --from 2026-09-01 --to 2026-09-30", "gryvia usage --group-by sku -o json", "gryvia usage --group-by day -o csv", "gryvia usage --network --group-by zone-class", "gryvia usage --network --tenant acme --from 2026-09-01 -o csv"]))]
    Usage {
        /// Only this tenant
        #[arg(long)]
        tenant: Option<String>,

        /// Earliest record start: YYYY-MM-DD or an RFC 3339 timestamp
        #[arg(long)]
        from: Option<String>,

        /// Latest record start: YYYY-MM-DD (the whole day is included) or an RFC 3339 timestamp
        #[arg(long)]
        to: Option<String>,

        /// How to group the rows
        #[arg(long, value_enum, default_value_t = GroupBy::Tenant)]
        group_by: GroupBy,

        /// Show measured network egress (GryviaNetworkUsageRecord) instead of GPU usage
        #[arg(long)]
        network: bool,

        /// Output format
        #[arg(short, long, value_enum, default_value_t = UsageFormat::Table, env = "GRYVIA_OUTPUT")]
        output: UsageFormat,
    },

    /// Build monthly invoice estimates from metered usage
    ///
    /// Groups GryviaUsageRecord objects (all namespaces) into one invoice per tenant for a UTC month, with
    /// one line per SKU. Records are assigned to the month of their start time. Invoices are estimates
    /// from job run time; `open` is true while an included job is still running.
    #[command(after_help = examples(&["gryvia invoice", "gryvia invoice --tenant acme --month 2026-09", "gryvia invoice --month 2026-09 -o csv", "gryvia invoice -o json"]))]
    Invoice {
        /// Only this tenant
        #[arg(long)]
        tenant: Option<String>,

        /// Month to invoice, YYYY-MM (UTC; default: the current month)
        #[arg(long, value_parser = commands::invoice::parse_month)]
        month: Option<(i32, u32)>,

        /// Output format
        #[arg(short, long, value_enum, default_value_t = UsageFormat::Table, env = "GRYVIA_OUTPUT")]
        output: UsageFormat,
    },

    /// Interactive job creation wizard
    #[command(after_help = examples(&["gryvia create job"]))]
    Create {
        /// Resource type (job, quota)
        resource: String,
    },

    /// Validate a job YAML file
    #[command(after_help = examples(&["gryvia validate job.yaml"]))]
    Validate {
        /// Path to job YAML file
        file: String,
    },

    /// Show cluster health and diagnostics
    #[command(after_help = examples(&["gryvia health", "gryvia health gpu"]))]
    Health {
        /// Check specific component (gpu, storage, network, all)
        #[arg(default_value = "all")]
        component: String,
    },

    /// Report GPU capacity per type: free, pending demand and shortfall
    ///
    /// Read-only. Shows total, allocated and free GPUs per GPU type. Supply comes from GryviaGpuNode objects, allocation from Running jobs and demand
    /// from Pending/Queued/Scheduling jobs. Jobs that do not name a GPU type (or say "any") are
    /// counted only in the cluster totals. This is a snapshot, not a forecast.
    #[command(visible_alias = "cap")]
    #[command(after_help = examples(&["gryvia capacity", "gryvia capacity --gpu-type H100 -o json"]))]
    Capacity {
        /// Output format
        #[arg(short, long, value_enum, default_value_t = OutputFormat::Table, env = "GRYVIA_OUTPUT")]
        output: OutputFormat,

        /// Only report this GPU type (case-insensitive); jobs without a type are excluded
        #[arg(long)]
        gpu_type: Option<String>,
    },

    /// Generate shell completions
    #[command(after_help = examples(&["gryvia completion bash > /etc/bash_completion.d/gryvia", "gryvia completion zsh > ~/.zsh/completions/_gryvia"]))]
    Completion {
        /// Shell to generate completions for
        #[arg(value_enum)]
        shell: Shell,
    },

    /// Show client and platform versions
    #[command(after_help = examples(&["gryvia version", "gryvia version --client"]))]
    Version {
        /// Only show the client version (needs no cluster)
        #[arg(long)]
        client: bool,

        /// Namespace the platform is installed in
        #[arg(long, default_value = "gryvia-system")]
        platform_namespace: String,
    },

    /// Mark nodes for maintenance: cordon, optionally drain, and list
    ///
    /// Uses the standard Kubernetes cordon flag plus the annotations gryvia.io/maintenance (start
    /// time) and gryvia.io/maintenance-reason. Draining uses the Eviction API only, so
    /// PodDisruptionBudgets are respected; pods are never deleted directly.
    #[command(after_help = examples(&["gryvia maintenance start gpu-node-01 --reason driver-update --drain", "gryvia maintenance list"]))]
    Maintenance {
        #[command(subcommand)]
        action: MaintenanceCommands,
    },

    /// Network intelligence commands
    #[command(after_help = examples(&["gryvia network status", "gryvia network anomalies --severity high"]))]
    Network {
        #[command(subcommand)]
        action: NetworkCommands,
    },

    /// Security detection and enforcement commands
    #[command(after_help = examples(&["gryvia security status", "gryvia security alerts --alert-type blocked"]))]
    Security {
        #[command(subcommand)]
        action: SecurityCommands,
    },

    /// GPU communication and training analysis commands
    #[command(after_help = examples(&["gryvia gpu memory --node gpu-node-01", "gryvia gpu nccl --job llm-training"]))]
    Gpu {
        #[command(subcommand)]
        action: GpuCommands,
    },
}

#[derive(Subcommand)]
enum TenantCommands {
    /// List tenants
    List {
        /// Output format
        #[arg(short, long, value_enum, default_value_t = OutputFormat::Table, env = "GRYVIA_OUTPUT")]
        output: OutputFormat,
    },

    /// Show one tenant
    Get {
        /// Tenant name
        name: String,

        /// Output format
        #[arg(short, long, value_enum, default_value_t = OutputFormat::Table)]
        output: OutputFormat,
    },

    /// Create (or update) a tenant
    Create {
        /// Tenant name (becomes the namespace tenant-<name>)
        name: String,

        /// Human-readable name (defaults to the tenant name)
        #[arg(long)]
        display_name: Option<String>,

        /// SKU the tenant may use; repeat for several (default: all enabled SKUs)
        #[arg(long = "allowed-sku")]
        allowed_sku: Vec<String>,

        /// Maximum concurrent GPUs
        #[arg(long)]
        max_gpus: Option<u32>,

        /// Isolate the tenant's network from other tenants
        #[arg(long, value_name = "BOOL", action = clap::ArgAction::Set)]
        isolated: Option<bool>,
    },

    /// Delete a tenant
    Delete {
        /// Tenant name
        name: String,

        /// Skip confirmation
        #[arg(short, long)]
        yes: bool,
    },
}

#[derive(Subcommand)]
enum MaintenanceCommands {
    /// Cordon a node and mark it as under maintenance
    Start {
        /// Kubernetes node name
        node: String,

        /// Why the node is being taken out of service (stored in the annotation)
        #[arg(long)]
        reason: Option<String>,

        /// Also evict the node's pods via the Eviction API (skips DaemonSet and mirror pods;
        /// pods blocked by a PodDisruptionBudget are reported and left running)
        #[arg(long)]
        drain: bool,

        /// Skip the confirmation prompt for --drain
        #[arg(short, long)]
        yes: bool,
    },

    /// Uncordon a node and remove the maintenance annotations
    End {
        /// Kubernetes node name
        node: String,
    },

    /// List nodes marked for maintenance, with age and reason
    List {
        /// Output format
        #[arg(short, long, value_enum, default_value_t = OutputFormat::Table, env = "GRYVIA_OUTPUT")]
        output: OutputFormat,
    },
}

#[derive(Subcommand)]
enum NetworkCommands {
    /// Trace network flows for a service
    Trace {
        /// Target service name
        service: String,

        /// Trace duration (e.g., 2m, 5m, 1h)
        #[arg(short, long, default_value = "2m")]
        duration: String,

        /// Capture level (l3, l4, l7)
        #[arg(short, long, default_value = "l7")]
        level: String,

        /// Target namespace
        #[arg(long, default_value = "default")]
        trace_namespace: String,

        /// Follow / live stream mode
        #[arg(short, long)]
        follow: bool,
    },

    /// Show recent network flows
    Flows {
        /// Filter by service name
        #[arg(short, long)]
        service: Option<String>,

        /// Target namespace
        #[arg(long, default_value = "default")]
        flow_namespace: String,

        /// Time window (e.g., 5m, 1h, 24h)
        #[arg(short, long, default_value = "5m")]
        last: String,

        /// Output format
        #[arg(short, long, value_enum, default_value_t = OutputFormat::Table, env = "GRYVIA_OUTPUT")]
        output: OutputFormat,
    },

    /// Show service dependency graph
    Graph {
        /// Target namespace
        #[arg(long, default_value = "default")]
        graph_namespace: String,

        /// Output format (ascii, json)
        #[arg(short, long, default_value = "ascii")]
        format: String,
    },

    /// Manage flow policies
    Policy {
        #[command(subcommand)]
        action: PolicyCommands,
    },

    /// Show network health overview
    Status {
        /// Target namespace
        #[arg(long, default_value = "default")]
        status_namespace: String,
    },

    /// Show detected network anomalies
    Anomalies {
        /// Filter by service
        #[arg(short, long)]
        service: Option<String>,

        /// Filter by severity (critical, high, medium, low)
        #[arg(long)]
        severity: Option<String>,

        /// Target namespace
        #[arg(long, default_value = "default")]
        anomaly_namespace: String,

        /// Output format
        #[arg(short, long, value_enum, default_value_t = OutputFormat::Table, env = "GRYVIA_OUTPUT")]
        output: OutputFormat,
    },
}

#[derive(Subcommand)]
enum PolicyCommands {
    /// Show auto-generated policy suggestions
    Suggest {
        /// Target namespace
        #[arg(long, default_value = "default")]
        policy_namespace: String,
    },

    /// Apply a suggested policy
    Apply {
        /// Policy name to apply
        policy_name: String,

        /// Target namespace
        #[arg(long, default_value = "default")]
        policy_namespace: String,
    },

    /// List all flow policies
    List {
        /// Target namespace
        #[arg(long, default_value = "default")]
        policy_namespace: String,

        /// Output format
        #[arg(short, long, value_enum, default_value_t = OutputFormat::Table, env = "GRYVIA_OUTPUT")]
        output: OutputFormat,
    },
}

#[derive(Subcommand)]
enum SecurityCommands {
    /// Show security alerts
    Alerts {
        /// Filter by severity (critical, high, medium, low)
        #[arg(long)]
        severity: Option<String>,

        /// Filter by alert type (escape, mining, exfiltration, privesc)
        #[arg(long, name = "type")]
        alert_type: Option<String>,

        /// Target namespace
        #[arg(long, default_value = "default")]
        security_namespace: String,
    },

    /// Show security status overview
    Status {
        /// Target namespace
        #[arg(long, default_value = "default")]
        security_namespace: String,
    },

    /// Manage security policies
    Policy {
        #[command(subcommand)]
        action: SecurityPolicyCommands,
    },
}

#[derive(Subcommand)]
enum SecurityPolicyCommands {
    /// List security policies
    List {
        /// Target namespace
        #[arg(long, default_value = "default")]
        security_namespace: String,

        /// Output format
        #[arg(short, long, value_enum, default_value_t = OutputFormat::Table, env = "GRYVIA_OUTPUT")]
        output: OutputFormat,
    },

    /// Create a security policy
    Create {
        /// Policy name
        name: String,

        /// Target namespaces (comma-separated)
        #[arg(long)]
        namespaces: String,

        /// Detection rules to enable (comma-separated: escape,mining,exfiltration,privesc,driver_fim)
        #[arg(long)]
        rules: String,

        /// Enable automatic blocking of detected threats
        #[arg(long)]
        auto_block: bool,

        /// Target namespace for the policy CR
        #[arg(long, default_value = "default")]
        security_namespace: String,
    },
}

#[derive(Subcommand)]
enum GpuCommands {
    /// Show NCCL collective operation stats for a job
    Nccl {
        /// Job name
        #[arg(long)]
        job: String,

        /// Follow / live stream mode
        #[arg(short, long)]
        follow: bool,

        /// Target namespace
        #[arg(long, default_value = "default")]
        gpu_namespace: String,
    },

    /// Show GPU memory transfer stats for a node
    Memory {
        /// Node name
        #[arg(long)]
        node: String,

        /// Target namespace
        #[arg(long, default_value = "default")]
        gpu_namespace: String,
    },

    /// Show RDMA stats for a node
    Rdma {
        /// Node name
        #[arg(long)]
        node: String,

        /// Target namespace
        #[arg(long, default_value = "default")]
        gpu_namespace: String,
    },

    /// Show training insights for a job
    Training {
        /// Job name
        #[arg(long)]
        job: String,

        /// Target namespace
        #[arg(long, default_value = "default")]
        gpu_namespace: String,
    },
}

#[tokio::main]
async fn main() {
    if let Err(err) = run().await {
        for line in display::fatal_lines(&err, ui::color_enabled()) {
            eprintln!("{line}");
        }
        std::process::exit(1);
    }
}

async fn run() -> Result<()> {
    let args: Vec<String> = std::env::args().collect();
    let no_color = ui::args_request_no_color(&args);
    ui::init(no_color);

    // Root help is rendered here so commands can be shown in named groups.
    if ui::help::wants_root_help(&args) {
        let color = colored::control::SHOULD_COLORIZE.should_colorize();
        print!("{}", ui::help::render_root_help(&Cli::command(), color));
        return Ok(());
    }

    let mut command = Cli::command();
    if no_color {
        command = command.color(ColorChoice::Never);
    }
    let matches = command.get_matches_from(&args);
    let cli = Cli::from_arg_matches(&matches).unwrap_or_else(|e| e.exit());

    // Setup logging
    // Library log lines (for example the kube client's) are noise in normal use; show them with --verbose or RUST_LOG.
    let default_level = if cli.verbose { "debug" } else { "error" };
    let filter = tracing_subscriber::EnvFilter::try_from_default_env()
        .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new(default_level));
    tracing_subscriber::fmt().with_env_filter(filter).init();

    // Commands that never touch the cluster run before a kube client is built, so they work with no kubeconfig.
    match &cli.command {
        Commands::Completion { shell } => {
            clap_complete::generate(
                *shell,
                &mut Cli::command(),
                "gryvia",
                &mut std::io::stdout(),
            );
            return Ok(());
        }
        Commands::Validate { file } => {
            commands::validate::execute(file).await?;
            return Ok(());
        }
        Commands::Version { client: true, .. } => {
            commands::version::print_client();
            return Ok(());
        }
        _ => {}
    }

    // Save namespace flag before passing ownership to client
    let namespace_flag = cli.namespace.clone();

    // Create Kubernetes client
    let client = client::GryviaClient::new(cli.context, cli.namespace).await?;

    // Execute command
    match cli.command {
        Commands::Submit { file, wait, logs } => {
            commands::submit::execute(&client, &file, wait, logs).await?;
        }
        Commands::List {
            resource,
            all_namespaces,
            output,
        } => {
            commands::list::execute(&client, &resource, all_namespaces, output.as_str()).await?;
        }
        Commands::Get {
            resource,
            name,
            output,
        } => {
            commands::get::execute(&client, &resource, &name, output.as_str()).await?;
        }
        Commands::Delete {
            resource,
            name,
            yes,
        } => {
            commands::delete::execute(&client, &resource, &name, yes).await?;
        }
        Commands::Status {
            job: Some(job),
            follow,
            ..
        } => {
            commands::status::execute(&client, &job, follow).await?;
        }
        Commands::Status {
            job: None,
            node,
            brief,
            wait,
            wait_timeout,
            output,
            platform_namespace,
            ..
        } => {
            let opts = commands::platform_status::Options {
                namespace: platform_namespace,
                node,
                brief,
                wait,
                wait_timeout: commands::platform_status::parse_duration(&wait_timeout)?,
                output,
            };
            let code = commands::platform_status::execute(&client, opts).await?;
            if code != 0 {
                std::process::exit(code);
            }
        }
        Commands::Logs {
            job,
            follow,
            tail,
            replica,
        } => {
            commands::logs::execute(&client, &job, follow, tail, replica).await?;
        }
        Commands::Cancel { jobs, yes } => {
            commands::cancel::execute(&client, &jobs, yes).await?;
        }
        Commands::Cluster {
            detailed,
            watch,
            output,
        } => {
            commands::cluster::execute(&client, detailed, watch, output.as_str()).await?;
        }
        Commands::Quota {
            team,
            budget,
            output,
        } => {
            commands::quota::execute(&client, team, budget, output.as_str()).await?;
        }
        Commands::Cost {
            team,
            period,
            detailed,
        } => {
            commands::cost::execute(&client, team, &period, detailed).await?;
        }
        Commands::Queue {
            name,
            watch,
            output,
        } => {
            commands::queue::execute(&client, name, watch, output.as_str()).await?;
        }
        Commands::Catalog { output } => {
            commands::catalog::execute(&client, output.as_str()).await?;
        }
        Commands::Tenant { action } => match action {
            TenantCommands::List { output } => {
                commands::tenant::list(&client, output.as_str()).await?;
            }
            TenantCommands::Get { name, output } => {
                commands::tenant::get(&client, &name, output.as_str()).await?;
            }
            TenantCommands::Create {
                name,
                display_name,
                allowed_sku,
                max_gpus,
                isolated,
            } => {
                commands::tenant::create(
                    &client,
                    &name,
                    display_name.as_deref(),
                    &allowed_sku,
                    max_gpus,
                    isolated,
                )
                .await?;
            }
            TenantCommands::Delete { name, yes } => {
                commands::tenant::delete(&client, &name, yes).await?;
            }
        },
        Commands::Usage {
            tenant,
            from,
            to,
            group_by,
            network,
            output,
        } => {
            if network {
                let opts = commands::network_usage::Options {
                    tenant,
                    from,
                    to,
                    group_by,
                    output,
                };
                commands::network_usage::execute(&client, opts).await?;
            } else {
                let opts = commands::usage::Options {
                    tenant,
                    from,
                    to,
                    group_by,
                    output,
                };
                commands::usage::execute(&client, opts).await?;
            }
        }
        Commands::Invoice {
            tenant,
            month,
            output,
        } => {
            let opts = commands::invoice::Options {
                tenant,
                month,
                output,
            };
            commands::invoice::execute(&client, opts).await?;
        }
        Commands::Create { resource } => {
            commands::create::execute(&client, &resource).await?;
        }
        Commands::Completion { .. } | Commands::Validate { .. } => {
            unreachable!("handled before the cluster client is created")
        }
        Commands::Version {
            platform_namespace, ..
        } => {
            commands::version::execute(&client, &platform_namespace).await?;
        }
        Commands::Health { component } => {
            commands::health::execute(&client, &component).await?;
        }
        Commands::Capacity { output, gpu_type } => {
            commands::capacity::execute(&client, output.as_str(), gpu_type.as_deref()).await?;
        }
        Commands::Maintenance { action } => match action {
            MaintenanceCommands::Start {
                node,
                reason,
                drain,
                yes,
            } => {
                commands::maintenance::start(&client, &node, reason.as_deref(), drain, yes).await?;
            }
            MaintenanceCommands::End { node } => {
                commands::maintenance::end(&client, &node).await?;
            }
            MaintenanceCommands::List { output } => {
                commands::maintenance::list(&client, output.as_str()).await?;
            }
        },
        Commands::Network { action } => {
            let has_ns_flag = namespace_flag.is_some();
            let ns_default = client.namespace().to_string();
            match action {
                NetworkCommands::Trace {
                    service,
                    duration,
                    level,
                    trace_namespace,
                    follow,
                } => {
                    let effective_ns = if has_ns_flag {
                        &ns_default
                    } else {
                        &trace_namespace
                    };
                    commands::trace::execute(
                        &client,
                        &service,
                        &duration,
                        &level,
                        effective_ns,
                        follow,
                    )
                    .await?;
                }
                NetworkCommands::Flows {
                    service,
                    flow_namespace,
                    last,
                    output,
                } => {
                    let effective_ns = if has_ns_flag {
                        &ns_default
                    } else {
                        &flow_namespace
                    };
                    commands::flows::execute(
                        &client,
                        service.as_deref(),
                        effective_ns,
                        &last,
                        output.as_str(),
                    )
                    .await?;
                }
                NetworkCommands::Graph {
                    graph_namespace,
                    format,
                } => {
                    let effective_ns = if has_ns_flag {
                        &ns_default
                    } else {
                        &graph_namespace
                    };
                    commands::graph::execute(&client, effective_ns, &format).await?;
                }
                NetworkCommands::Policy {
                    action: policy_action,
                } => match policy_action {
                    PolicyCommands::Suggest { policy_namespace } => {
                        let effective_ns = if has_ns_flag {
                            &ns_default
                        } else {
                            &policy_namespace
                        };
                        commands::policy::execute(
                            &client,
                            commands::policy::PolicyAction::Suggest {
                                namespace: effective_ns.to_string(),
                            },
                        )
                        .await?;
                    }
                    PolicyCommands::Apply {
                        policy_name,
                        policy_namespace,
                    } => {
                        let effective_ns = if has_ns_flag {
                            &ns_default
                        } else {
                            &policy_namespace
                        };
                        commands::policy::execute(
                            &client,
                            commands::policy::PolicyAction::Apply {
                                policy_name,
                                namespace: effective_ns.to_string(),
                            },
                        )
                        .await?;
                    }
                    PolicyCommands::List {
                        policy_namespace,
                        output,
                    } => {
                        let effective_ns = if has_ns_flag {
                            &ns_default
                        } else {
                            &policy_namespace
                        };
                        commands::policy::execute(
                            &client,
                            commands::policy::PolicyAction::List {
                                namespace: effective_ns.to_string(),
                                output: output.as_str().to_string(),
                            },
                        )
                        .await?;
                    }
                },
                NetworkCommands::Status { status_namespace } => {
                    let effective_ns = if has_ns_flag {
                        &ns_default
                    } else {
                        &status_namespace
                    };
                    commands::network::execute_status(&client, effective_ns).await?;
                }
                NetworkCommands::Anomalies {
                    service,
                    severity,
                    anomaly_namespace,
                    output,
                } => {
                    let effective_ns = if has_ns_flag {
                        &ns_default
                    } else {
                        &anomaly_namespace
                    };
                    commands::network::execute_anomalies(
                        &client,
                        effective_ns,
                        service.as_deref(),
                        severity.as_deref(),
                        output.as_str(),
                    )
                    .await?;
                }
            }
        }
        Commands::Security { action } => {
            let has_ns_flag = namespace_flag.is_some();
            let ns_default = client.namespace().to_string();
            match action {
                SecurityCommands::Alerts {
                    severity,
                    alert_type,
                    security_namespace,
                } => {
                    let effective_ns = if has_ns_flag {
                        &ns_default
                    } else {
                        &security_namespace
                    };
                    commands::security::execute(
                        &client,
                        commands::security::SecurityAction::Alerts {
                            severity,
                            alert_type,
                            namespace: effective_ns.to_string(),
                        },
                    )
                    .await?;
                }
                SecurityCommands::Status { security_namespace } => {
                    let effective_ns = if has_ns_flag {
                        &ns_default
                    } else {
                        &security_namespace
                    };
                    commands::security::execute(
                        &client,
                        commands::security::SecurityAction::Status {
                            namespace: effective_ns.to_string(),
                        },
                    )
                    .await?;
                }
                SecurityCommands::Policy {
                    action: policy_action,
                } => match policy_action {
                    SecurityPolicyCommands::List {
                        security_namespace,
                        output,
                    } => {
                        let effective_ns = if has_ns_flag {
                            &ns_default
                        } else {
                            &security_namespace
                        };
                        commands::security::execute(
                            &client,
                            commands::security::SecurityAction::PolicyList {
                                namespace: effective_ns.to_string(),
                                output: output.as_str().to_string(),
                            },
                        )
                        .await?;
                    }
                    SecurityPolicyCommands::Create {
                        name,
                        namespaces,
                        rules,
                        auto_block,
                        security_namespace,
                    } => {
                        let effective_ns = if has_ns_flag {
                            &ns_default
                        } else {
                            &security_namespace
                        };
                        let ns_list: Vec<String> = namespaces
                            .split(',')
                            .map(|s| s.trim().to_string())
                            .collect();
                        let rule_list: Vec<String> =
                            rules.split(',').map(|s| s.trim().to_string()).collect();
                        commands::security::execute(
                            &client,
                            commands::security::SecurityAction::PolicyCreate {
                                name,
                                namespaces: ns_list,
                                rules: rule_list,
                                auto_block,
                                namespace: effective_ns.to_string(),
                            },
                        )
                        .await?;
                    }
                },
            }
        }
        Commands::Gpu { action } => {
            let has_ns_flag = namespace_flag.is_some();
            let ns_default = client.namespace().to_string();
            match action {
                GpuCommands::Nccl {
                    job,
                    follow,
                    gpu_namespace,
                } => {
                    let effective_ns = if has_ns_flag {
                        &ns_default
                    } else {
                        &gpu_namespace
                    };
                    commands::gpu_trace::execute(
                        &client,
                        commands::gpu_trace::GpuAction::Nccl {
                            job,
                            follow,
                            namespace: effective_ns.to_string(),
                        },
                    )
                    .await?;
                }
                GpuCommands::Memory {
                    node,
                    gpu_namespace,
                } => {
                    let effective_ns = if has_ns_flag {
                        &ns_default
                    } else {
                        &gpu_namespace
                    };
                    commands::gpu_trace::execute(
                        &client,
                        commands::gpu_trace::GpuAction::Memory {
                            node,
                            namespace: effective_ns.to_string(),
                        },
                    )
                    .await?;
                }
                GpuCommands::Rdma {
                    node,
                    gpu_namespace,
                } => {
                    let effective_ns = if has_ns_flag {
                        &ns_default
                    } else {
                        &gpu_namespace
                    };
                    commands::gpu_trace::execute(
                        &client,
                        commands::gpu_trace::GpuAction::Rdma {
                            node,
                            namespace: effective_ns.to_string(),
                        },
                    )
                    .await?;
                }
                GpuCommands::Training { job, gpu_namespace } => {
                    let effective_ns = if has_ns_flag {
                        &ns_default
                    } else {
                        &gpu_namespace
                    };
                    commands::gpu_trace::execute(
                        &client,
                        commands::gpu_trace::GpuAction::Training {
                            job,
                            namespace: effective_ns.to_string(),
                        },
                    )
                    .await?;
                }
            }
        }
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Every `gryvia ...` line in a command's "Examples:" help must parse with the real argument definitions.
    #[test]
    fn help_examples_parse() {
        fn collect(cmd: &clap::Command, out: &mut Vec<String>) {
            if let Some(text) = cmd.get_after_help() {
                for line in text.to_string().lines() {
                    let line = line.trim();
                    if line.starts_with("gryvia ") {
                        out.push(line.split(" > ").next().unwrap().to_string());
                    }
                }
            }
            for sub in cmd.get_subcommands() {
                collect(sub, out);
            }
        }
        let mut examples = Vec::new();
        collect(&Cli::command(), &mut examples);
        assert!(
            examples.len() >= 25,
            "expected examples on most commands, got {}",
            examples.len()
        );
        for line in examples {
            let args: Vec<&str> = line.split_whitespace().collect();
            if let Err(e) = Cli::command().try_get_matches_from(args) {
                panic!("help example does not parse: `{line}`: {e}");
            }
        }
    }

    #[test]
    fn every_top_level_command_has_an_example() {
        for sub in Cli::command().get_subcommands() {
            if sub.get_name() == "help" {
                continue;
            }
            assert!(
                sub.get_after_help().is_some(),
                "`{}` needs an Examples block",
                sub.get_name()
            );
        }
    }

    #[test]
    fn visible_aliases_resolve_to_their_commands() {
        for (alias, target) in [
            ("ls", "list"),
            ("rm", "delete"),
            ("describe", "get"),
            ("cap", "capacity"),
            ("skus", "catalog"),
        ] {
            let m = Cli::command()
                .try_get_matches_from(match target {
                    "list" => vec!["gryvia", alias, "jobs"],
                    "delete" => vec!["gryvia", alias, "job", "x"],
                    "get" => vec!["gryvia", alias, "job", "x"],
                    _ => vec!["gryvia", alias],
                })
                .unwrap();
            assert_eq!(m.subcommand_name(), Some(target));
        }
    }

    #[test]
    fn data_commands_take_an_output_format() {
        for args in [
            vec!["gryvia", "cluster", "-o", "json"],
            vec!["gryvia", "quota", "-o", "yaml"],
            vec!["gryvia", "queue", "-o", "json"],
            vec!["gryvia", "maintenance", "list", "-o", "json"],
            vec!["gryvia", "capacity", "-o", "yaml"],
            vec!["gryvia", "status", "-o", "json"],
            vec!["gryvia", "catalog", "-o", "json"],
            vec!["gryvia", "tenant", "list", "-o", "yaml"],
            vec!["gryvia", "tenant", "get", "acme", "-o", "json"],
            vec!["gryvia", "usage", "-o", "csv"],
            vec!["gryvia", "usage", "--group-by", "day", "-o", "json"],
            vec![
                "gryvia",
                "usage",
                "--network",
                "--group-by",
                "zone-class",
                "-o",
                "csv",
            ],
            vec!["gryvia", "invoice", "-o", "csv"],
            vec![
                "gryvia", "invoice", "--tenant", "acme", "--month", "2026-09", "-o", "yaml",
            ],
        ] {
            assert!(
                Cli::command().try_get_matches_from(args.clone()).is_ok(),
                "{args:?} should parse"
            );
        }
        assert!(Cli::command()
            .try_get_matches_from(["gryvia", "quota", "-o", "csv"])
            .is_err());
    }

    #[test]
    fn tenant_create_takes_repeated_skus_and_a_bool() {
        let m = Cli::command()
            .try_get_matches_from([
                "gryvia",
                "tenant",
                "create",
                "acme",
                "--allowed-sku",
                "a",
                "--allowed-sku",
                "b",
                "--max-gpus",
                "8",
                "--isolated",
                "false",
            ])
            .unwrap();
        let (_, create) = m
            .subcommand_matches("tenant")
            .unwrap()
            .subcommand()
            .unwrap();
        assert_eq!(create.get_many::<String>("allowed_sku").unwrap().count(), 2);
        assert_eq!(create.get_one::<bool>("isolated"), Some(&false));
        assert!(Cli::command()
            .try_get_matches_from(["gryvia", "tenant", "create", "acme", "--max-gpus", "x"])
            .is_err());
        assert!(Cli::command()
            .try_get_matches_from(["gryvia", "usage", "--group-by", "week"])
            .is_err());
        for bad in ["2026-13", "2026-9", "sept", "2026-09-01", ""] {
            assert!(
                Cli::command()
                    .try_get_matches_from(["gryvia", "invoice", "--month", bad])
                    .is_err(),
                "--month {bad:?} should be a usage error"
            );
        }
        assert!(Cli::command()
            .try_get_matches_from(["gryvia", "catalog", "-o", "csv"])
            .is_err());
    }

    #[test]
    fn output_flag_accepts_only_known_formats() {
        assert!(Cli::command()
            .try_get_matches_from(["gryvia", "list", "jobs", "-o", "json"])
            .is_ok());
        assert!(Cli::command()
            .try_get_matches_from(["gryvia", "list", "jobs", "-o", "xml"])
            .is_err());
        assert!(Cli::command()
            .try_get_matches_from(["gryvia", "get", "job", "x", "-o", "table"])
            .is_err());
    }
}
