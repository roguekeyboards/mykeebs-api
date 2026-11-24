package dynamo

import (
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	apictx "github.com/roguekeyboards/mykeebs-api/internal/ctx"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/dynamo/mocks"
)

type SwitchRepositorySuite struct {
	suite.Suite

	mockClient *mocks.MockDynamoAPI
	repo       *SwitchRepository
}

func TestSwitchRepositorySuite(t *testing.T) {
	suite.Run(t, new(SwitchRepositorySuite))
}

func (s *SwitchRepositorySuite) SetupTest() {
	s.mockClient = mocks.NewMockDynamoAPI(s.T())
	s.repo = &SwitchRepository{client: s.mockClient, tableName: "switch-table"}
}

func (s *SwitchRepositorySuite) TestList_Succeeds() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			return len(in.ExclusiveStartKey) == 0 && *in.Limit == queryReadSize
		})).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{
				{
					"user_id": &types.AttributeValueMemberS{Value: "alice"},
					"id":      &types.AttributeValueMemberS{Value: "sw1"},
					"brand":   &types.AttributeValueMemberS{Value: "Gateron"},
				},
			},
		}, nil)

	switches, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")

	s.Require().NoError(err)
	s.Empty(next)
	s.Require().Len(switches, 1)
	s.Equal("sw1", switches[0].ID)
	s.Equal("Gateron", switches[0].Brand)
}

func (s *SwitchRepositorySuite) TestList_EmptyVisibilities_ReturnsEmptySliceWithoutQuerying() {
	// No EXPECT() on s.mockClient.Query - an empty visibilities slice must
	// short-circuit before building a Query, since expression.In(...)
	// requires at least one value and would otherwise panic.
	switches, next, err := s.repo.List(s.T().Context(), "alice", nil, 20, "")

	s.Require().NoError(err)
	s.NotNil(switches)
	s.Empty(switches)
	s.Empty(next)
}

func (s *SwitchRepositorySuite) TestList_EmptyResult_ReturnsEmptySliceNotNil() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil)

	switches, _, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")

	s.Require().NoError(err)
	s.NotNil(switches)
	s.Empty(switches)
}

func (s *SwitchRepositorySuite) TestList_ReturnsEncodedCursor_WhenMorePagesExist() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{{
				"user_id": &types.AttributeValueMemberS{Value: "alice"},
				"id":      &types.AttributeValueMemberS{Value: "sw1"},
			}},
			LastEvaluatedKey: map[string]types.AttributeValue{
				"user_id": &types.AttributeValueMemberS{Value: "alice"},
				"id":      &types.AttributeValueMemberS{Value: "sw1"},
			},
		}, nil)

	_, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 1, "")

	s.Require().NoError(err)
	s.NotEmpty(next)
}

func (s *SwitchRepositorySuite) TestList_DecodesCursor_IntoExclusiveStartKey() {
	// Round-trip: encode a key via a first call, then confirm the second
	// call's ExclusiveStartKey matches what was encoded.
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			return len(in.ExclusiveStartKey) == 0
		})).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{{
				"user_id": &types.AttributeValueMemberS{Value: "alice"},
				"id":      &types.AttributeValueMemberS{Value: "sw1"},
			}},
			LastEvaluatedKey: map[string]types.AttributeValue{
				"user_id": &types.AttributeValueMemberS{Value: "alice"},
				"id":      &types.AttributeValueMemberS{Value: "sw1"},
			},
		}, nil).Once()

	_, cursor, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 1, "")
	s.Require().NoError(err)

	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			key, ok := in.ExclusiveStartKey["id"].(*types.AttributeValueMemberS)
			return ok && key.Value == "sw1"
		})).
		Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil).Once()

	_, _, err = s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, cursor)
	s.Require().NoError(err)
}

func (s *SwitchRepositorySuite) TestList_InvalidCursor_ReturnsErrInvalidCursor() {
	switches, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "not-valid-base64!!")

	s.Require().ErrorIs(err, repository.ErrInvalidCursor)
	s.Nil(switches)
	s.Empty(next)
}

func (s *SwitchRepositorySuite) TestList_QueryError_Propagates() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	switches, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")

	s.Require().Error(err)
	s.Nil(switches)
	s.Empty(next)
}

