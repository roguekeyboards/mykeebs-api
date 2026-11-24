package dynamo

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	apictx "github.com/roguekeyboards/mykeebs-api/internal/ctx"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// ECPartRepository is the DynamoDB-backed repository.ECPartRepository.
type ECPartRepository struct {
	client    dynamoAPI
	tableName string
}

var _ repository.ECPartRepository = (*ECPartRepository)(nil)

// NewECPartRepository returns an ECPartRepository backed by client.
func NewECPartRepository(client *dynamodb.Client, tableName string) *ECPartRepository {
	return &ECPartRepository{client: client, tableName: tableName}
}

// List implements repository.ECPartRepository.
func (r *ECPartRepository) List(
	ctx context.Context,
	ownerID string,
	visibilities []repository.Visibility,
	partType repository.ECPartType,
	limit int,
	cursor string,
) ([]repository.ECPart, string, error) {
	// No visibility tier is readable, so nothing can match - short-circuit
	// rather than build a Query with an empty IN(...) filter (which the
	// expression builder below would panic constructing).
	if len(visibilities) == 0 {
		return []repository.ECPart{}, "", nil
	}

	startKey, cursorPartType, err := decodeOwnerCursor(cursor, ownerID)
	if err != nil {
		return nil, "", err
	}
	if len(startKey) > 0 && cursorPartType != string(partType) {
		return nil, "", fmt.Errorf("%w: part type filter changed since this cursor was issued", repository.ErrInvalidCursor)
	}

	visValues := make([]expression.OperandBuilder, len(visibilities))
	for i, v := range visibilities {
		visValues[i] = expression.Value(v)
	}

	filter := expression.Name("visibility").In(visValues[0], visValues[1:]...)
	if partType != "" {
		filter = filter.And(expression.Name("part_type").Equal(expression.Value(partType)))
	}

	builder := expression.NewBuilder().
		WithKeyCondition(expression.Key("user_id").Equal(expression.Value(ownerID))).
		WithFilter(filter)

	expr, err := builder.Build()
	if err != nil {
		return nil, "", fmt.Errorf("building EC part list expression: %w", err)
	}

	items, lastKey, err := queryFullPage(ctx, r.client, &dynamodb.QueryInput{
		TableName:                 &r.tableName,
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ExclusiveStartKey:         startKey,
	}, limit)
	if err != nil {
		return nil, "", fmt.Errorf("querying EC parts for owner %q: %w", ownerID, err)
	}

	parts := []repository.ECPart{}
	if err := attributevalue.UnmarshalListOfMaps(items, &parts); err != nil {
		return nil, "", fmt.Errorf("unmarshalling EC parts for owner %q: %w", ownerID, err)
	}

	nextCursor, err := encodeCursor(lastKey, "", string(partType))
	if err != nil {
		return nil, "", fmt.Errorf("encoding next cursor: %w", err)
	}

	return parts, nextCursor, nil
}

// Get implements repository.ECPartRepository.
func (r *ECPartRepository) Get(ctx context.Context, ownerID, id string) (*repository.ECPart, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &r.tableName,
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: ownerID},
			"id":      &types.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("getting EC part %q for owner %q: %w", id, ownerID, err)
	}

	if len(out.Item) == 0 {
		return nil, repository.ErrNotFound
	}

	var p repository.ECPart
	if err := attributevalue.UnmarshalMap(out.Item, &p); err != nil {
		return nil, fmt.Errorf("unmarshalling EC part %q for owner %q: %w", id, ownerID, err)
	}

	return &p, nil
}

// Create implements repository.ECPartRepository.
func (r *ECPartRepository) Create(ctx context.Context, p repository.ECPart) (*repository.ECPart, error) {
	ownerID, ok := apictx.UserID(ctx)
	if !ok {
		return nil, fmt.Errorf("creating EC part %q: %w", p.ID, repository.ErrNoUserID)
	}
	p.UserID = ownerID

	item, err := attributevalue.MarshalMap(p)
	if err != nil {
		return nil, fmt.Errorf("marshalling EC part %q for owner %q: %w", p.ID, p.UserID, err)
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           &r.tableName,
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(id)"),
	})
	if err != nil {
		if _, ok := errors.AsType[*types.ConditionalCheckFailedException](err); ok {
			return nil, repository.ErrAlreadyExists
		}
		return nil, fmt.Errorf("creating EC part %q for owner %q: %w", p.ID, p.UserID, err)
	}

	return &p, nil
}

