package main

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	awscloudfront "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/roguekeyboards/mykeebs-api/internal/moderation"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/cloudfront"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/dynamo"
)

// stack is one environment's resources, looked up by their logical IDs in
// template.yaml.
type stack struct {
	cfg      aws.Config
	bucket   string
	logGroup string
	function string
	dlqURL   string
	cdn      *cloudfront.Invalidator
	images   *moderation.Images

	// expiryDays is the stack's RejectedImageExpirationDays.
	expiryDays int
}

func resolveStack(ctx context.Context, cfg aws.Config, name string) (*stack, error) {
	cfn := cloudformation.NewFromConfig(cfg)
	expiryDays, err := expiryDays(ctx, cfn, name)
	if err != nil {
		return nil, err
	}

	ids := map[string]string{}
	pages := cloudformation.NewListStackResourcesPaginator(cfn,
		&cloudformation.ListStackResourcesInput{StackName: &name})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing the resources of stack %s: %w", name, err)
		}
		for _, r := range page.StackResourceSummaries {
			ids[aws.ToString(r.LogicalResourceId)] = aws.ToString(r.PhysicalResourceId)
		}
	}
	for _, logical := range []string{
		"ImagesBucket", "ModerateFunctionLogGroup", "SwitchTable", "ECPartTable", "KeyboardTable",
		"KeycapSetTable", "BuildTable", "ProfileTable", "ProfileUsernameTable",
		"ModerateFunction", "ModerationDLQ", "ImagesDistribution",
	} {
		if ids[logical] == "" {
			return nil, fmt.Errorf("stack %s has no %s", name, logical)
		}
	}

	db := dynamodb.NewFromConfig(cfg)
	return &stack{
		cfg:        cfg,
		expiryDays: expiryDays,
		bucket:     ids["ImagesBucket"],
		logGroup:   ids["ModerateFunctionLogGroup"],
		function:   ids["ModerateFunction"],
		dlqURL:     ids["ModerationDLQ"],
		cdn:        cloudfront.NewInvalidator(awscloudfront.NewFromConfig(cfg), ids["ImagesDistribution"]),
		images: &moderation.Images{
			Keyboards:  dynamo.NewKeyboardRepository(db, ids["KeyboardTable"]),
			Builds:     dynamo.NewBuildRepository(db, ids["BuildTable"]),
			Switches:   dynamo.NewSwitchRepository(db, ids["SwitchTable"]),
			ECParts:    dynamo.NewECPartRepository(db, ids["ECPartTable"]),
			KeycapSets: dynamo.NewKeycapSetRepository(db, ids["KeycapSetTable"]),
			Profiles:   dynamo.NewProfileRepository(db, ids["ProfileTable"], ids["ProfileUsernameTable"]),
		},
	}, nil
}

// expiryDays reads the stack's RejectedImageExpirationDays parameter.
func expiryDays(ctx context.Context, cfn *cloudformation.Client, name string) (int, error) {
	out, err := cfn.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{StackName: &name})
	if err != nil {
		return 0, fmt.Errorf("describing stack %s: %w", name, err)
	}
	for _, p := range out.Stacks[0].Parameters {
		if aws.ToString(p.ParameterKey) == "RejectedImageExpirationDays" {
			days, err := strconv.Atoi(aws.ToString(p.ParameterValue))
			if err != nil {
				return 0, fmt.Errorf("stack %s's RejectedImageExpirationDays: %w", name, err)
			}
			return days, nil
		}
	}
	return 0, fmt.Errorf("stack %s has no RejectedImageExpirationDays parameter", name)
}

func (st *stack) dlq() *moderation.DLQ {
	return moderation.NewDLQ(sqs.NewFromConfig(st.cfg), lambda.NewFromConfig(st.cfg), st.dlqURL, st.function)
}

func (st *stack) overrides() *moderation.Overrides {
	reviews := moderation.NewReviewLog(cloudwatchlogs.NewFromConfig(st.cfg), st.logGroup)
	return moderation.NewOverrides(s3.NewFromConfig(st.cfg), st.images, reviews, st.bucket, st.cdn)
}
