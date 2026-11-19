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

type ECPartRepositorySuite struct {
	suite.Suite

	mockClient *mocks.MockDynamoAPI
	repo       *ECPartRepository
}

func TestECPartRepositorySuite(t *testing.T) {
	suite.Run(t, new(ECPartRepositorySuite))
}

func (s *ECPartRepositorySuite) SetupTest() {
	s.mockClient = mocks.NewMockDynamoAPI(s.T())
	s.repo = &ECPartRepository{client: s.mockClient, tableName: "ec-part-table"}
}

func (s *ECPartRepositorySuite) TestList_Succeeds() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			return len(in.ExclusiveStartKey) == 0 && *in.Limit == queryReadSize
		})).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{
				{
					"user_id": &types.AttributeValueMemberS{Value: "alice"},
					"id":      &types.AttributeValueMemberS{Value: "p1"},
					"brand":   &types.AttributeValueMemberS{Value: "DynaCap"},
				},
			},
		}, nil)

	parts, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, "", 20, "")

	s.Require().NoError(err)
	s.Empty(next)
	s.Require().Len(parts, 1)
	s.Equal("p1", parts[0].ID)
	s.Equal("DynaCap", parts[0].Brand)
}

func (s *ECPartRepositorySuite) TestList_EmptyVisibilities_ReturnsEmptySliceWithoutQuerying() {
	parts, next, err := s.repo.List(s.T().Context(), "alice", nil, "", 20, "")

	s.Require().NoError(err)
	s.NotNil(parts)
	s.Empty(parts)
	s.Empty(next)
}

func (s *ECPartRepositorySuite) TestList_EmptyResult_ReturnsEmptySliceNotNil() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil)

	parts, _, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, "", 20, "")

	s.Require().NoError(err)
	s.NotNil(parts)
	s.Empty(parts)
}

func (s *ECPartRepositorySuite) TestList_ReturnsEncodedCursor_WhenMorePagesExist() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{{
				"user_id": &types.AttributeValueMemberS{Value: "alice"},
				"id":      &types.AttributeValueMemberS{Value: "p1"},
			}},
			LastEvaluatedKey: map[string]types.AttributeValue{
				"user_id": &types.AttributeValueMemberS{Value: "alice"},
				"id":      &types.AttributeValueMemberS{Value: "p1"},
			},
		}, nil)

	_, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, "", 1, "")

	s.Require().NoError(err)
	s.NotEmpty(next)
}

func (s *ECPartRepositorySuite) TestList_DecodesCursor_IntoExclusiveStartKey() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			return len(in.ExclusiveStartKey) == 0
		})).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{{
				"user_id": &types.AttributeValueMemberS{Value: "alice"},
				"id":      &types.AttributeValueMemberS{Value: "p1"},
			}},
			LastEvaluatedKey: map[string]types.AttributeValue{
				"user_id": &types.AttributeValueMemberS{Value: "alice"},
				"id":      &types.AttributeValueMemberS{Value: "p1"},
			},
		}, nil).Once()

	_, cursor, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, "", 1, "")
	s.Require().NoError(err)

	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			key, ok := in.ExclusiveStartKey["id"].(*types.AttributeValueMemberS)
			return ok && key.Value == "p1"
		})).
		Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil).Once()

	_, _, err = s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, "", 20, cursor)
	s.Require().NoError(err)
}

func (s *ECPartRepositorySuite) TestList_InvalidCursor_ReturnsErrInvalidCursor() {
	parts, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, "", 20, "not-valid-base64!!")

	s.Require().ErrorIs(err, repository.ErrInvalidCursor)
	s.Nil(parts)
	s.Empty(next)
}

func (s *ECPartRepositorySuite) TestList_QueryError_Propagates() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	parts, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, "", 20, "")

	s.Require().Error(err)
	s.Nil(parts)
	s.Empty(next)
}

func (s *ECPartRepositorySuite) TestGet_Succeeds() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.GetItemInput) bool {
			userID, ok := in.Key["user_id"].(*types.AttributeValueMemberS)
			id, ok2 := in.Key["id"].(*types.AttributeValueMemberS)
			return ok && ok2 && userID.Value == "alice" && id.Value == "p1"
		})).
		Return(&dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"user_id": &types.AttributeValueMemberS{Value: "alice"},
				"id":      &types.AttributeValueMemberS{Value: "p1"},
				"brand":   &types.AttributeValueMemberS{Value: "DynaCap"},
			},
		}, nil)

	p, err := s.repo.Get(s.T().Context(), "alice", "p1")

	s.Require().NoError(err)
	s.Equal("p1", p.ID)
	s.Equal("DynaCap", p.Brand)
}

func (s *ECPartRepositorySuite) TestGet_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	p, err := s.repo.Get(s.T().Context(), "alice", "missing")

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(p)
}