func (s *SwitchRepositorySuite) TestGet_Succeeds() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.GetItemInput) bool {
			userID, ok := in.Key["user_id"].(*types.AttributeValueMemberS)
			id, ok2 := in.Key["id"].(*types.AttributeValueMemberS)
			return ok && ok2 && userID.Value == "alice" && id.Value == "sw1"
		})).
		Return(&dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"user_id": &types.AttributeValueMemberS{Value: "alice"},
				"id":      &types.AttributeValueMemberS{Value: "sw1"},
				"brand":   &types.AttributeValueMemberS{Value: "Gateron"},
			},
		}, nil)

	sw, err := s.repo.Get(s.T().Context(), "alice", "sw1")

	s.Require().NoError(err)
	s.Equal("sw1", sw.ID)
	s.Equal("Gateron", sw.Brand)
}

func (s *SwitchRepositorySuite) TestGet_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	sw, err := s.repo.Get(s.T().Context(), "alice", "missing")

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(sw)
}

func (s *SwitchRepositorySuite) TestGet_GetItemError_Propagates() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	sw, err := s.repo.Get(s.T().Context(), "alice", "sw1")

	s.Require().Error(err)
	s.Nil(sw)
}

func (s *SwitchRepositorySuite) TestCreate_Succeeds() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	sw, err := s.repo.Create(ctx, repository.Switch{ID: "sw1", Brand: "Gateron"})

	s.Require().NoError(err)
	s.Equal(&repository.Switch{UserID: "alice", ID: "sw1", Brand: "Gateron"}, sw)
}

func (s *SwitchRepositorySuite) TestCreate_AlreadyExists_ReturnsErrAlreadyExists() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	sw, err := s.repo.Create(ctx, repository.Switch{ID: "sw1"})

	s.Require().ErrorIs(err, repository.ErrAlreadyExists)
	s.Nil(sw)
}

func (s *SwitchRepositorySuite) TestCreate_PutItemError_Propagates() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	sw, err := s.repo.Create(ctx, repository.Switch{ID: "sw1"})

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrAlreadyExists)
	s.Nil(sw)
}

func (s *SwitchRepositorySuite) TestCreate_NoUserIDInContext_ReturnsError() {
	// No EXPECT() on PutItem - see repository.ErrNoUserID.
	sw, err := s.repo.Create(s.T().Context(), repository.Switch{ID: "sw1"})

	s.Require().Error(err)
	s.Nil(sw)
}

// updatedItem is a stand-in for an UpdateItem ALL_NEW response.
func (s *SwitchRepositorySuite) updatedItem() map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"user_id": &types.AttributeValueMemberS{Value: "alice"},
		"id":      &types.AttributeValueMemberS{Value: "sw1"},
		"brand":   &types.AttributeValueMemberS{Value: "Gateron"},
	}
}

func (s *SwitchRepositorySuite) TestUpdate_Succeeds() {
	// A bare EXPECT() on UpdateItem, no GetItem: the happy path never reads.
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.UpdateItemInput) bool {
			key := in.Key["id"].(*types.AttributeValueMemberS)
			return key.Value == "sw1" &&
				strings.Contains(*in.ConditionExpression, "attribute_exists") &&
				in.ReturnValues == types.ReturnValueAllNew
		})).
		Return(&dynamodb.UpdateItemOutput{Attributes: s.updatedItem()}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	sw, err := s.repo.Update(ctx, repository.Switch{ID: "sw1", Brand: "Gateron"})

	s.Require().NoError(err)
	s.Equal("Gateron", sw.Brand)
}

func (s *SwitchRepositorySuite) TestUpdate_OmittedOptionalFields_AreRemoved() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.UpdateItemInput) bool {
			// the four nil optional fields become a REMOVE clause
			return strings.Contains(*in.UpdateExpression, "REMOVE")
		})).
		Return(&dynamodb.UpdateItemOutput{Attributes: s.updatedItem()}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	_, err := s.repo.Update(ctx, repository.Switch{ID: "sw1", Brand: "Gateron"})

	s.Require().NoError(err)
}

func (s *SwitchRepositorySuite) TestUpdate_NoPurchases_RemovesPurchases() {
	var captured *dynamodb.UpdateItemInput
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.UpdateItemInput) bool {
			captured = in
			return true
		})).
		Return(&dynamodb.UpdateItemOutput{Attributes: s.updatedItem()}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	_, err := s.repo.Update(ctx, repository.Switch{ID: "sw1", Brand: "Gateron"})

	s.Require().NoError(err)
	s.True(removesAttr(captured, "purchases"))
}

