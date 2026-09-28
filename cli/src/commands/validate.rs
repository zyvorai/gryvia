use anyhow::{Context, Result};
use std::fs;
use serde_yaml;
use kube::core::DynamicObject;

use crate::display;

pub async fn execute(file: &str) -> Result<()> {
    display::print_info(&format!("Validating: {}", file));

    let contents = fs::read_to_string(file)
        .with_context(|| format!("Failed to read file: {}", file))?;

    let obj: DynamicObject = serde_yaml::from_str(&contents)
        .context("Failed to parse YAML file")?;

    display::print_success("YAML file is valid");

    // Validate that it's a known Gryvia resource type
    let known_kinds = ["FabricAIJob", "FabricQuota", "FabricGpuNode", "FabricStorage", "FabricNetwork"];
    if let Some(ref types) = obj.types {
        if types.api_version != "gryvia.io/v1" {
            display::print_warning(&format!("apiVersion '{}' is not gryvia.io/v1", types.api_version));
        }
        if !known_kinds.contains(&types.kind.as_str()) {
            display::print_warning(&format!("kind '{}' is not a known Gryvia resource type", types.kind));
        } else {
            display::print_success(&format!("Resource type '{}' is valid", types.kind));
        }
    } else {
        display::print_warning("Missing apiVersion/kind in YAML");
    }

    Ok(())
}