func (s *ECPartRepositorySuite) TestGet_GetItemError_Propagates() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	p, err := s.repo.Get(s.T().Context(), "alice", "p1")

	s.Require().Error(err)
	s.Nil(p)
}

func (s *ECPartRepositorySuite) TestCreate_Succeeds() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	p, err := s.repo.Create(ctx, repository.ECPart{ID: "p1", Brand: "DynaCap"})

	s.Require().NoError(err)
	s.Equal(&repository.ECPart{UserID: "alice", ID: "p1", Brand: "DynaCap"}, p)
}

func (s *ECPartRepositorySuite) TestCreate_AlreadyExists_ReturnsErrAlreadyExists() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	p, err := s.repo.Create(ctx, repository.ECPart{ID: "p1"})

	s.Require().ErrorIs(err, repository.ErrAlreadyExists)
	s.Nil(p)
}

func (s *ECPartRepositorySuite) TestCreate_PutItemError_Propagates() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	p, err := s.repo.Create(ctx, repository.ECPart{ID: "p1"})

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrAlreadyExists)
	s.Nil(p)
}

func (s *ECPartRepositorySuite) TestCreate_NoUserIDInContext_ReturnsError() {
	p, err := s.repo.Create(s.T().Context(), repository.ECPart{ID: "p1"})

	s.Require().Error(err)
	s.Nil(p)
}

// updatedItem is a stand-in for an UpdateItem ALL_NEW response.
func (s *ECPartRepositorySuite) updatedItem() map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"user_id": &types.AttributeValueMemberS{Value: "alice"},
		"id":      &types.AttributeValueMemberS{Value: "p1"},
		"brand":   &types.AttributeValueMemberS{Value: "DynaCap"},
	}
}

func (s *ECPartRepositorySuite) TestUpdate_Succeeds() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.UpdateItemInput) bool {
			key := in.Key["id"].(*types.AttributeValueMemberS)
			return key.Value == "p1" &&
				strings.Contains(*in.ConditionExpression, "attribute_exists") &&
				in.ReturnValues == types.ReturnValueAllNew
		})).
		Return(&dynamodb.UpdateItemOutput{Attributes: s.updatedItem()}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	p, err := s.repo.Update(ctx, repository.ECPart{ID: "p1", Brand: "DynaCap"})

	s.Require().NoError(err)
	s.Equal("DynaCap", p.Brand)
}

func (s *ECPartRepositorySuite) TestUpdate_OmittedOptionalFields_AreRemoved() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.UpdateItemInput) bool {
			return strings.Contains(*in.UpdateExpression, "REMOVE")
		})).
		Return(&dynamodb.UpdateItemOutput{Attributes: s.updatedItem()}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	_, err := s.repo.Update(ctx, repository.ECPart{ID: "p1", Brand: "DynaCap"})

	s.Require().NoError(err)
}

func (s *ECPartRepositorySuite) TestUpdate_ReturnsPersistedImagePath() {
	after := s.updatedItem()
	after["image_path"] = &types.AttributeValueMemberS{Value: "ec-parts/alice/p1/image/up1"}
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(&dynamodb.UpdateItemOutput{Attributes: after}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	p, err := s.repo.Update(ctx, repository.ECPart{ID: "p1", Brand: "DynaCap"})

	s.Require().NoError(err)
	s.Require().NotNil(p.ImagePath)
	s.Equal(repository.ECPartImageKey("ec-parts/alice/p1/image/up1"), *p.ImagePath)
}

func (s *ECPartRepositorySuite) TestUpdate_ConditionFails_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	p, err := s.repo.Update(ctx, repository.ECPart{ID: "p1", Brand: "DynaCap"})

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(p)
}

func (s *ECPartRepositorySuite) TestUpdate_UpdateItemError_Propagates() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	p, err := s.repo.Update(ctx, repository.ECPart{ID: "p1"})

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrNotFound)
	s.Nil(p)
}

func (s *ECPartRepositorySuite) TestUpdate_NoUserIDInContext_ReturnsError() {
	p, err := s.repo.Update(s.T().Context(), repository.ECPart{ID: "p1"})

	s.Require().Error(err)
	s.Nil(p)
}