func (s *SwitchRepositorySuite) TestUpdate_WithPurchases_SetsPurchases() {
	var captured *dynamodb.UpdateItemInput
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.UpdateItemInput) bool {
			captured = in
			return true
		})).
		Return(&dynamodb.UpdateItemOutput{Attributes: s.updatedItem()}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	_, err := s.repo.Update(ctx, repository.Switch{ID: "sw1", Brand: "Gateron", Purchases: repository.BulkPurchases{{Quantity: new(70)}}})

	s.Require().NoError(err)
	s.True(setsAttr(captured, "purchases"))
}

func (s *SwitchRepositorySuite) TestUpdate_ReturnsPersistedImagePath() {
	after := s.updatedItem()
	after["image_path"] = &types.AttributeValueMemberS{Value: "switches/alice/sw1/image/up1"}
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(&dynamodb.UpdateItemOutput{Attributes: after}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	sw, err := s.repo.Update(ctx, repository.Switch{ID: "sw1", Brand: "Gateron"})

	s.Require().NoError(err)
	s.Require().NotNil(sw.ImagePath)
	s.Equal(repository.SwitchImageKey("switches/alice/sw1/image/up1"), *sw.ImagePath)
}

func (s *SwitchRepositorySuite) TestUpdate_ConditionFails_ReturnsErrNotFound() {
	// attribute_exists(id) on the sort key can only fail when no item exists
	// at (user_id, id) - a concurrent delete. No follow-up read.
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	sw, err := s.repo.Update(ctx, repository.Switch{ID: "sw1", Brand: "Gateron"})

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(sw)
}

func (s *SwitchRepositorySuite) TestUpdate_UpdateItemError_Propagates() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	sw, err := s.repo.Update(ctx, repository.Switch{ID: "sw1"})

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrNotFound)
	s.Nil(sw)
}

func (s *SwitchRepositorySuite) TestUpdate_NoUserIDInContext_ReturnsError() {
	// No EXPECT() on UpdateItem - see repository.ErrNoUserID.
	sw, err := s.repo.Update(s.T().Context(), repository.Switch{ID: "sw1"})

	s.Require().Error(err)
	s.Nil(sw)
}

func (s *SwitchRepositorySuite) TestDelete_Succeeds() {
	s.mockClient.EXPECT().
		DeleteItem(mock.Anything, mock.Anything).
		Return(&dynamodb.DeleteItemOutput{}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	err := s.repo.Delete(ctx, "sw1")

	s.Require().NoError(err)
}

func (s *SwitchRepositorySuite) TestDelete_NoUserIDInContext_ReturnsError() {
	// No EXPECT() on DeleteItem - see repository.ErrNoUserID.
	err := s.repo.Delete(s.T().Context(), "sw1")

	s.Require().Error(err)
}

func (s *SwitchRepositorySuite) TestDelete_DeleteItemError_Propagates() {
	s.mockClient.EXPECT().
		DeleteItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	err := s.repo.Delete(ctx, "sw1")

	s.Require().Error(err)
}

func (s *SwitchRepositorySuite) TestSetImagePath_Succeeds() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.UpdateItemInput) bool {
			return strings.Contains(*in.UpdateExpression, "SET") &&
				strings.Contains(*in.ConditionExpression, "attribute_exists")
		})).
		Return(&dynamodb.UpdateItemOutput{}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	err := s.repo.SetImagePath(ctx, "sw1", "switches/alice/sw1/image/up1")

	s.Require().NoError(err)
}

func (s *SwitchRepositorySuite) TestSetImagePath_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	err := s.repo.SetImagePath(ctx, "sw1", "p")

	s.Require().ErrorIs(err, repository.ErrNotFound)
}

func (s *SwitchRepositorySuite) TestSetImagePath_UpdateItemError_Propagates() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	err := s.repo.SetImagePath(ctx, "sw1", "p")

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrNotFound)
}

func (s *SwitchRepositorySuite) TestSetImagePath_NoUserIDInContext_ReturnsError() {
	// No EXPECT() on UpdateItem - see repository.ErrNoUserID.
	err := s.repo.SetImagePath(s.T().Context(), "sw1", "p")

	s.Require().Error(err)
}

func (s *SwitchRepositorySuite) TestClearImagePath_Succeeds() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.UpdateItemInput) bool {
			return strings.Contains(*in.UpdateExpression, "REMOVE") &&
				in.ReturnValues == types.ReturnValueAllOld
		})).
		Return(&dynamodb.UpdateItemOutput{Attributes: map[string]types.AttributeValue{
			"image_path": &types.AttributeValueMemberS{Value: "switches/alice/sw1/image/up1"},
		}}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	cleared, err := s.repo.ClearImagePath(ctx, "sw1")

	s.Require().NoError(err)
	s.Require().NotNil(cleared)
	s.Equal(repository.SwitchImageKey("switches/alice/sw1/image/up1"), *cleared)
}

