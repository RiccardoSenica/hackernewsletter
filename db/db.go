package db

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Table struct {
	DynamoDbClient *dynamodb.Client
	TableName      string
}

type News struct {
	Id          int
	Title       string
	Text        string
	Time        time.Time
	By          string
	Url         string
	Score       int
	Parent      int
	Descendants string
	Category    string
	Delete      bool
	Dead        bool
}

type DynamoDBDescribeTableAPI interface {
	DescribeTable(ctx context.Context,
		params *dynamodb.DescribeTableInput,
		optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
}

func CreateTable(basics Table) (*types.TableDescription, error) {
	var tableDesc *types.TableDescription
	table, err := basics.DynamoDbClient.CreateTable(context.TODO(), &dynamodb.CreateTableInput{
		KeySchema: []types.KeySchemaElement{{
			AttributeName: aws.String("Id"),
			KeyType:       types.KeyTypeHash,
		}, {
			AttributeName: aws.String("Title"),
			KeyType:       types.KeyTypeRange,
		}},
		AttributeDefinitions: []types.AttributeDefinition{{
			AttributeName: aws.String("Id"),
			AttributeType: types.ScalarAttributeTypeN,
		}, {
			AttributeName: aws.String("Title"),
			AttributeType: types.ScalarAttributeTypeS,
		}},
		TableName: aws.String(basics.TableName),
		ProvisionedThroughput: &types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(10),
			WriteCapacityUnits: aws.Int64(10),
		},
	})
	if err != nil {
		log.Printf("Couldn't create table %v. Here's why: %v\n", basics.TableName, err)
	} else {
		waiter := dynamodb.NewTableExistsWaiter(basics.DynamoDbClient)
		err = waiter.Wait(context.TODO(), &dynamodb.DescribeTableInput{
			TableName: aws.String(basics.TableName)}, 5*time.Minute)
		if err != nil {
			log.Printf("Wait for table exists failed. Here's why: %v\n", err)
		}
		tableDesc = table.TableDescription
	}
	return tableDesc, err
}

func GetTableInfo(c context.Context, api DynamoDBDescribeTableAPI, input *dynamodb.DescribeTableInput) (*dynamodb.DescribeTableOutput, error) {
	return api.DescribeTable(c, input)
}

func AddNewsBatch(basics Table, news []News) (int, error) {
	var err error
	var item map[string]types.AttributeValue
	written := 0
	batchSize := 25
	start := 0
	end := start + batchSize
	for start < len(news) {
		var writeReqs []types.WriteRequest
		if end > len(news) {
			end = len(news)
		}
		for _, entry := range news[start:end] {
			item, err = attributevalue.MarshalMap(entry)
			if err != nil {
				log.Printf("Couldn't marshal news %v for batch writing: %v\n", entry.Title, err)
			} else {
				writeReqs = append(writeReqs, types.WriteRequest{PutRequest: &types.PutRequest{Item: item}})
			}
		}
		_, err = basics.DynamoDbClient.BatchWriteItem(context.TODO(), &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{basics.TableName: writeReqs}})
		if err != nil {
			log.Printf("Couldn't add a batch of news to %v: %v\n", basics.TableName, err)
		} else {
			written += len(writeReqs)
		}
		start = end
		end += batchSize
	}

	return written, err
}