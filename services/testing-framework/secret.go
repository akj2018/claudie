package testingframework

import (
	"encoding/base64"
	"fmt"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/services/testing-framework/templates"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type SecretData struct {
	SecretName string
	Namespace  string
	FieldName  string
	Manifest   string
}

// deleteSecret will delete a secret in the cluster in the specified namespace
func deleteSecret(setName string) error {
	kc := kubectl.Kubectl{MaxKubectlRetries: 3}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		kc.Stdout = comm.GetStdOut(setName)
		kc.Stderr = comm.GetStdErr(setName)
	}
	return kc.KubectlDeleteResource("secret", setName, "-n", envs.Namespace)
}

// applySecret function will create a secret with the specified name in the specified namespace for manifest provided
func applySecret(manifest []byte, pathToTestSet, secretName string) error {
	template := templateUtils.Templates{Directory: pathToTestSet}
	tpl, err := templateUtils.LoadTemplate(templates.SecretTemplate)
	if err != nil {
		return fmt.Errorf("error while loading secret.goyaml : %w", err)
	}
	d := &SecretData{
		SecretName: secretName,
		Namespace:  envs.Namespace,
		FieldName:  secretName,
		Manifest:   base64.StdEncoding.EncodeToString(manifest),
	}
	secret, err := template.GenerateToString(tpl, d)
	if err != nil {
		return fmt.Errorf("error while generating string for secret %s : %w", secretName, err)
	}
	kc := kubectl.Kubectl{MaxKubectlRetries: 3}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		kc.Stdout = comm.GetStdOut(pathToTestSet)
		kc.Stderr = comm.GetStdErr(pathToTestSet)
	}
	return kc.KubectlApplyString(secret, "-n", envs.Namespace)
}

// getManifestName will read the name of the manifest from the file and return it,
// so it can be used as an id to retrieve it from database in configChecker()
func getManifestName(yamlFile []byte) (string, error) {
	var manifest manifest.Manifest
	err := yaml.Unmarshal(yamlFile, &manifest)
	if err != nil {
		return "", fmt.Errorf("error while unmarshalling a manifest file: %w", err)
	}

	if manifest.Name != "" {
		return manifest.Name, nil
	}
	return "", fmt.Errorf("manifest does not have a name defined, which could be used as DB id")
}
