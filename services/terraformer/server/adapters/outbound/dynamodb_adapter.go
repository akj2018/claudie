package outboundAdapters

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/internal/envs"
)

var (
	awsRegion          = envs.AwsRegion
	awsAccessKeyId     = envs.AwsAccesskeyId
	awsSecretAccessKey = envs.AwsSecretAccessKey

	dynamoURL = envs.DynamoURL
	// This DynamoDB table is used for Terraform state locking
	dynamoDBTableName = envs.DynamoTable
)

type DynamoDBAdapter struct {
	Client           *dynamodb.Client
	healtcheckClient *dynamodb.Client
}

// createDynamoDBClient creates a DynamoDB client.
func createDynamoDBClient() *dynamodb.Client {
	return dynamodb.NewFromConfig(
		aws.Config{
			Region: awsRegion,
			Credentials: aws.CredentialsProviderFunc(
				func(ctx context.Context) (aws.Credentials, error) {
					return aws.Credentials{AccessKeyID: awsAccessKeyId, SecretAccessKey: awsSecretAccessKey}, nil
				},
			),

			EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(
				func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{URL: dynamoURL}, nil
				},
			),

			RetryMaxAttempts: 10,
			RetryMode:        aws.RetryModeStandard,
		},
	)
}

// CreateDynamoDBAdapter creates 2 separate dynamoDB clients - one for healthcheck and another for general usage.
// These 2 dynamoDB clients are then used to construct a DynamoDBAdapter instance.
// Returns the DynamoDBAdapter instance.
func CreateDynamoDBAdapter() *DynamoDBAdapter {
	dynamoDBAdapter := &DynamoDBAdapter{
		Client:           createDynamoDBClient(),
		healtcheckClient: createDynamoDBClient(),
	}

	return dynamoDBAdapter
}

// Healthcheck function checks whether
// the DynamoDB table for Terraform state locking exists or not.
func (d *DynamoDBAdapter) Healthcheck() error {
	tables, err := d.healtcheckClient.ListTables(context.Background(), nil)
	if err != nil {
		return err
	}

	for _, table := range tables.TableNames {
		if table == dynamoDBTableName {
			return nil
		}
	}

	return fmt.Errorf("dynamoDB does not contain %s table", dynamoDBTableName)
}

// DeleteLockFile deletes terraform state lock file (related to the given cluster), from DynamoDB.
func (d *DynamoDBAdapter) DeleteLockFile(ctx context.Context, projectName, clusterId string, keyFormat string) error {
	// Get the DynamoDB key (keyname is LockID) which maps to the Terraform state-lock file
	key, err := attributevalue.Marshal(fmt.Sprintf(keyFormat, minioBucketName, projectName, clusterId))
	if err != nil {
		return fmt.Errorf("error composing DynamoDB key for the Terraform state-lock file for cluster %s: %w", clusterId, err)
	}

	log.Debug().Msgf("Deleting Terraform state-lock file with DynamoDB key: %v", key)

	// Delete the Terraform state-lock file from DynamoDB
	if _, err := d.Client.DeleteItem(ctx,
		&dynamodb.DeleteItemInput{
			TableName: aws.String(dynamoDBTableName),
			Key:       map[string]types.AttributeValue{"LockID": key},
		},
	); err != nil {
		return fmt.Errorf("failed to remove Terraform state-lock file %v : %w", clusterId, err)
	}

	return nil
}