// Update rewrites the caller's EC part from the request body. image_path is
// left unnamed so it carries forward - the body never sets it.
//
// The condition is attribute_exists(id) alone: a full-body write is
// last-write-wins, but it must not resurrect a row a concurrent Delete just
// removed. id is the sort key, so the condition can only fail when no item
// exists at (user_id, id) - that's ErrNotFound.
func (r *ECPartRepository) Update(ctx context.Context, p repository.ECPart) (*repository.ECPart, error) {
	ownerID, ok := apictx.UserID(ctx)
	if !ok {
		return nil, fmt.Errorf("updating EC part %q: %w", p.ID, repository.ErrNoUserID)
	}

	update := expression.
		Set(expression.Name("brand"), expression.Value(p.Brand)).
		Set(expression.Name("name"), expression.Value(p.Name)).
		Set(expression.Name("part_type"), expression.Value(p.PartType)).
		Set(expression.Name("visibility"), expression.Value(p.Visibility))
	update = setOrRemovePtr(update, "domes", p.Domes)
	update = setOrRemovePtr(update, "sliders", p.Sliders)
	update = setOrRemovePtr(update, "housings", p.Housings)
	update = setOrRemovePtr(update, "springs", p.Springs)
	update = setOrRemovePtr(update, "rings", p.Rings)
	update = setOrRemovePtr(update, "notes", p.Notes)
	update = setOrRemoveSlice(update, "purchases", p.Purchases)

	expr, err := expression.NewBuilder().
		WithUpdate(update).
		WithCondition(expression.AttributeExists(expression.Name("id"))).
		Build()
	if err != nil {
		return nil, fmt.Errorf("building EC part update expression for %q: %w", p.ID, err)
	}

	out, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 &r.tableName,
		Key:                       ecPartKey(ownerID, p.ID),
		UpdateExpression:          expr.Update(),
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ReturnValues:              types.ReturnValueAllNew,
	})
	if err != nil {
		if _, ok := errors.AsType[*types.ConditionalCheckFailedException](err); ok {
			return nil, repository.ErrNotFound
		}
		return nil, fmt.Errorf("updating EC part %q for owner %q: %w", p.ID, ownerID, err)
	}

	var updated repository.ECPart
	if err := attributevalue.UnmarshalMap(out.Attributes, &updated); err != nil {
		return nil, fmt.Errorf("unmarshalling updated EC part %q for owner %q: %w", p.ID, ownerID, err)
	}

	return &updated, nil
}

// Delete implements repository.ECPartRepository. Idempotent: a nonexistent
// id is not an error.
func (r *ECPartRepository) Delete(ctx context.Context, id string) error {
	ownerID, ok := apictx.UserID(ctx)
	if !ok {
		return fmt.Errorf("deleting EC part %q: %w", id, repository.ErrNoUserID)
	}

	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &r.tableName,
		Key:       ecPartKey(ownerID, id),
	})
	if err != nil {
		return fmt.Errorf("deleting EC part %q for owner %q: %w", id, ownerID, err)
	}

	return nil
}

// DeleteAll implements repository.ECPartRepository.
func (r *ECPartRepository) DeleteAll(ctx context.Context) error {
	return deleteOwnerItems(ctx, r.client, r.tableName)
}

func ecPartKey(ownerID, partID string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"user_id": &types.AttributeValueMemberS{Value: ownerID},
		"id":      &types.AttributeValueMemberS{Value: partID},
	}
}

