{{- $clusterName := .ClusterName }}
{{- $clusterHash := .ClusterHash }}
{{- range $i, $region := .Regions }}
resource "aws_vpc" "claudie_vpc_{{ $region }}" {
  provider   = aws.k8s_nodepool_{{ $region }}
  cidr_block = "10.0.0.0/16"

  tags = {
    Name            = "{{ $clusterName }}-{{ $clusterHash }}-{{ $region }}-vpc"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "aws_internet_gateway" "claudie_gateway_{{ $region }}" {
  provider = aws.k8s_nodepool_{{ $region }}
  vpc_id   = aws_vpc.claudie_vpc_{{ $region }}.id

  tags = {
    Name            = "{{ $clusterName }}-{{ $clusterHash }}-{{ $region }}-gateway"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "aws_route_table" "claudie_route_table_{{ $region }}" {
  provider     = aws.k8s_nodepool_{{ $region }}
  vpc_id       = aws_vpc.claudie_vpc_{{ $region }}.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.claudie_gateway_{{ $region }}.id
  }

  tags = {
    Name            = "{{ $clusterName }}-{{ $clusterHash }}-{{ $region }}-rt"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "aws_security_group" "claudie_sg_{{ $region }}" {
  provider               = aws.k8s_nodepool_{{ $region }}
  vpc_id                 = aws_vpc.claudie_vpc_{{ $region }}.id
  revoke_rules_on_delete = true

  tags = {
    Name            = "{{ $clusterName }}-{{ $clusterHash }}-{{ $region }}-sg"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "aws_security_group_rule" "allow_egress_{{ $region }}" {
  provider          = aws.k8s_nodepool_{{ $region }}
  type              = "egress"
  from_port         = 0
  to_port           = 65535
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie_sg_{{ $region }}.id
}


resource "aws_security_group_rule" "allow_ssh_{{ $region }}" {
  provider          = aws.k8s_nodepool_{{ $region }}
  type              = "ingress"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie_sg_{{ $region }}.id
}
{{- if index $.Metadata "loadBalancers" | targetPorts | isMissing 6443 }}
resource "aws_security_group_rule" "allow_kube_api_{{ $region }}" {
  provider          = aws.k8s_nodepool_{{ $region }}
  type              = "ingress"
  from_port         = 6443
  to_port           = 6443
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie_sg_{{ $region }}.id
}
{{- end}}

resource "aws_security_group_rule" "allow_wireguard_{{ $region }}" {
  provider          = aws.k8s_nodepool_{{ $region }}
  type              = "ingress"
  from_port         = 51820
  to_port           = 51820
  protocol          = "udp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie_sg_{{ $region }}.id
}

resource "aws_security_group_rule" "allow_icmp_{{ $region }}" {
  provider          = aws.k8s_nodepool_{{ $region }}
  type              = "ingress"
  from_port         = 8
  to_port           = 0
  protocol          = "icmp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie_sg_{{ $region }}.id
}

resource "aws_key_pair" "claudie_pair_{{ $region }}" {
  provider   = aws.k8s_nodepool_{{ $region }}
  key_name   = "{{ $clusterName }}-{{ $clusterHash }}-key"
  public_key = file("./public.pem")
  tags = {
    Name            = "{{ $clusterName }}-{{ $clusterHash }}-{{ $region }}-key"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}
{{- end }}

{{- range $i, $nodepool := .NodePools }}
resource "aws_subnet" "{{ $nodepool.Name }}_subnet" {
  provider                = aws.k8s_nodepool_{{ $nodepool.NodePool.Region  }}
  vpc_id                  = aws_vpc.claudie_vpc_{{ $nodepool.NodePool.Region }}.id
  cidr_block              = "{{ index $.Metadata (printf "%s-subnet-cidr" $nodepool.Name) }}"
  map_public_ip_on_launch = true
  availability_zone       = "{{ $nodepool.NodePool.Zone }}"

  tags = {
    Name            = "{{ $nodepool.Name }}-{{ $clusterHash }}-subnet"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "aws_route_table_association" "{{ $nodepool.Name }}_rta" {
  provider       = aws.k8s_nodepool_{{ $nodepool.NodePool.Region  }}
  subnet_id      = aws_subnet.{{ $nodepool.Name }}_subnet.id
  route_table_id = aws_route_table.claudie_route_table_{{ $nodepool.NodePool.Region }}.id
}

{{- range $node := $nodepool.Nodes }}
resource "aws_instance" "{{ $node.Name }}" {
  provider          = aws.k8s_nodepool_{{ $nodepool.NodePool.Region }}
  availability_zone = "{{ $nodepool.NodePool.Zone }}"
  instance_type     = "{{ $nodepool.NodePool.ServerType }}"
  ami               = "{{ $nodepool.NodePool.Image }}"
  
  associate_public_ip_address = true
  key_name               = aws_key_pair.claudie_pair_{{ $nodepool.NodePool.Region }}.key_name
  subnet_id              = aws_subnet.{{ $nodepool.Name }}_subnet.id
  vpc_security_group_ids = [aws_security_group.claudie_sg_{{ $nodepool.NodePool.Region }}.id]

  tags = {
    Name            = "{{ $node.Name }}"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
  
  root_block_device {
    volume_size           = 100
    delete_on_termination = true
    volume_type           = "gp2"
  }
 
  user_data = <<EOF
#!/bin/bash
set -euxo pipefail
# Allow ssh connection for root
sed -n 's/^.*ssh-rsa/ssh-rsa/p' /root/.ssh/authorized_keys > /root/.ssh/temp
cat /root/.ssh/temp > /root/.ssh/authorized_keys
rm /root/.ssh/temp
echo 'PermitRootLogin without-password' >> /etc/ssh/sshd_config && echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config && echo "PubkeyAcceptedKeyTypes=+ssh-rsa" >> sshd_config && service sshd restart
{{- if not $nodepool.IsControl }}
# Mount EBS volume only when not mounted yet
sleep 50
disk=$(ls -l /dev/disk/by-id | grep "${replace("${aws_ebs_volume.{{ $node.Name }}_volume.id}", "-", "")}" | awk '{print $NF}')
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
}

{{- if not $nodepool.IsControl }}
resource "aws_ebs_volume" "{{ $node.Name }}_volume" {
  provider          = aws.k8s_nodepool_{{ $nodepool.NodePool.Region }}
  availability_zone = "{{ $nodepool.NodePool.Zone }}"
  size              = {{ $nodepool.NodePool.StorageDiskSize }}
  type              = "gp2"

  tags = {
    Name            = "{{ $node.Name }}-storage"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "aws_volume_attachment" "{{ $node.Name }}_volume_att" {
  provider    = aws.k8s_nodepool_{{ $nodepool.NodePool.Region }}
  device_name = "/dev/sdh"
  volume_id   = aws_ebs_volume.{{ $node.Name }}_volume.id
  instance_id = aws_instance.{{ $node.Name }}.id
}
{{- end }}
{{- end }}

output  "{{ $nodepool.Name }}" {
  value = {
    {{- range $j, $node := $nodepool.Nodes }}
    {{- $name := (printf "%s-%s-%s-%d" $clusterName $clusterHash $nodepool.Name $j ) }}
    "${aws_instance.{{ $node.Name }}.tags_all.Name}" =  aws_instance.{{ $node.Name }}.public_ip
    {{- end }}
  }
}
{{- end }}