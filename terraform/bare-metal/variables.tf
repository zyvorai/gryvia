variable "cluster_name" {
  description = "Name of the KubeFabric cluster"
  type        = string
  default     = "kubefabric-baremetal"
}

variable "control_plane_endpoint" {
  description = "Control plane endpoint (load balancer or single master IP)"
  type        = string
}

variable "control_nodes" {
  description = "Control plane nodes"
  type = list(object({
    name = string
    ip   = string
  }))
}

variable "gpu_nodes" {
  description = "GPU worker nodes"
  type = list(object({
    name          = string
    ip            = string
    gpu_type      = string # H100, A100, L40, V100, T4
    gpu_count     = number
    rdma_enabled  = bool
    rdma_device   = string # e.g., mlx5_0, ib0
    storage_mount = string # e.g., /mnt/vast
  }))
}

variable "pod_subnet" {
  description = "Pod network CIDR"
  type        = string
  default     = "10.244.0.0/16"
}

variable "service_subnet" {
  description = "Service network CIDR"
  type        = string
  default     = "10.96.0.0/12"
}

variable "rdma_subnet" {
  description = "RDMA network CIDR (InfiniBand/RoCE)"
  type        = string
  default     = "192.168.100.0/24"
}

variable "storage_backend" {
  description = "Storage backend (vast, weka, ddn, lustre)"
  type        = string
  default     = "vast"
}

variable "storage_endpoint" {
  description = "Storage cluster endpoint"
  type        = string
}

variable "storage_mountpath" {
  description = "Storage mount path on nodes"
  type        = string
  default     = "/mnt/kubefabric-storage"
}

variable "storage_rdma_enabled" {
  description = "Enable RDMA for storage access"
  type        = bool
  default     = true
}

variable "kubernetes_version" {
  description = "Kubernetes version to install"
  type        = string
  default     = "1.28.5"
}

variable "ssh_user" {
  description = "SSH user for node access"
  type        = string
  default     = "root"
}

variable "ssh_private_key_path" {
  description = "Path to SSH private key"
  type        = string
  default     = "~/.ssh/id_rsa"
}