func (s *ECPartRepositorySuite) TestDelete_Succeeds() {
	s.mockClient.EXPECT().
		DeleteItem(mock.Anything, mock.Anything).
		Return(&dynamodb.DeleteItemOutput{}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	err := s.repo.Delete(ctx, "p1")

	s.Require().NoError(err)
}

func (s *ECPartRepositorySuite) TestDelete_NoUserIDInContext_ReturnsError() {
	err := s.repo.Delete(s.T().Context(), "p1")

	s.Require().Error(err)
}

func (s *ECPartRepositorySuite) TestDelete_DeleteItemError_Propagates() {
	s.mockClient.EXPECT().
		DeleteItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	err := s.repo.Delete(ctx, "p1")

	s.Require().Error(err)
}

func (s *ECPartRepositorySuite) TestSetImagePath_Succeeds() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.UpdateItemInput) bool {
			return strings.Contains(*in.UpdateExpression, "SET") &&
				strings.Contains(*in.ConditionExpression, "attribute_exists")
		})).
		Return(&dynamodb.UpdateItemOutput{}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	err := s.repo.SetImagePath(ctx, "p1", "ec-parts/alice/p1/image/up1")

	s.Require().NoError(err)
}

func (s *ECPartRepositorySuite) TestSetImagePath_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	err := s.repo.SetImagePath(ctx, "p1", "p")

	s.Require().ErrorIs(err, repository.ErrNotFound)
}

func (s *ECPartRepositorySuite) TestSetImagePath_UpdateItemError_Propagates() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	err := s.repo.SetImagePath(ctx, "p1", "p")

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrNotFound)
}

func (s *ECPartRepositorySuite) TestSetImagePath_NoUserIDInContext_ReturnsError() {
	err := s.repo.SetImagePath(s.T().Context(), "p1", "p")

	s.Require().Error(err)
}

func (s *ECPartRepositorySuite) TestClearImagePath_Succeeds() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.UpdateItemInput) bool {
			return strings.Contains(*in.UpdateExpression, "REMOVE") &&
				in.ReturnValues == types.ReturnValueAllOld
		})).
		Return(&dynamodb.UpdateItemOutput{Attributes: map[string]types.AttributeValue{
			"image_path": &types.AttributeValueMemberS{Value: "ec-parts/alice/p1/image/up1"},
		}}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	cleared, err := s.repo.ClearImagePath(ctx, "p1")

	s.Require().NoError(err)
	s.Require().NotNil(cleared)
	s.Equal(repository.ECPartImageKey("ec-parts/alice/p1/image/up1"), *cleared)
}

func (s *ECPartRepositorySuite) TestClearImagePath_ImageAlreadyAbsent_ReturnsNilWithoutError() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(&dynamodb.UpdateItemOutput{Attributes: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: "p1"},
		}}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	cleared, err := s.repo.ClearImagePath(ctx, "p1")

	s.Require().NoError(err)
	s.Nil(cleared)
}

func (s *ECPartRepositorySuite) TestClearImagePath_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	cleared, err := s.repo.ClearImagePath(ctx, "p1")

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(cleared)
}

func (s *ECPartRepositorySuite) TestClearImagePath_NoUserIDInContext_ReturnsError() {
	cleared, err := s.repo.ClearImagePath(s.T().Context(), "p1")

	s.Require().Error(err)
	s.Nil(cleared)
}

func (s *ECPartRepositorySuite) TestSetImagePath_MarksPending() {
	var captured *dynamodb.UpdateItemInput
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.UpdateItemInput) bool {
			captured = in
			return true
		})).
		Return(&dynamodb.UpdateItemOutput{}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	s.Require().NoError(s.repo.SetImagePath(ctx, "p1", "ec-parts/alice/p1/image/up1"))

	s.True(setsAttr(captured, "image_status"))
	s.True(exprValuesContainS(captured.ExpressionAttributeValues, "pending"))
	s.True(removesAttr(captured, "image_rejection_reason"))
}

func (s *ECPartRepositorySuite) TestClearImagePath_ClearsStatus() {
	var captured *dynamodb.UpdateItemInput
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.UpdateItemInput) bool {
			captured = in
			return true
		})).
		Return(&dynamodb.UpdateItemOutput{}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	_, err := s.repo.ClearImagePath(ctx, "p1")
	s.Require().NoError(err)

	s.True(removesAttr(captured, "image_status"))
	s.True(removesAttr(captured, "image_rejection_reason"))
}

func (s *ECPartRepositorySuite) TestSetImageStatus_ConditionedOnPathAndStatus() {
	var captured *dynamodb.UpdateItemInput
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.UpdateItemInput) bool {
			captured = in
			return true
		})).
		Return(&dynamodb.UpdateItemOutput{}, nil)

	reason := "unprocessable"
	ok, err := s.repo.SetImageStatus(s.T().Context(), "alice", "p1", "ec-parts/alice/p1/image/up1", repository.ImageStatusPending, repository.ImageStatusRejected, &reason)

	s.Require().NoError(err)
	s.True(ok)
	s.Equal("alice", captured.Key["user_id"].(*types.AttributeValueMemberS).Value)
	s.Equal("p1", captured.Key["id"].(*types.AttributeValueMemberS).Value)
	s.True(names(captured.ExpressionAttributeNames, "image_path", "image_status"))
	s.True(exprValuesContainS(captured.ExpressionAttributeValues, "ec-parts/alice/p1/image/up1"))
	s.True(exprValuesContainS(captured.ExpressionAttributeValues, "pending"))
	s.True(setsAttr(captured, "image_status"))
	s.True(exprValuesContainS(captured.ExpressionAttributeValues, "rejected"))
	s.True(setsAttr(captured, "image_rejection_reason"))
	s.True(exprValuesContainS(captured.ExpressionAttributeValues, "unprocessable"))
}

