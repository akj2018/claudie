{{- $clusterName := .ClusterName}}
{{- $clusterHash := .ClusterHash}}
{{- $index :=  0}}

variable "default_compartment_id" {
  type    = string
  default = "{{ (index .NodePools 0).NodePool.Provider.OciCompartmentOcid }}"
}

variable "oci_storage_disk_name" {
  default = "oraclevdb"
  type    = string
}

{{- range $i, $region := .Regions }}
resource "oci_core_vcn" "claudie_vcn-{{ $region }}" {
  provider        = oci.k8s_nodepool_{{ $region }}
  compartment_id  = var.default_compartment_id
  display_name    = "{{ $clusterName }}-{{ $clusterHash }}-vcn"
  cidr_blocks     = ["10.0.0.0/16"]

  freeform_tags = {
    "Managed-by"      = "Claudie"
    "Claudie-cluster" = "{{ $clusterName }}-{{ $clusterHash }}"
  } 
}

resource "oci_core_internet_gateway" "claudie_gateway_{{ $region }}" {
  provider        = oci.k8s_nodepool_{{ $region }}
  compartment_id  = var.default_compartment_id
  display_name    = "{{ $clusterName }}-{{ $clusterHash }}-gateway"
  vcn_id          = oci_core_vcn.claudie_vcn-{{ $region }}.id
  enabled         = true

  freeform_tags = {
    "Managed-by"      = "Claudie"
    "Claudie-cluster" = "{{ $clusterName }}-{{ $clusterHash }}"
  } 
}  

