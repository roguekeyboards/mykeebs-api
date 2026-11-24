package db

import (
	"context"
	"fmt"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support"
)

// SeedSwitch PutItems a switch directly into DynamoDB, bypassing the API -
// for specs that need switch fixture data in place before exercising a
// different route. version is set to 0, matching what Create would have
// set it to - Update/SetImagePath/ClearImagePath condition their write on
// version via a CAS loop, which fails on a fixture item missing the
// attribute entirely (attribute_not_exists != 0), not just a genuine
// version mismatch.
func SeedSwitch(ctx context.Context, ownerID, id, visibility string) error {
	table := NewDynamoTable(ctx, support.SwitchTableName())
	return table.PutItem(ctx, map[string]any{
		"user_id":    ownerID,
		"id":         id,
		"brand":      "Gateron",
		"name":       "Yellow",
		"type":       "Linear",
		"visibility": visibility,
		"version":    0,
		"purchases": []map[string]any{{
			"vendor":       "NovelKeys",
			"price":        0.35,
			"order_status": "Delivered",
		}},
	})
}

// SeedSwitchWithImage is [SeedSwitch] plus an ImagePath with no S3 object
// behind it, so the switch gets an image URL that can't be fetched.
func SeedSwitchWithImage(ctx context.Context, ownerID, id, visibility string) error {
	table := NewDynamoTable(ctx, support.SwitchTableName())
	return table.PutItem(ctx, switchWithImage(ownerID, id, visibility))
}

// SeedSwitchWithModeratedImage is [SeedSwitchWithImage] with the image's
// moderation status already written. An empty reason is left out.
func SeedSwitchWithModeratedImage(ctx context.Context, ownerID, id, visibility, status, reason string) error {
	item := switchWithImage(ownerID, id, visibility)
	item["image_status"] = status
	if reason != "" {
		item["image_rejection_reason"] = reason
	}
	table := NewDynamoTable(ctx, support.SwitchTableName())
	return table.PutItem(ctx, item)
}

func switchWithImage(ownerID, id, visibility string) map[string]any {
	return map[string]any{
		"user_id":    ownerID,
		"id":         id,
		"brand":      "Gateron",
		"name":       "Yellow",
		"type":       "Linear",
		"visibility": visibility,
		"version":    0,
		"purchases": []map[string]any{{
			"vendor": "NovelKeys",
			"price":  0.35,
		}},
		"image_path": fmt.Sprintf("switches/%s/%s/image/up1", ownerID, id),
	}
}

// DeleteSwitch removes a switch seeded by SeedSwitch, or one created via the
// API during a spec.
func DeleteSwitch(ctx context.Context, ownerID, id string) error {
	table := NewDynamoTable(ctx, support.SwitchTableName())
	return table.DeleteItem(ctx, map[string]string{"user_id": ownerID, "id": id})
}

// SwitchImage is a switch's stored image slot and its moderation status.
type SwitchImage struct {
	Path            string `dynamodbav:"image_path"`
	Status          string `dynamodbav:"image_status"`
	RejectionReason string `dynamodbav:"image_rejection_reason"`
}

// GetSwitchImage reads a switch's image slot straight from DynamoDB, for
// specs waiting on the moderation function.
func GetSwitchImage(ctx context.Context, ownerID, id string) (SwitchImage, error) {
	var img SwitchImage
	_, err := NewDynamoTable(ctx, support.SwitchTableName()).
		GetItem(ctx, map[string]string{"user_id": ownerID, "id": id}, &img)
	return img, err
}