// SetImagePath implements repository.ECPartRepository.
func (r *ECPartRepository) SetImagePath(ctx context.Context, id string, key repository.ECPartImageKey) error {
	ownerID, ok := apictx.UserID(ctx)
	if !ok {
		return fmt.Errorf("setting image path for EC part %q: %w", id, repository.ErrNoUserID)
	}

	update := expression.Set(expression.Name("image_path"), expression.Value(key)).
		Set(expression.Name("image_status"), expression.Value(repository.ImageStatusPending)).
		Remove(expression.Name("image_rejection_reason"))
	expr, err := expression.NewBuilder().
		WithUpdate(update).
		WithCondition(expression.AttributeExists(expression.Name("id"))).
		Build()
	if err != nil {
		return fmt.Errorf("building image path update for EC part %q: %w", id, err)
	}

	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 &r.tableName,
		Key:                       ecPartKey(ownerID, id),
		UpdateExpression:          expr.Update(),
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	})
	if err != nil {
		if _, ok := errors.AsType[*types.ConditionalCheckFailedException](err); ok {
			return repository.ErrNotFound
		}
		return fmt.Errorf("setting image path for EC part %q owner %q: %w", id, ownerID, err)
	}

	return nil
}

// ClearImagePath implements repository.ECPartRepository. ALL_OLD reports the
// key that was cleared, or nil when nothing was set.
func (r *ECPartRepository) ClearImagePath(ctx context.Context, id string) (*repository.ECPartImageKey, error) {
	ownerID, ok := apictx.UserID(ctx)
	if !ok {
		return nil, fmt.Errorf("clearing image path for EC part %q: %w", id, repository.ErrNoUserID)
	}

	update := expression.Remove(expression.Name("image_path")).
		Remove(expression.Name("image_status")).
		Remove(expression.Name("image_rejection_reason"))
	expr, err := expression.NewBuilder().
		WithUpdate(update).
		WithCondition(expression.AttributeExists(expression.Name("id"))).
		Build()
	if err != nil {
		return nil, fmt.Errorf("building image path clear for EC part %q: %w", id, err)
	}

	out, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 &r.tableName,
		Key:                       ecPartKey(ownerID, id),
		UpdateExpression:          expr.Update(),
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ReturnValues:              types.ReturnValueAllOld,
	})
	if err != nil {
		if _, ok := errors.AsType[*types.ConditionalCheckFailedException](err); ok {
			return nil, repository.ErrNotFound
		}
		return nil, fmt.Errorf("clearing image path for EC part %q owner %q: %w", id, ownerID, err)
	}

	old := struct {
		ImagePath *repository.ECPartImageKey `dynamodbav:"image_path"`
	}{}
	if err := attributevalue.UnmarshalMap(out.Attributes, &old); err != nil {
		return nil, fmt.Errorf("unmarshalling cleared EC part %q image path for owner %q: %w", id, ownerID, err)
	}
	if old.ImagePath == nil {
		return nil, nil //nolint:nilnil // no image already set is a valid, expected result
	}

	return old.ImagePath, nil
}

// SetImageStatus implements repository.ECPartRepository.
func (r *ECPartRepository) SetImageStatus(ctx context.Context, ownerID, id string, forPath repository.ECPartImageKey, from, to repository.ImageStatus, reason *string) (bool, error) {
	update := statusUpdate(expression.Name("image_status"), expression.Name("image_rejection_reason"), to, reason)
	cond := expression.AttributeExists(expression.Name("id")).
		And(expression.Name("image_path").Equal(expression.Value(forPath))).
		And(statusIs(expression.Name("image_status"), from))

	expr, err := expression.NewBuilder().WithUpdate(update).WithCondition(cond).Build()
	if err != nil {
		return false, fmt.Errorf("building image status update for EC part %q: %w", id, err)
	}

	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 &r.tableName,
		Key:                       ecPartKey(ownerID, id),
		UpdateExpression:          expr.Update(),
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	})
	if err != nil {
		if _, ok := errors.AsType[*types.ConditionalCheckFailedException](err); ok {
			return false, nil
		}
		return false, fmt.Errorf("setting image status for EC part %q owner %q: %w", id, ownerID, err)
	}

	return true, nil
}
