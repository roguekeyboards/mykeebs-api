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

// SwitchRepository is the DynamoDB-backed repository.SwitchRepository.
type SwitchRepository struct {
	client    dynamoAPI
	tableName string
}

var _ repository.SwitchRepository = (*SwitchRepository)(nil)

// NewSwitchRepository returns a SwitchRepository backed by client.
func NewSwitchRepository(client *dynamodb.Client, tableName string) *SwitchRepository {
	return &SwitchRepository{client: client, tableName: tableName}
}

// List implements repository.SwitchRepository.
func (r *SwitchRepository) List(
	ctx context.Context,
	ownerID string,
	visibilities []repository.Visibility,
	limit int,
	cursor string,
) ([]repository.Switch, string, error) {
	// No visibility tier is readable, so nothing can match - short-circuit
	// rather than build a Query with an empty IN(...) filter (which the
	// expression builder below would panic constructing).
	if len(visibilities) == 0 {
		return []repository.Switch{}, "", nil
	}

	startKey, _, err := decodeOwnerCursor(cursor, ownerID)
	if err != nil {
		return nil, "", err
	}

	visValues := make([]expression.OperandBuilder, len(visibilities))
	for i, v := range visibilities {
		visValues[i] = expression.Value(v)
	}

	builder := expression.NewBuilder().
		WithKeyCondition(expression.Key("user_id").Equal(expression.Value(ownerID))).
		WithFilter(expression.Name("visibility").In(visValues[0], visValues[1:]...))

	expr, err := builder.Build()
	if err != nil {
		return nil, "", fmt.Errorf("building switch list expression: %w", err)
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
		return nil, "", fmt.Errorf("querying switches for owner %q: %w", ownerID, err)
	}

	switches := []repository.Switch{}
	if err := attributevalue.UnmarshalListOfMaps(items, &switches); err != nil {
		return nil, "", fmt.Errorf("unmarshalling switches for owner %q: %w", ownerID, err)
	}

	nextCursor, err := encodeCursor(lastKey, "", "")
	if err != nil {
		return nil, "", fmt.Errorf("encoding next cursor: %w", err)
	}

	return switches, nextCursor, nil
}

// Get implements repository.SwitchRepository.
func (r *SwitchRepository) Get(ctx context.Context, ownerID, id string) (*repository.Switch, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &r.tableName,
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: ownerID},
			"id":      &types.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("getting switch %q for owner %q: %w", id, ownerID, err)
	}

	if len(out.Item) == 0 {
		return nil, repository.ErrNotFound
	}

	var sw repository.Switch
	if err := attributevalue.UnmarshalMap(out.Item, &sw); err != nil {
		return nil, fmt.Errorf("unmarshalling switch %q for owner %q: %w", id, ownerID, err)
	}

	return &sw, nil
}

// Create implements repository.SwitchRepository.
func (r *SwitchRepository) Create(ctx context.Context, sw repository.Switch) (*repository.Switch, error) {
	ownerID, ok := apictx.UserID(ctx)
	if !ok {
		return nil, fmt.Errorf("creating switch %q: %w", sw.ID, repository.ErrNoUserID)
	}
	sw.UserID = ownerID

	item, err := attributevalue.MarshalMap(sw)
	if err != nil {
		return nil, fmt.Errorf("marshalling switch %q for owner %q: %w", sw.ID, sw.UserID, err)
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
		return nil, fmt.Errorf("creating switch %q for owner %q: %w", sw.ID, sw.UserID, err)
	}

	return &sw, nil
}

// Update rewrites the caller's switch from the request body. image_path is
// left unnamed so it carries forward - the body never sets it.
//
// The condition is attribute_exists(id) alone: a full-body write is
// last-write-wins, but it must not resurrect a row a concurrent Delete just
// removed. id is the sort key, so the condition can only fail when no item
// exists at (user_id, id) - that's ErrNotFound.
func (r *SwitchRepository) Update(ctx context.Context, sw repository.Switch) (*repository.Switch, error) {
	ownerID, ok := apictx.UserID(ctx)
	if !ok {
		return nil, fmt.Errorf("updating switch %q: %w", sw.ID, repository.ErrNoUserID)
	}

	update := expression.
		Set(expression.Name("brand"), expression.Value(sw.Brand)).
		Set(expression.Name("name"), expression.Value(sw.Name)).
		Set(expression.Name("type"), expression.Value(sw.Type)).
		Set(expression.Name("material"), expression.Value(sw.Material)).
		Set(expression.Name("force"), expression.Value(sw.Force)).
		Set(expression.Name("spring"), expression.Value(sw.Spring)).
		Set(expression.Name("visibility"), expression.Value(sw.Visibility))
	update = setOrRemovePtr(update, "manufacturer", sw.Manufacturer)
	update = setOrRemovePtr(update, "pins", sw.Pins)
	update = setOrRemovePtr(update, "factory_lubed", sw.FactoryLubed)
	update = setOrRemovePtr(update, "notes", sw.Notes)
	update = setOrRemoveSlice(update, "purchases", sw.Purchases)

	expr, err := expression.NewBuilder().
		WithUpdate(update).
		WithCondition(expression.AttributeExists(expression.Name("id"))).
		Build()
	if err != nil {
		return nil, fmt.Errorf("building switch update expression for switch %q: %w", sw.ID, err)
	}

	out, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 &r.tableName,
		Key:                       switchKey(ownerID, sw.ID),
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
		return nil, fmt.Errorf("updating switch %q for owner %q: %w", sw.ID, ownerID, err)
	}

	var updated repository.Switch
	if err := attributevalue.UnmarshalMap(out.Attributes, &updated); err != nil {
		return nil, fmt.Errorf("unmarshalling updated switch %q for owner %q: %w", sw.ID, ownerID, err)
	}

	return &updated, nil
}

