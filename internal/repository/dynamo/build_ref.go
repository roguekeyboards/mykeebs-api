package dynamo

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// buildRefIndexName is BuildRefIndex, the GSI (user_id HASH / ref_id RANGE)
// that answers "which builds reference entity X" via the refMarker items
// this file writes alongside real Build items. See template.yaml's
// BuildTable resource.
const buildRefIndexName = "BuildRefIndex"

// refMarker is one (build, referenced-entity) pointer, written into
// BuildTable alongside the real Build item so BuildRefIndex can answer
// "which builds reference entity X" without a table Scan. See
// [deriveRefMarkers] for how a Build's reference fields become markers.
type refMarker struct {
	ownerID string
	refType repository.RefType
	refID   string
	buildID string
}

// sortKey is the marker item's "id" attribute - synthetic, never a real
// build id, so it can never collide with a real Build item in the same
// table. The "z" prefix is deliberate: it sorts after every character a
// build id (a lowercase hex UUIDv4) can contain, so a user_id partition
// Query in default ascending order scans every real Build before it reaches
// any marker - List's key condition stops at refMarkerPrefix, so it never
// reads a marker.
func (m refMarker) sortKey() string {
	return fmt.Sprintf("%s%s#%s#%s", refMarkerPrefix, m.refType, m.refID, m.buildID)
}

const refMarkerPrefix = "zREF#"

// deriveRefMarkers computes the full set of refMarkers for b's current
// Keyboard/Switches/ECParts/KeycapKits. A KeycapKit's refID is the composite
// "<KeycapSetID>#<KitID>", so a whole-KeycapSet delete can find every
// referencing build via a begins_with(ref_id, "<SetID>#") query without a
// separate set-level marker - a Build never references a bare KeycapSet
// without a specific kit. Duplicate markers are collapsed, since one
// transaction can't write the same item twice.
func deriveRefMarkers(ownerID string, b repository.Build) []refMarker {
	var markers []refMarker

	if b.Keyboard != "" {
		markers = append(markers, refMarker{ownerID: ownerID, refType: repository.RefTypeKeyboard, refID: b.Keyboard, buildID: b.ID})
	}

	for _, entry := range b.Switches {
		markers = append(markers, refMarker{ownerID: ownerID, refType: repository.RefTypeSwitch, refID: entry.Switch, buildID: b.ID})
	}

	for _, entry := range b.ECParts {
		markers = append(markers, refMarker{ownerID: ownerID, refType: repository.RefTypeECPart, refID: entry.ECPart, buildID: b.ID})
	}

	for _, entry := range b.KeycapKits {
		refID := fmt.Sprintf("%s#%s", entry.KeycapSet, entry.Kit)
		markers = append(markers, refMarker{ownerID: ownerID, refType: repository.RefTypeKeycapKit, refID: refID, buildID: b.ID})
	}

	slices.SortFunc(markers, func(a, b refMarker) int { return strings.Compare(a.sortKey(), b.sortKey()) })
	return slices.Compact(markers)
}

// diffRefMarkers returns the markers present in newMarkers but not
// oldMarkers (toAdd), and the markers present in oldMarkers but not
// newMarkers (toRemove). Used by Update to write only the reference deltas
// rather than deleting and recreating every marker on every write.
func diffRefMarkers(oldMarkers, newMarkers []refMarker) (toAdd, toRemove []refMarker) {
	oldSet := make(map[refMarker]bool, len(oldMarkers))
	for _, m := range oldMarkers {
		oldSet[m] = true
	}
	newSet := make(map[refMarker]bool, len(newMarkers))
	for _, m := range newMarkers {
		newSet[m] = true
	}

	for _, m := range newMarkers {
		if !oldSet[m] {
			toAdd = append(toAdd, m)
		}
	}
	for _, m := range oldMarkers {
		if !newSet[m] {
			toRemove = append(toRemove, m)
		}
	}

	return toAdd, toRemove
}

// putMarkerTransactItem builds the TransactWriteItem that creates m in
// tableName, for inclusion alongside the base Build item's own write in one
// TransactWriteItems call.
func putMarkerTransactItem(tableName string, m refMarker) types.TransactWriteItem {
	return types.TransactWriteItem{
		Put: &types.Put{
			TableName: aws.String(tableName),
			Item: map[string]types.AttributeValue{
				"user_id":   &types.AttributeValueMemberS{Value: m.ownerID},
				"id":        &types.AttributeValueMemberS{Value: m.sortKey()},
				"item_type": &types.AttributeValueMemberS{Value: "build_ref_marker"},
				"ref_type":  &types.AttributeValueMemberS{Value: string(m.refType)},
				"ref_id":    &types.AttributeValueMemberS{Value: m.refID},
				"build_id":  &types.AttributeValueMemberS{Value: m.buildID},
			},
		},
	}
}

