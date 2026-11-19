package db

import (
	"context"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support"
)

// SeedECPart PutItems a domes EC part directly into DynamoDB, bypassing
// the API, for specs that need fixture data in place first.
func SeedECPart(ctx context.Context, ownerID, id, visibility string) error {
	return SeedECPartOfType(ctx, ownerID, id, visibility, "domes")
}

// SeedECPartOfType is [SeedECPart] with the given part_type and no
// type-specific details.
func SeedECPartOfType(ctx context.Context, ownerID, id, visibility, partType string) error {
	table := NewDynamoTable(ctx, support.ECPartTableName())
	return table.PutItem(ctx, map[string]any{
		"user_id":    ownerID,
		"id":         id,
		"brand":      "DynaCap",
		"name":       "Heavy",
		"part_type":  partType,
		"visibility": visibility,
		"purchase": map[string]any{
			"vendor":       "Omnitype",
			"price":        36.0,
			"quantity":     72,
			"order_status": "Delivered",
		},
	})
}

// DeleteECPart removes an EC part directly from DynamoDB.
func DeleteECPart(ctx context.Context, ownerID, id string) error {
	table := NewDynamoTable(ctx, support.ECPartTableName())
	return table.DeleteItem(ctx, map[string]string{"user_id": ownerID, "id": id})
}

// ECPartImage is an EC part's stored image slot and its moderation status.
type ECPartImage struct {
	Path   string `dynamodbav:"image_path"`
	Status string `dynamodbav:"image_status"`
}

// GetECPartImage reads an EC part's image slot straight from DynamoDB, for
// specs waiting on the moderation function.
func GetECPartImage(ctx context.Context, ownerID, id string) (ECPartImage, error) {
	var img ECPartImage
	_, err := NewDynamoTable(ctx, support.ECPartTableName()).
		GetItem(ctx, map[string]string{"user_id": ownerID, "id": id}, &img)
	return img, err
}