func (s *SwitchRepositorySuite) TestClearImagePath_ImageAlreadyAbsent_ReturnsNilWithoutError() {
	// The REMOVE still runs; ALL_OLD without image_path means nothing was set.
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(&dynamodb.UpdateItemOutput{Attributes: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: "sw1"},
		}}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	cleared, err := s.repo.ClearImagePath(ctx, "sw1")

	s.Require().NoError(err)
	s.Nil(cleared)
}

func (s *SwitchRepositorySuite) TestClearImagePath_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	cleared, err := s.repo.ClearImagePath(ctx, "sw1")

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(cleared)
}

func (s *SwitchRepositorySuite) TestClearImagePath_NoUserIDInContext_ReturnsError() {
	// No EXPECT() on UpdateItem - see repository.ErrNoUserID.
	cleared, err := s.repo.ClearImagePath(s.T().Context(), "sw1")

	s.Require().Error(err)
	s.Nil(cleared)
}

func (s *SwitchRepositorySuite) TestSetImagePath_MarksPending() {
	var captured *dynamodb.UpdateItemInput
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.UpdateItemInput) bool {
			captured = in
			return true
		})).
		Return(&dynamodb.UpdateItemOutput{}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	s.Require().NoError(s.repo.SetImagePath(ctx, "sw1", "switches/alice/sw1/image/up1"))

	s.True(setsAttr(captured, "image_status"))
	s.True(exprValuesContainS(captured.ExpressionAttributeValues, "pending"))
	s.True(removesAttr(captured, "image_rejection_reason"))
}

func (s *SwitchRepositorySuite) TestClearImagePath_ClearsStatus() {
	var captured *dynamodb.UpdateItemInput
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.UpdateItemInput) bool {
			captured = in
			return true
		})).
		Return(&dynamodb.UpdateItemOutput{}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	_, err := s.repo.ClearImagePath(ctx, "sw1")
	s.Require().NoError(err)

	s.True(removesAttr(captured, "image_status"))
	s.True(removesAttr(captured, "image_rejection_reason"))
}

func (s *SwitchRepositorySuite) TestSetImageStatus_ConditionedOnPathAndStatus() {
	var captured *dynamodb.UpdateItemInput
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.UpdateItemInput) bool {
			captured = in
			return true
		})).
		Return(&dynamodb.UpdateItemOutput{}, nil)

	reason := "unprocessable"
	ok, err := s.repo.SetImageStatus(s.T().Context(), "alice", "sw1", "switches/alice/sw1/image/up1", repository.ImageStatusPending, repository.ImageStatusRejected, &reason)

	s.Require().NoError(err)
	s.True(ok)
	s.Equal("alice", captured.Key["user_id"].(*types.AttributeValueMemberS).Value)
	s.Equal("sw1", captured.Key["id"].(*types.AttributeValueMemberS).Value)
	s.True(names(captured.ExpressionAttributeNames, "image_path", "image_status"))
	s.True(exprValuesContainS(captured.ExpressionAttributeValues, "switches/alice/sw1/image/up1"))
	s.True(exprValuesContainS(captured.ExpressionAttributeValues, "pending"))
	s.True(setsAttr(captured, "image_status"))
	s.True(exprValuesContainS(captured.ExpressionAttributeValues, "rejected"))
	s.True(setsAttr(captured, "image_rejection_reason"))
	s.True(exprValuesContainS(captured.ExpressionAttributeValues, "unprocessable"))
}

func (s *SwitchRepositorySuite) TestSetImageStatus_ConditionFails_ReturnsFalseWithoutError() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})

	ok, err := s.repo.SetImageStatus(s.T().Context(), "alice", "sw1", "switches/alice/sw1/image/up1", repository.ImageStatusPending, repository.ImageStatusApproved, nil)

	s.Require().NoError(err)
	s.False(ok)
}

func (s *SwitchRepositorySuite) TestSetImageStatus_UpdateItemError_Propagates() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ok, err := s.repo.SetImageStatus(s.T().Context(), "alice", "sw1", "switches/alice/sw1/image/up1", repository.ImageStatusPending, repository.ImageStatusApproved, nil)

	s.Require().Error(err)
	s.False(ok)
}