// deleteMarkerTransactItem builds the TransactWriteItem that removes m from
// tableName.
func deleteMarkerTransactItem(tableName string, m refMarker) types.TransactWriteItem {
	return types.TransactWriteItem{
		Delete: &types.Delete{
			TableName: aws.String(tableName),
			Key: map[string]types.AttributeValue{
				"user_id": &types.AttributeValueMemberS{Value: m.ownerID},
				"id":      &types.AttributeValueMemberS{Value: m.sortKey()},
			},
		},
	}
}

// FindBuildsReferencingKeyboard implements repository.BuildRepository.
func (r *BuildRepository) FindBuildsReferencingKeyboard(ctx context.Context, ownerID, keyboardID string) ([]string, error) {
	return r.findReferencingBuilds(ctx, ownerID, repository.RefTypeKeyboard, keyboardID)
}

// FindBuildsReferencingSwitch implements repository.BuildRepository.
func (r *BuildRepository) FindBuildsReferencingSwitch(ctx context.Context, ownerID, switchID string) ([]string, error) {
	return r.findReferencingBuilds(ctx, ownerID, repository.RefTypeSwitch, switchID)
}

// FindBuildsReferencingECPart implements repository.BuildRepository.
func (r *BuildRepository) FindBuildsReferencingECPart(ctx context.Context, ownerID, ecPartID string) ([]string, error) {
	return r.findReferencingBuilds(ctx, ownerID, repository.RefTypeECPart, ecPartID)
}

// FindBuildsReferencingKeycapKit implements repository.BuildRepository. The
// composite "<KeycapSetID>#<KitID>" ref_id format is a dynamo-package
// implementation detail - callers pass the two ids separately.
func (r *BuildRepository) FindBuildsReferencingKeycapKit(ctx context.Context, ownerID, keycapSetID, kitID string) ([]string, error) {
	refID := fmt.Sprintf("%s#%s", keycapSetID, kitID)
	return r.findReferencingBuilds(ctx, ownerID, repository.RefTypeKeycapKit, refID)
}

// FindBuildsReferencingKeycapSet implements repository.BuildRepository. It
// matches every kit in keycapSetID via a begins_with prefix query - a Build
// never references a bare KeycapSet without a specific kit, so there's no
// separate set-level marker to look up directly (see deriveRefMarkers).
func (r *BuildRepository) FindBuildsReferencingKeycapSet(ctx context.Context, ownerID, keycapSetID string) ([]string, error) {
	prefix := keycapSetID + "#"
	expr, err := expression.NewBuilder().
		WithKeyCondition(expression.Key("user_id").Equal(expression.Value(ownerID)).
			And(expression.Key("ref_id").BeginsWith(prefix))).
		Build()
	if err != nil {
		return nil, fmt.Errorf("building reverse-lookup prefix expression for keycap set %q: %w", keycapSetID, err)
	}

	return r.queryReferencingBuilds(ctx, repository.RefTypeKeycapKit, prefix, expr)
}

// findReferencingBuilds is the shared exact-match implementation behind the
// three FindBuildsReferencingX methods that look up by a single ref_id
// (KeycapSet is the exception - it prefix-matches, see
// FindBuildsReferencingKeycapSet).
func (r *BuildRepository) findReferencingBuilds(ctx context.Context, ownerID string, refType repository.RefType, refID string) ([]string, error) {
	expr, err := expression.NewBuilder().
		WithKeyCondition(expression.Key("user_id").Equal(expression.Value(ownerID)).
			And(expression.Key("ref_id").Equal(expression.Value(refID)))).
		Build()
	if err != nil {
		return nil, fmt.Errorf("building reverse-lookup expression for %s %q: %w", refType, refID, err)
	}

	return r.queryReferencingBuilds(ctx, refType, refID, expr)
}

func (r *BuildRepository) queryReferencingBuilds(ctx context.Context, refType repository.RefType, refID string, expr expression.Expression) ([]string, error) {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:                 &r.tableName,
		IndexName:                 aws.String(buildRefIndexName),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	})
	if err != nil {
		return nil, fmt.Errorf("querying %s for referencing builds of %s %q: %w", buildRefIndexName, refType, refID, err)
	}

	buildIDs := make([]string, 0, len(out.Items))
	for _, item := range out.Items {
		buildID, ok := item["build_id"].(*types.AttributeValueMemberS)
		if !ok {
			continue
		}
		buildIDs = append(buildIDs, buildID.Value)
	}

	return buildIDs, nil
}
