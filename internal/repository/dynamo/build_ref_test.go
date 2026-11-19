package dynamo

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/dynamo/mocks"
)

type BuildRefSuite struct {
	suite.Suite
}

func TestBuildRefSuite(t *testing.T) {
	suite.Run(t, new(BuildRefSuite))
}

func (s *BuildRefSuite) TestDeriveRefMarkers_KeyboardOnly() {
	b := repository.Build{ID: "b1", Keyboard: "kb1"}

	markers := deriveRefMarkers("alice", b)

	s.Require().Len(markers, 1)
	s.Equal(refMarker{ownerID: "alice", refType: "keyboard", refID: "kb1", buildID: "b1"}, markers[0])
}

func (s *BuildRefSuite) TestDeriveRefMarkers_Switches() {
	b := repository.Build{
		ID:       "b1",
		Keyboard: "kb1",
		Switches: []repository.BuildSwitchEntry{
			{Switch: "sw1", Count: 10},
			{Switch: "sw2", Count: 20},
		},
	}

	markers := deriveRefMarkers("alice", b)

	s.Require().Len(markers, 3)
	s.Contains(markers, refMarker{ownerID: "alice", refType: "switch", refID: "sw1", buildID: "b1"})
	s.Contains(markers, refMarker{ownerID: "alice", refType: "switch", refID: "sw2", buildID: "b1"})
}

func (s *BuildRefSuite) TestDeriveRefMarkers_KeycapKits_UseCompositeRefID() {
	b := repository.Build{
		ID:       "b1",
		Keyboard: "kb1",
		KeycapKits: []repository.BuildKeycapKitEntry{
			{KeycapSet: "ks1", Kit: "kit1"},
		},
	}

	markers := deriveRefMarkers("alice", b)

	s.Require().Len(markers, 2)
	s.Contains(markers, refMarker{ownerID: "alice", refType: "keycap_kit", refID: "ks1#kit1", buildID: "b1"})
}

func (s *BuildRefSuite) TestDeriveRefMarkers_EmptyKeyboard_NoKeyboardMarker() {
	b := repository.Build{ID: "b1"}

	markers := deriveRefMarkers("alice", b)

	s.Empty(markers)
}

func (s *BuildRefSuite) TestDiffRefMarkers_ReturnsAddsAndRemoves() {
	oldMarkers := []refMarker{
		{ownerID: "alice", refType: "keyboard", refID: "kb1", buildID: "b1"},
		{ownerID: "alice", refType: "switch", refID: "sw1", buildID: "b1"},
	}
	newMarkers := []refMarker{
		{ownerID: "alice", refType: "keyboard", refID: "kb1", buildID: "b1"},
		{ownerID: "alice", refType: "switch", refID: "sw2", buildID: "b1"},
	}

	toAdd, toRemove := diffRefMarkers(oldMarkers, newMarkers)

	s.Require().Len(toAdd, 1)
	s.Equal("sw2", toAdd[0].refID)
	s.Require().Len(toRemove, 1)
	s.Equal("sw1", toRemove[0].refID)
}

func (s *BuildRefSuite) TestDiffRefMarkers_NoChanges_ReturnsEmptyBoth() {
	markers := []refMarker{
		{ownerID: "alice", refType: "keyboard", refID: "kb1", buildID: "b1"},
	}

	toAdd, toRemove := diffRefMarkers(markers, markers)

	s.Empty(toAdd)
	s.Empty(toRemove)
}

func (s *BuildRefSuite) TestRefMarkerSortKey_IsSyntheticAndUnique() {
	m := refMarker{ownerID: "alice", refType: "keyboard", refID: "kb1", buildID: "b1"}

	s.Equal("zREF#keyboard#kb1#b1", m.sortKey())
}

func (s *BuildRefSuite) TestPutMarkerTransactItem_HasExpectedKeyAndTable() {
	m := refMarker{ownerID: "alice", refType: "keyboard", refID: "kb1", buildID: "b1"}

	item := putMarkerTransactItem("build-table", m)

	s.Require().NotNil(item.Put)
	s.Equal("build-table", *item.Put.TableName)
	userID, ok := item.Put.Item["user_id"].(*types.AttributeValueMemberS)
	s.Require().True(ok)
	s.Equal("alice", userID.Value)
	id, ok := item.Put.Item["id"].(*types.AttributeValueMemberS)
	s.Require().True(ok)
	s.Equal("zREF#keyboard#kb1#b1", id.Value)
	refID, ok := item.Put.Item["ref_id"].(*types.AttributeValueMemberS)
	s.Require().True(ok)
	s.Equal("kb1", refID.Value)
	buildID, ok := item.Put.Item["build_id"].(*types.AttributeValueMemberS)
	s.Require().True(ok)
	s.Equal("b1", buildID.Value)
}

func (s *BuildRefSuite) TestDeleteMarkerTransactItem_HasExpectedKeyAndTable() {
	m := refMarker{ownerID: "alice", refType: "keyboard", refID: "kb1", buildID: "b1"}

	item := deleteMarkerTransactItem("build-table", m)

	s.Require().NotNil(item.Delete)
	s.Equal("build-table", *item.Delete.TableName)
	userID, ok := item.Delete.Key["user_id"].(*types.AttributeValueMemberS)
	s.Require().True(ok)
	s.Equal("alice", userID.Value)
	id, ok := item.Delete.Key["id"].(*types.AttributeValueMemberS)
	s.Require().True(ok)
	s.Equal("zREF#keyboard#kb1#b1", id.Value)
}

