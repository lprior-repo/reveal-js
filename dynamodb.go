package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DynamoDB record structure
type DynamoRecord struct {
	PK              string    `dynamodbav:"PK"`              // Organization#RepoName
	SK              string    `dynamodbav:"SK"`              // METADATA
	Organization    string    `dynamodbav:"organization"`
	Repository      string    `dynamodbav:"repository"`
	IsTerraform     bool      `dynamodbav:"is_terraform"`
	IsEvaporate     bool      `dynamodbav:"is_evaporate"`
	TotalResources  int       `dynamodbav:"total_resources"`
	TotalWorkspaces int       `dynamodbav:"total_workspaces"`
	Providers       []string  `dynamodbav:"providers"`
	LastAnalyzed    time.Time `dynamodbav:"last_analyzed"`
	TTL             int64     `dynamodbav:"ttl"`
}

func UpdateDynamoDB(ctx context.Context, config *Config, results []OrganizationResult) error {
	if !config.UseDynamoDB {
		return nil
	}

	fmt.Printf("📊 Updating DynamoDB table: %s\n", config.TableName)

	// Load AWS config
	cfg, err := loadAWSConfig(config.AWSRegion)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := dynamodb.NewFromConfig(cfg)

	// Ensure table exists
	if err := ensureTableExists(ctx, client, config.TableName); err != nil {
		return fmt.Errorf("failed to ensure table exists: %w", err)
	}

	// Update records
	for _, org := range results {
		for _, repo := range org.Repositories {
			record := DynamoRecord{
				PK:              fmt.Sprintf("%s#%s", org.Organization, repo.Repository.Name),
				SK:              "METADATA",
				Organization:    org.Organization,
				Repository:      repo.Repository.Name,
				IsTerraform:     repo.IsTerraform,
				IsEvaporate:     repo.IsEvaporate,
				TotalResources:  repo.TotalResources,
				TotalWorkspaces: len(repo.Workspaces),
				Providers:       repo.AllProviders,
				LastAnalyzed:    repo.LastAnalyzed,
				TTL:             time.Now().Add(30 * 24 * time.Hour).Unix(), // 30 days TTL
			}

			item, err := attributevalue.MarshalMap(record)
			if err != nil {
				log.Printf("Failed to marshal record for %s: %v", record.PK, err)
				continue
			}

			_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
				TableName: aws.String(config.TableName),
				Item:      item,
			})
			if err != nil {
				log.Printf("Failed to put item %s: %v", record.PK, err)
				continue
			}
		}
	}

	fmt.Printf("✅ DynamoDB update completed\n")
	return nil
}

func ensureTableExists(ctx context.Context, client *dynamodb.Client, tableName string) error {
	// Check if table exists
	_, err := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err == nil {
		return nil // Table exists
	}

	// Create table if it doesn't exist
	fmt.Printf("📋 Creating DynamoDB table: %s\n", tableName)
	
	_, err = client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("PK"),
				KeyType:       types.KeyTypeHash,
			},
			{
				AttributeName: aws.String("SK"),
				KeyType:       types.KeyTypeRange,
			},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("PK"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("SK"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("organization"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			{
				IndexName: aws.String("OrganizationIndex"),
				KeySchema: []types.KeySchemaElement{
					{
						AttributeName: aws.String("organization"),
						KeyType:       types.KeyTypeHash,
					},
					{
						AttributeName: aws.String("SK"),
						KeyType:       types.KeyTypeRange,
					},
				},
				Projection: &types.Projection{
					ProjectionType: types.ProjectionTypeAll,
				},
				ProvisionedThroughput: &types.ProvisionedThroughput{
					ReadCapacityUnits:  aws.Int64(5),
					WriteCapacityUnits: aws.Int64(5),
				},
			},
		},
		ProvisionedThroughput: &types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		},
	})
	
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Wait for table to become active
	waiter := dynamodb.NewTableExistsWaiter(client)
	return waiter.Wait(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	}, 2*time.Minute)
}

func loadAWSConfig(region string) (aws.Config, error) {
	return config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
	)
}