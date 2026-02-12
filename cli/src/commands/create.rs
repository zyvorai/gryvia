use anyhow::Result;
use crate::client::KubeFabricClient;
use crate::display;

pub async fn execute(_client: &KubeFabricClient, resource: &str) -> Result<()> {
    display::print_info(&format!("Creating {}", resource));
    display::print_warning("Interactive creation wizard not yet implemented");
    display::print_info("Use 'kubefabric submit -f <file>' to create from YAML");
    Ok(())
}
