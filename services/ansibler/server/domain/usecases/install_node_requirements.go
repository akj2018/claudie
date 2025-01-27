package usecases

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"

	commonUtils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/ansibler/server/utils"
	"github.com/berops/claudie/services/ansibler/templates"
)

const ansiblePlaybookFilePath = "../../ansible-playbooks/longhorn-req.yml"

// InstallNodeRequirements installs pre-requisite tools (currently only for LongHorn) on all the nodes
func (u *Usecases) InstallNodeRequirements(request *pb.InstallRequest) (*pb.InstallResponse, error) {
	logger := log.With().Str("project", request.ProjectName).Str("cluster", request.Desired.ClusterInfo.Name).Logger()
	logger.Info().Msgf("Installing node requirements")

	NodepoolsInfo := &NodepoolsInfo{
		Nodepools: utils.NodePools{
			Dynamic: commonUtils.GetCommonDynamicNodePools(request.Desired.ClusterInfo.NodePools),
			Static:  commonUtils.GetCommonStaticNodePools(request.Desired.ClusterInfo.NodePools),
		},
		PrivateKey:     request.Desired.ClusterInfo.PrivateKey,
		ClusterID:      fmt.Sprintf("%s-%s", request.Desired.ClusterInfo.Name, request.Desired.ClusterInfo.Hash),
		ClusterNetwork: request.Desired.Network,
	}

	if err := installLonghornRequirements(NodepoolsInfo); err != nil {
		logger.Err(err).Msgf("Error encountered while installing node requirements")
		return nil, fmt.Errorf("error encountered while installing node requirements for cluster %s project %s : %w", request.Desired.ClusterInfo.Name, request.ProjectName, err)
	}

	logger.Info().Msgf("Node requirements were successfully installed")
	return &pb.InstallResponse{Desired: request.Desired, DesiredLbs: request.DesiredLbs}, nil
}

// installLonghornRequirements installs pre-requisite tools for LongHorn in all the nodes
func installLonghornRequirements(nodepoolsInfo *NodepoolsInfo) error {
	// Directory where files (required by Ansible) will be generated.
	clusterDirectory := filepath.Join(baseDirectory, outputDirectory, commonUtils.CreateHash(commonUtils.HashLength))
	if err := commonUtils.CreateDirectory(clusterDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", clusterDirectory, err)
	}

	// generate private SSH key which will be used by Ansible
	if err := commonUtils.CreateKeyFile(nodepoolsInfo.PrivateKey, clusterDirectory, fmt.Sprintf("%s.%s", nodepoolsInfo.ClusterID, sshPrivateKeyFileExtension)); err != nil {
		return fmt.Errorf("failed to create key file for %s : %w", nodepoolsInfo.ClusterID, err)
	}

	if err := utils.GenerateInventoryFile(templates.AllNodesInventoryTemplate, clusterDirectory,
		// Value of Ansible template parameters
		AllNodesInventoryData{
			NodepoolsInfo: []*NodepoolsInfo{nodepoolsInfo},
		},
	); err != nil {
		return fmt.Errorf("failed to generate inventory file for all nodes in %s : %w", clusterDirectory, err)
	}

	ansible := utils.Ansible{
		Playbook:  ansiblePlaybookFilePath,
		Inventory: utils.InventoryFileName,
		Directory: clusterDirectory,
	}
	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("Node requirements - %s", nodepoolsInfo.ClusterID)); err != nil {
		return fmt.Errorf("error while running ansible playbook at %s to install Longhorn requirements : %w", clusterDirectory, err)
	}

	return os.RemoveAll(clusterDirectory)
}
