use anyhow::{Context, Result};
use std::fs;
use serde_yaml;
use kube::core::DynamicObject;

use crate::display;

pub async fn execute(file: &str) -> Result<()> {
    display::print_info(&format!("Validating: {}", file));

    let contents = fs::read_to_string(file)
        .with_context(|| format!("Failed to read file: {}", file))?;

    let _job: DynamicObject = serde_yaml::from_str(&contents)
        .context("Failed to parse YAML file")?;

    display::print_success("YAML file is valid");

    // TODO: Add schema validation
    display::print_warning("Full schema validation not yet implemented");

    Ok(())
}
