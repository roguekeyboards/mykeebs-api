// Command moderate is the moderation function: S3 invokes it for each
// image uploaded to the images bucket. See internal/moderation.
package main

import (
	"context"
	"log"
	"log/slog"
	"os"

	"github.com/alecthomas/kong"
	"github.com/aws/aws-lambda-go/lambda"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/roguekeyboards/mykeebs-api/internal/moderation"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/dynamo"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	var cfg Config
	kong.Parse(&cfg)

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("loading AWS config: %v", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	images := &moderation.Images{
		Keyboards:  dynamo.NewKeyboardRepository(dynamoClient, cfg.KeyboardTableName),
		Builds:     dynamo.NewBuildRepository(dynamoClient, cfg.BuildTableName),
		Switches:   dynamo.NewSwitchRepository(dynamoClient, cfg.SwitchTableName),
		ECParts:    dynamo.NewECPartRepository(dynamoClient, cfg.ECPartTableName),
		KeycapSets: dynamo.NewKeycapSetRepository(dynamoClient, cfg.KeycapSetTableName),
		Profiles:   dynamo.NewProfileRepository(dynamoClient, cfg.ProfileTableName, cfg.ProfileUsernameTableName),
	}
	m := moderation.New(s3.NewFromConfig(awsCfg), rekognition.NewFromConfig(awsCfg), images, cfg.Calibration)

	lambda.Start(m.Handle)
}
