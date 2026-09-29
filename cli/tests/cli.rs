//! End-to-end checks of the built binary. None of these need a cluster: they run with no kubeconfig.

use assert_cmd::Command;
use predicates::prelude::*;

fn gryvia() -> Command {
    let mut cmd = Command::cargo_bin("gryvia").unwrap();
    cmd.env("KUBECONFIG", "/nonexistent/kubeconfig")
        .env("HOME", "/nonexistent")
        .env_remove("NO_COLOR")
        .env_remove("CLICOLOR_FORCE");
    cmd
}

#[test]
fn root_help_is_grouped_and_needs_no_cluster() {
    gryvia()
        .arg("--help")
        .assert()
        .success()
        .stdout(predicate::str::contains("Workloads:"))
        .stdout(predicate::str::contains("Cluster:"))
        .stdout(predicate::str::contains("Operations:"))
        .stdout(predicate::str::contains("Observe:"))
        .stdout(predicate::str::contains("Utilities:"))
        .stdout(predicate::str::contains("--no-color"));
}

#[test]
fn no_arguments_shows_the_same_help() {
    gryvia()
        .assert()
        .success()
        .stdout(predicate::str::contains("Workloads:"));
}

#[test]
fn piped_help_has_no_escape_codes() {
    gryvia()
        .arg("--help")
        .assert()
        .success()
        .stdout(predicate::str::contains('\x1b').not());
}

#[test]
fn forced_color_adds_escape_codes_and_no_color_wins() {
    gryvia()
        .env("CLICOLOR_FORCE", "1")
        .arg("--help")
        .assert()
        .success()
        .stdout(predicate::str::contains('\x1b'));
    gryvia()
        .env("CLICOLOR_FORCE", "1")
        .arg("--no-color")
        .arg("--help")
        .assert()
        .success()
        .stdout(predicate::str::contains('\x1b').not());
    gryvia()
        .env("CLICOLOR_FORCE", "1")
        .env("NO_COLOR", "1")
        .arg("--help")
        .assert()
        .success()
        .stdout(predicate::str::contains('\x1b').not());
}

#[test]
fn subcommand_help_has_examples() {
    gryvia()
        .args(["list", "--help"])
        .assert()
        .success()
        .stdout(predicate::str::contains("Examples:"))
        .stdout(predicate::str::contains("gryvia list jobs"));
}

#[test]
fn completions_generate_for_each_shell() {
    for shell in ["bash", "zsh", "fish", "powershell"] {
        gryvia()
            .args(["completion", shell])
            .assert()
            .success()
            .stdout(predicate::str::contains("gryvia"));
    }
}

#[test]
fn version_client_needs_no_cluster() {
    gryvia()
        .args(["version", "--client"])
        .assert()
        .success()
        .stdout(predicate::str::contains("gryvia "));
}

#[test]
fn unknown_flags_and_bad_formats_exit_2() {
    gryvia().arg("--bogus").assert().code(2);
    gryvia()
        .args(["list", "jobs", "-o", "xml"])
        .assert()
        .code(2);
}

#[test]
fn validate_works_without_a_cluster() {
    let path = std::env::temp_dir().join(format!("gryvia-cli-test-{}.yaml", std::process::id()));
    std::fs::write(
        &path,
        "apiVersion: gryvia.io/v1alpha1\nkind: GryviaAIJob\nmetadata:\n  name: t\nspec:\n  type: training\n  gpus: 1\n  image: busybox\n",
    )
    .unwrap();
    gryvia()
        .args(["validate", path.to_str().unwrap()])
        .assert()
        .success();
    let _ = std::fs::remove_file(path);
}
