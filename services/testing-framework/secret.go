package testingframework

import (
	"encoding/base64"
	"fmt"

	"github.com/Berops/platform/services/kuber/server/kubectl"
	"github.com/Berops/platform/services/scheduler/manifest"
	"github.com/Berops/platform/utils"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

const (
	secretTpl  = "secret.goyaml"
	secretFile = "secret.yaml"
)

type SecretData struct {
	SecretName string
	Namespace  string
	FieldName  string
	Manifest   string
}

// deleteSecret will delete a secret in the cluster in the specified namespace
func deleteSecret(setName, namespace string) error {
	kc := kubectl.Kubectl{}
	return kc.KubectlDeleteResource("secret", setName, namespace)
}

// manageSecret function will create a secret.yaml file in test set directory, with a specified manifest in data encoded as base64 string
func manageSecret(manifest []byte, pathToTestSet, secretName, namespace string) error {
	templateLoader := utils.TemplateLoader{Directory: utils.TestingTemplates}
	template := utils.Templates{Directory: pathToTestSet}
	tpl, err := templateLoader.LoadTemplate(secretTpl)
	if err != nil {
		return err
	}
	d := &SecretData{
		SecretName: secretName,
		Namespace:  namespace,
		FieldName:  secretName,
		Manifest:   base64.StdEncoding.EncodeToString(manifest),
	}
	err = template.Generate(tpl, secretFile, d)
	if err != nil {
		return err
	}
	kc := kubectl.Kubectl{Directory: pathToTestSet}
	return kc.KubectlApply(secretFile, "")
}

// getManifestName will read the name of the manifest from the file and return it,
// so it can be used as an id to retrieve it from database in configChecker()
func getManifestName(yamlFile []byte) (string, error) {
	var manifest manifest.Manifest
	err := yaml.Unmarshal(yamlFile, &manifest)
	if err != nil {
		log.Error().Msgf("Error while unmarshalling a manifest file: %v", err)
		return "", err
	}

	if manifest.Name != "" {
		return manifest.Name, nil
	}
	return "", fmt.Errorf("manifest does not have a name defined, which could be used as DB id")
}