func (s *ECPartRepositorySuite) TestSetImageStatus_ConditionFails_ReturnsFalseWithoutError() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})

	ok, err := s.repo.SetImageStatus(s.T().Context(), "alice", "p1", "ec-parts/alice/p1/image/up1", repository.ImageStatusPending, repository.ImageStatusApproved, nil)

	s.Require().NoError(err)
	s.False(ok)
}

func (s *ECPartRepositorySuite) TestSetImageStatus_UpdateItemError_Propagates() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ok, err := s.repo.SetImageStatus(s.T().Context(), "alice", "p1", "ec-parts/alice/p1/image/up1", repository.ImageStatusPending, repository.ImageStatusApproved, nil)

	s.Require().Error(err)
	s.False(ok)
}

func (s *ECPartRepositorySuite) TestList_PartType_FiltersOnIt() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			for _, v := range in.ExpressionAttributeValues {
				if str, ok := v.(*types.AttributeValueMemberS); ok && str.Value == "domes" {
					return strings.Contains(*in.FilterExpression, " AND ")
				}
			}
			return false
		})).
		Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil)

	_, _, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, repository.ECPartTypeDomes, 20, "")

	s.Require().NoError(err)
}

func (s *ECPartRepositorySuite) TestList_NoPartType_DoesNotFilterOnIt() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			return !strings.Contains(*in.FilterExpression, " AND ")
		})).
		Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil)

	_, _, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, "", 20, "")

	s.Require().NoError(err)
}

func (s *ECPartRepositorySuite) TestList_CursorFromAnotherPartType_ReturnsErrInvalidCursor() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{{
				"user_id": &types.AttributeValueMemberS{Value: "alice"},
				"id":      &types.AttributeValueMemberS{Value: "p1"},
			}},
			LastEvaluatedKey: map[string]types.AttributeValue{
				"user_id": &types.AttributeValueMemberS{Value: "alice"},
				"id":      &types.AttributeValueMemberS{Value: "p1"},
			},
		}, nil).Once()

	_, cursor, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, repository.ECPartTypeDomes, 1, "")
	s.Require().NoError(err)

	parts, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, repository.ECPartTypeSprings, 20, cursor)

	s.Require().ErrorIs(err, repository.ErrInvalidCursor)
	s.Nil(parts)
	s.Empty(next)
}

func (s *ECPartRepositorySuite) TestUpdate_SetSubObject_IsWrittenAndOthersRemoved() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.UpdateItemInput) bool {
			var setDomes, removeSliders bool
			for placeholder, name := range in.ExpressionAttributeNames {
				switch name {
				case "domes":
					setDomes = strings.Contains(*in.UpdateExpression, placeholder+" = ")
				case "sliders":
					removeSliders = strings.Contains(*in.UpdateExpression, "REMOVE") &&
						!strings.Contains(*in.UpdateExpression, placeholder+" = ")
				}
			}
			return setDomes && removeSliders
		})).
		Return(&dynamodb.UpdateItemOutput{Attributes: s.updatedItem()}, nil)

	ctx := apictx.WithUserID(s.T().Context(), "alice")
	_, err := s.repo.Update(ctx, repository.ECPart{
		ID: "p1", Brand: "DynaCap", PartType: repository.ECPartTypeDomes,
		Domes: &repository.ECDomes{Weight: new(45.0)},
	})

	s.Require().NoError(err)
}

func (s *ECPartRepositorySuite) TestGet_UnmarshalsSubObject() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"user_id":   &types.AttributeValueMemberS{Value: "alice"},
				"id":        &types.AttributeValueMemberS{Value: "p1"},
				"part_type": &types.AttributeValueMemberS{Value: "sliders"},
				"sliders": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
					"size": &types.AttributeValueMemberS{Value: "2U"},
					"stem": &types.AttributeValueMemberS{Value: "OEM"},
				}},
			},
		}, nil)

	p, err := s.repo.Get(s.T().Context(), "alice", "p1")

	s.Require().NoError(err)
	s.Equal(repository.ECPartTypeSliders, p.PartType)
	s.Require().NotNil(p.Sliders)
	s.Equal(repository.ECPartSize2U, *p.Sliders.Size)
	s.Equal(repository.ECSliderStemOEM, *p.Sliders.Stem)
}