resource "oci_core_default_security_list" "claudie_security_rules_{{ $region }}" {
  provider                    = oci.k8s_nodepool_{{ $region }}
  manage_default_resource_id  = oci_core_vcn.claudie_vcn-{{ $region }}.default_security_list_id
  display_name                = "{{ $clusterName }}-{{ $clusterHash }}_security_rules"

  egress_security_rules {  
    destination = "0.0.0.0/0"
    protocol    = "all"
    description = "Allow all egress"
  }

  ingress_security_rules {
    protocol    = "1"
    source      = "0.0.0.0/0"
    description = "Allow all ICMP"
  }

  ingress_security_rules {
    protocol    = "6"
    source      = "0.0.0.0/0"
    tcp_options {
      min = "22"
      max = "22"
    }
    description = "Allow SSH connections"
  }

  {{- if index $.Metadata "loadBalancers" | targetPorts | isMissing 6443 }}
  ingress_security_rules {
    protocol    = "6"
    source      = "0.0.0.0/0"
    tcp_options {
      max = "6443"
      min = "6443"
    }
    description = "Allow kube API port"
  }
  {{- end }}

  ingress_security_rules {
    protocol    = "17"
    source      = "0.0.0.0/0"
    udp_options {
      max = "51820"
      min = "51820"
    }
    description = "Allow Wireguard VPN port"
  }

  freeform_tags = {
    "Managed-by"      = "Claudie"
    "Claudie-cluster" = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "oci_core_default_route_table" "claudie_routes_{{ $region }}" {
  provider                    = oci.k8s_nodepool_{{ $region }}
  manage_default_resource_id  = oci_core_vcn.claudie_vcn-{{ $region }}.default_route_table_id

  route_rules {
    destination       = "0.0.0.0/0"
    network_entity_id = oci_core_internet_gateway.claudie_gateway_{{ $region }}.id
    destination_type  = "CIDR_BLOCK"
  }

  freeform_tags = {
    "Managed-by"      = "Claudie"
    "Claudie-cluster" = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}
{{- end }}

{{- range $i, $nodepool := .NodePools }}
resource "oci_core_subnet" "{{ $nodepool.Name }}_subnet" {
  provider            = oci.k8s_nodepool_{{ $nodepool.NodePool.Region }}
  vcn_id              = oci_core_vcn.claudie_vcn-{{ $nodepool.NodePool.Region }}.id
  cidr_block          = "{{ index $.Metadata (printf "%s-subnet-cidr" $nodepool.Name)  }}"
  compartment_id      = var.default_compartment_id
  display_name        = "{{ $clusterName }}-{{ $clusterHash }}-subnet"
  security_list_ids   = [oci_core_vcn.claudie_vcn-{{ $nodepool.NodePool.Region }}.default_security_list_id]
  route_table_id      = oci_core_vcn.claudie_vcn-{{ $nodepool.NodePool.Region }}.default_route_table_id
  dhcp_options_id     = oci_core_vcn.claudie_vcn-{{ $nodepool.NodePool.Region }}.default_dhcp_options_id
  availability_domain = "{{ $nodepool.NodePool.Zone }}"

  freeform_tags = {
    "Managed-by"      = "Claudie"
    "Claudie-cluster" = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

{{- range $node := $nodepool.Nodes }}
resource "oci_core_instance" "{{ $node.Name }}" {
  provider            = oci.k8s_nodepool_{{ $nodepool.NodePool.Region }}
  compartment_id      = var.default_compartment_id
  availability_domain = "{{ $nodepool.NodePool.Zone }}"
  shape               = "{{ $nodepool.NodePool.ServerType }}"
  display_name        = "{{ $node.Name }}"

  metadata = {
      ssh_authorized_keys = file("./public.pem")
      user_data = base64encode(<<EOF
      #cloud-config
      runcmd:
        # Allow Claudie to ssh as root
        - sed -n 's/^.*ssh-rsa/ssh-rsa/p' /root/.ssh/authorized_keys > /root/.ssh/temp
        - cat /root/.ssh/temp > /root/.ssh/authorized_keys
        - rm /root/.ssh/temp
        - echo 'PermitRootLogin without-password' >> /etc/ssh/sshd_config && echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config && echo "PubkeyAcceptedKeyTypes=+ssh-rsa" >> sshd_config && service sshd restart
        # Disable iptables
        # Accept all traffic to avoid ssh lockdown via iptables firewall rules
        - iptables -P INPUT ACCEPT
        - iptables -P FORWARD ACCEPT
        - iptables -P OUTPUT ACCEPT
        # Flush and cleanup
        - iptables -F
        - iptables -X
        - iptables -Z
        # Make changes persistent
        - netfilter-persistent save
        {{- if not $nodepool.IsControl }}
        # Mount volume
        - |
          sleep 50
          disk=$(ls -l /dev/oracleoci | grep "${var.oci_storage_disk_name}" | awk '{print $NF}')
          disk=$(basename "$disk")
          if ! grep -qs "/dev/$disk" /proc/mounts; then
            mkdir -p /opt/claudie/data
            if ! blkid /dev/$disk | grep -q "TYPE=\"xfs\""; then
              mkfs.xfs /dev/$disk
            fi
            mount /dev/$disk /opt/claudie/data
            echo "/dev/$disk /opt/claudie/data xfs defaults 0 0" >> /etc/fstab
          fi
        {{- end }}
      EOF
      )
  }
  
  source_details {
    source_id               = "{{ $nodepool.NodePool.Image }}"
    source_type             = "image"
    boot_volume_size_in_gbs = "100"
  }

  create_vnic_details {
    assign_public_ip  = true
    subnet_id         = oci_core_subnet.{{ $nodepool.Name }}_subnet.id
  }

  freeform_tags = {
    "Managed-by"      = "Claudie"
    "Claudie-cluster" = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

{{- if not $nodepool.IsControl }}
resource "oci_core_volume" "{{ $node.Name }}_volume" {
  provider            = oci.k8s_nodepool_{{ $nodepool.NodePool.Region }}
  compartment_id      = var.default_compartment_id
  availability_domain = "{{ $nodepool.NodePool.Zone }}"
  size_in_gbs         = "{{ $nodepool.NodePool.StorageDiskSize }}"
  display_name        = "{{ $node.Name }}-volume"
  vpus_per_gb         = 10

  freeform_tags = {
    "Managed-by"      = "Claudie"
    "Claudie-cluster" = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "oci_core_volume_attachment" "{{ $node.Name }}_volume_att" {
  provider        = oci.k8s_nodepool_{{ $nodepool.NodePool.Region }}
  attachment_type = "paravirtualized"
  instance_id     = oci_core_instance.{{ $node.Name }}.id
  volume_id       = oci_core_volume.{{ $node.Name }}_volume.id
  display_name    = "{{ $node.Name }}-volume-att"
  device          = "/dev/oracleoci/${var.oci_storage_disk_name}"
}
{{- end }}
{{- end }}

output "{{ $nodepool.Name }}" {
  value = {
  {{- range $node := $nodepool.Nodes }}
    "${oci_core_instance.{{ $node.Name }}.display_name}" = oci_core_instance.{{ $node.Name }}.public_ip
  {{- end }}
  }
}
{{- end }}