func (s *BuildRefSuite) TestFindBuildsReferencingKeyboard_QueriesGSIByOwnerAndKeyboardID() {
	mockClient := mocks.NewMockDynamoAPI(s.T())
	repo := &BuildRepository{client: mockClient, tableName: "build-table"}

	mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			return *in.TableName == "build-table" && *in.IndexName == "BuildRefIndex" &&
				in.ExpressionAttributeValues[":1"].(*types.AttributeValueMemberS).Value == "kb1"
		})).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{
				{"build_id": &types.AttributeValueMemberS{Value: "b1"}},
				{"build_id": &types.AttributeValueMemberS{Value: "b2"}},
			},
		}, nil)

	buildIDs, err := repo.FindBuildsReferencingKeyboard(s.T().Context(), "alice", "kb1")

	s.Require().NoError(err)
	s.ElementsMatch([]string{"b1", "b2"}, buildIDs)
}

func (s *BuildRefSuite) TestFindBuildsReferencingSwitch_QueriesGSIByOwnerAndSwitchID() {
	mockClient := mocks.NewMockDynamoAPI(s.T())
	repo := &BuildRepository{client: mockClient, tableName: "build-table"}

	mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			return *in.TableName == "build-table" && *in.IndexName == "BuildRefIndex" &&
				in.ExpressionAttributeValues[":1"].(*types.AttributeValueMemberS).Value == "sw1"
		})).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{{"build_id": &types.AttributeValueMemberS{Value: "b1"}}},
		}, nil)

	buildIDs, err := repo.FindBuildsReferencingSwitch(s.T().Context(), "alice", "sw1")

	s.Require().NoError(err)
	s.Equal([]string{"b1"}, buildIDs)
}

func (s *BuildRefSuite) TestFindBuildsReferencingKeycapKit_QueriesGSIByOwnerAndCompositeRefID() {
	mockClient := mocks.NewMockDynamoAPI(s.T())
	repo := &BuildRepository{client: mockClient, tableName: "build-table"}

	mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			return *in.TableName == "build-table" && *in.IndexName == "BuildRefIndex" &&
				in.ExpressionAttributeValues[":1"].(*types.AttributeValueMemberS).Value == "ks1#kit1"
		})).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{{"build_id": &types.AttributeValueMemberS{Value: "b1"}}},
		}, nil)

	buildIDs, err := repo.FindBuildsReferencingKeycapKit(s.T().Context(), "alice", "ks1", "kit1")

	s.Require().NoError(err)
	s.Equal([]string{"b1"}, buildIDs)
}

func (s *BuildRefSuite) TestFindBuildsReferencingKeycapSet_UsesBeginsWithPrefix() {
	mockClient := mocks.NewMockDynamoAPI(s.T())
	repo := &BuildRepository{client: mockClient, tableName: "build-table"}

	mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			// begins_with queries can't also equality-match ref_id, so this
			// must be recognizable as a prefix query, not an exact match.
			return *in.TableName == "build-table" && *in.IndexName == "BuildRefIndex" &&
				in.ExpressionAttributeValues[":1"].(*types.AttributeValueMemberS).Value == "ks1#"
		})).
		Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil)

	buildIDs, err := repo.FindBuildsReferencingKeycapSet(s.T().Context(), "alice", "ks1")

	s.Require().NoError(err)
	s.Empty(buildIDs)
}

func (s *BuildRefSuite) TestDeriveRefMarkers_ECParts() {
	b := repository.Build{
		ID:       "b1",
		Keyboard: "kb1",
		ECParts:  []repository.BuildECPartEntry{{ECPart: "domes", Count: 60}, {ECPart: "sliders", Count: 60}},
	}

	markers := deriveRefMarkers("alice", b)

	s.Require().Len(markers, 3)
	s.Contains(markers, refMarker{ownerID: "alice", refType: "ec_part", refID: "domes", buildID: "b1"})
	s.Contains(markers, refMarker{ownerID: "alice", refType: "ec_part", refID: "sliders", buildID: "b1"})
}

func (s *BuildRefSuite) TestFindBuildsReferencingECPart_QueriesGSIByOwnerAndECPartID() {
	mockClient := mocks.NewMockDynamoAPI(s.T())
	repo := &BuildRepository{client: mockClient, tableName: "build-table"}

	mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			return *in.TableName == "build-table" && *in.IndexName == "BuildRefIndex" &&
				in.ExpressionAttributeValues[":1"].(*types.AttributeValueMemberS).Value == "p1"
		})).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{{"build_id": &types.AttributeValueMemberS{Value: "b1"}}},
		}, nil)

	buildIDs, err := repo.FindBuildsReferencingECPart(s.T().Context(), "alice", "p1")

	s.Require().NoError(err)
	s.Equal([]string{"b1"}, buildIDs)
}

func (s *BuildRefSuite) TestDeriveRefMarkers_RepeatedPartGetsOneMarker() {
	b := repository.Build{
		ID:       "b1",
		Switches: []repository.BuildSwitchEntry{{Switch: "sw1", Count: 30}, {Switch: "sw1", Count: 30}},
		ECParts:  []repository.BuildECPartEntry{{ECPart: "p1", Count: 30}, {ECPart: "p1", Count: 30}},
	}

	s.ElementsMatch([]refMarker{
		{ownerID: "alice", refType: "switch", refID: "sw1", buildID: "b1"},
		{ownerID: "alice", refType: "ec_part", refID: "p1", buildID: "b1"},
	}, deriveRefMarkers("alice", b))
}