// Delete implements repository.SwitchRepository. Idempotent: a nonexistent
// id is not an error.
func (r *SwitchRepository) Delete(ctx context.Context, id string) error {
	ownerID, ok := apictx.UserID(ctx)
	if !ok {
		return fmt.Errorf("deleting switch %q: %w", id, repository.ErrNoUserID)
	}

	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &r.tableName,
		Key:       switchKey(ownerID, id),
	})
	if err != nil {
		return fmt.Errorf("deleting switch %q for owner %q: %w", id, ownerID, err)
	}

	return nil
}

// DeleteAll implements repository.SwitchRepository.
func (r *SwitchRepository) DeleteAll(ctx context.Context) error {
	return deleteOwnerItems(ctx, r.client, r.tableName)
}

func switchKey(ownerID, switchID string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"user_id": &types.AttributeValueMemberS{Value: ownerID},
		"id":      &types.AttributeValueMemberS{Value: switchID},
	}
}

// setOrRemovePtr SETs an optional field when set, REMOVEs it when nil, so an
// Update that omits a field clears it rather than leaving the stale value.
func setOrRemovePtr[T any](update expression.UpdateBuilder, name string, v *T) expression.UpdateBuilder {
	if v == nil {
		return update.Remove(expression.Name(name))
	}
	return update.Set(expression.Name(name), expression.Value(*v))
}

// setOrRemoveSlice is setOrRemovePtr for a list, REMOVEing it when empty.
func setOrRemoveSlice[S ~[]E, E any](update expression.UpdateBuilder, name string, v S) expression.UpdateBuilder {
	if len(v) == 0 {
		return update.Remove(expression.Name(name))
	}
	return update.Set(expression.Name(name), expression.Value(v))
}

// SetImagePath implements repository.SwitchRepository.
func (r *SwitchRepository) SetImagePath(ctx context.Context, id string, key repository.SwitchImageKey) error {
	ownerID, ok := apictx.UserID(ctx)
	if !ok {
		return fmt.Errorf("setting image path for switch %q: %w", id, repository.ErrNoUserID)
	}

	update := expression.Set(expression.Name("image_path"), expression.Value(key)).
		Set(expression.Name("image_status"), expression.Value(repository.ImageStatusPending)).
		Remove(expression.Name("image_rejection_reason"))
	expr, err := expression.NewBuilder().
		WithUpdate(update).
		WithCondition(expression.AttributeExists(expression.Name("id"))).
		Build()
	if err != nil {
		return fmt.Errorf("building switch image path update for switch %q: %w", id, err)
	}

	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 &r.tableName,
		Key:                       switchKey(ownerID, id),
		UpdateExpression:          expr.Update(),
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	})
	if err != nil {
		if _, ok := errors.AsType[*types.ConditionalCheckFailedException](err); ok {
			return repository.ErrNotFound
		}
		return fmt.Errorf("setting image path for switch %q owner %q: %w", id, ownerID, err)
	}

	return nil
}

// ClearImagePath implements repository.SwitchRepository. ALL_OLD reports the
// key that was cleared, or nil when nothing was set.
func (r *SwitchRepository) ClearImagePath(ctx context.Context, id string) (*repository.SwitchImageKey, error) {
	ownerID, ok := apictx.UserID(ctx)
	if !ok {
		return nil, fmt.Errorf("clearing image path for switch %q: %w", id, repository.ErrNoUserID)
	}

	update := expression.Remove(expression.Name("image_path")).
		Remove(expression.Name("image_status")).
		Remove(expression.Name("image_rejection_reason"))
	expr, err := expression.NewBuilder().
		WithUpdate(update).
		WithCondition(expression.AttributeExists(expression.Name("id"))).
		Build()
	if err != nil {
		return nil, fmt.Errorf("building switch image path clear for switch %q: %w", id, err)
	}

	out, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 &r.tableName,
		Key:                       switchKey(ownerID, id),
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
		return nil, fmt.Errorf("clearing image path for switch %q owner %q: %w", id, ownerID, err)
	}

	old := struct {
		ImagePath *repository.SwitchImageKey `dynamodbav:"image_path"`
	}{}
	if err := attributevalue.UnmarshalMap(out.Attributes, &old); err != nil {
		return nil, fmt.Errorf("unmarshalling cleared switch %q image path for owner %q: %w", id, ownerID, err)
	}
	if old.ImagePath == nil {
		return nil, nil //nolint:nilnil // no image already set is a valid, expected result
	}

	return old.ImagePath, nil
}

// SetImageStatus implements repository.SwitchRepository.
func (r *SwitchRepository) SetImageStatus(ctx context.Context, ownerID, id string, forPath repository.SwitchImageKey, from, to repository.ImageStatus, reason *string) (bool, error) {
	update := statusUpdate(expression.Name("image_status"), expression.Name("image_rejection_reason"), to, reason)
	cond := expression.AttributeExists(expression.Name("id")).
		And(expression.Name("image_path").Equal(expression.Value(forPath))).
		And(statusIs(expression.Name("image_status"), from))

	expr, err := expression.NewBuilder().WithUpdate(update).WithCondition(cond).Build()
	if err != nil {
		return false, fmt.Errorf("building image status update for switch %q: %w", id, err)
	}

	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 &r.tableName,
		Key:                       switchKey(ownerID, id),
		UpdateExpression:          expr.Update(),
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	})
	if err != nil {
		if _, ok := errors.AsType[*types.ConditionalCheckFailedException](err); ok {
			return false, nil
		}
		return false, fmt.Errorf("setting image status for switch %q owner %q: %w", id, ownerID, err)
	}

	return true, nil
}
