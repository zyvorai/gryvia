mod commands;
mod client;
mod display;
mod types;

use clap::{Parser, Subcommand};
use anyhow::Result;

#[derive(Parser)]
#[command(name = "tensorreaper")]
#[command(about = "TensorReaper CLI - Manage GPU clusters for AI workloads", long_about = None)]
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
        /// Resource type to list (jobs, quotas, nodes)
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

    /// Network intelligence commands
    Network {
        #[command(subcommand)]
        action: NetworkCommands,
    },

    /// Security detection and enforcement commands
    Security {
        #[command(subcommand)]
        action: SecurityCommands,
    },

    /// GPU communication and training analysis commands
    Gpu {
        #[command(subcommand)]
        action: GpuCommands,
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

        /// Output format (table, json)
        #[arg(short, long, default_value = "table")]
        output: String,
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

        /// Output format (table, json)
        #[arg(short, long, default_value = "table")]
        output: String,
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

        /// Output format (table, json, yaml)
        #[arg(short, long, default_value = "table")]
        output: String,
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

        /// Output format (table, json)
        #[arg(short, long, default_value = "table")]
        output: String,
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
async fn main() -> Result<()> {
    let cli = Cli::parse();

    // Setup logging
    let log_level = if cli.verbose { "debug" } else { "info" };
    tracing_subscriber::fmt()
        .with_env_filter(log_level)
        .init();

    // Save namespace flag before passing ownership to client
    let namespace_flag = cli.namespace.clone();

    // Create Kubernetes client
    let client = client::TensorReaperClient::new(cli.context, cli.namespace).await?;

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
                    let effective_ns = if has_ns_flag { &ns_default } else { &trace_namespace };
                    commands::trace::execute(&client, &service, &duration, &level, effective_ns, follow).await?;
                }
                NetworkCommands::Flows {
                    service,
                    flow_namespace,
                    last,
                    output,
                } => {
                    let effective_ns = if has_ns_flag { &ns_default } else { &flow_namespace };
                    commands::flows::execute(&client, service.as_deref(), effective_ns, &last, &output).await?;
                }
                NetworkCommands::Graph {
                    graph_namespace,
                    format,
                } => {
                    let effective_ns = if has_ns_flag { &ns_default } else { &graph_namespace };
                    commands::graph::execute(&client, effective_ns, &format).await?;
                }
                NetworkCommands::Policy { action: policy_action } => {
                    match policy_action {
                        PolicyCommands::Suggest { policy_namespace } => {
                            let effective_ns = if has_ns_flag { &ns_default } else { &policy_namespace };
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
                            let effective_ns = if has_ns_flag { &ns_default } else { &policy_namespace };
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
                            let effective_ns = if has_ns_flag { &ns_default } else { &policy_namespace };
                            commands::policy::execute(
                                &client,
                                commands::policy::PolicyAction::List {
                                    namespace: effective_ns.to_string(),
                                    output,
                                },
                            )
                            .await?;
                        }
                    }
                }
                NetworkCommands::Status { status_namespace } => {
                    let effective_ns = if has_ns_flag { &ns_default } else { &status_namespace };
                    commands::network::execute_status(&client, effective_ns).await?;
                }
                NetworkCommands::Anomalies {
                    service,
                    severity,
                    anomaly_namespace,
                    output,
                } => {
                    let effective_ns = if has_ns_flag { &ns_default } else { &anomaly_namespace };
                    commands::network::execute_anomalies(
                        &client,
                        effective_ns,
                        service.as_deref(),
                        severity.as_deref(),
                        &output,
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
                    let effective_ns = if has_ns_flag { &ns_default } else { &security_namespace };
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
                    let effective_ns = if has_ns_flag { &ns_default } else { &security_namespace };
                    commands::security::execute(
                        &client,
                        commands::security::SecurityAction::Status {
                            namespace: effective_ns.to_string(),
                        },
                    )
                    .await?;
                }
                SecurityCommands::Policy { action: policy_action } => {
                    match policy_action {
                        SecurityPolicyCommands::List {
                            security_namespace,
                            output,
                        } => {
                            let effective_ns = if has_ns_flag { &ns_default } else { &security_namespace };
                            commands::security::execute(
                                &client,
                                commands::security::SecurityAction::PolicyList {
                                    namespace: effective_ns.to_string(),
                                    output,
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
                            let effective_ns = if has_ns_flag { &ns_default } else { &security_namespace };
                            let ns_list: Vec<String> = namespaces.split(',').map(|s| s.trim().to_string()).collect();
                            let rule_list: Vec<String> = rules.split(',').map(|s| s.trim().to_string()).collect();
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
                    }
                }
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
                    let effective_ns = if has_ns_flag { &ns_default } else { &gpu_namespace };
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
                    let effective_ns = if has_ns_flag { &ns_default } else { &gpu_namespace };
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
                    let effective_ns = if has_ns_flag { &ns_default } else { &gpu_namespace };
                    commands::gpu_trace::execute(
                        &client,
                        commands::gpu_trace::GpuAction::Rdma {
                            node,
                            namespace: effective_ns.to_string(),
                        },
                    )
                    .await?;
                }
                GpuCommands::Training {
                    job,
                    gpu_namespace,
                } => {
                    let effective_ns = if has_ns_flag { &ns_default } else { &gpu_namespace };
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
