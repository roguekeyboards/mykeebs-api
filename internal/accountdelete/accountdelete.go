// Package accountdelete deletes a user's whole account: every image they
// own, every build, keyboard, switch, EC part and keycap set, then their
// profile.
// mykeebs-api has no account apart from the profile, so the REST profile delete
// and MCP's delete_profile both call [Account.Delete].
package accountdelete

import (
	"context"
	"fmt"

	apictx "github.com/roguekeyboards/mykeebs-api/internal/ctx"
	"github.com/roguekeyboards/mykeebs-api/internal/log"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// ItemStore is one table of the caller's items.
type ItemStore interface {
	DeleteAll(ctx context.Context) error
}

// Notifier tells the operator an account was deleted, so they can remove
// the user from the identity provider by hand.
type Notifier interface {
	Notify(ctx context.Context, userID string) error
}

// NoticeFailed is logged when a notice can't be sent. template.yaml's
// AccountDeleteNoticeFailedFilter matches it, so keep the two in step.
const NoticeFailed = "account delete notice failed; remove the user from the identity provider by hand"

// Account holds what [Account.Delete] deletes from.
type Account struct {
	Images     repository.OwnerImageStore
	Builds     ItemStore
	Keyboards  ItemStore
	Switches   ItemStore
	ECParts    ItemStore
	KeycapSets ItemStore
	Profiles   repository.ProfileRepository
	// Notices is optional: nil sends none (deployments without an operator
	// address).
	Notices Notifier
}

// Delete deletes everything the caller on ctx owns: the item tables, then
// every image by key prefix (after the tables, so an image added while
// they're deleted is still swept), then the profile. Every step is
// idempotent, so a call that fails or times out partway can be retried to
// finish the rest. Once everything is gone it sends the operator a notice;
// a notice that fails is logged as [NoticeFailed], not returned, since the
// delete itself succeeded.
// Errors wrap the repository's, so callers can still match
// [repository.ErrMutationConflict].
func (a Account) Delete(ctx context.Context) error {
	for _, items := range []struct {
		name  string
		store ItemStore
	}{
		{"builds", a.Builds},
		{"keyboards", a.Keyboards},
		{"switches", a.Switches},
		{"EC parts", a.ECParts},
		{"keycap sets", a.KeycapSets},
	} {
		if err := items.store.DeleteAll(ctx); err != nil {
			return fmt.Errorf("deleting %s: %w", items.name, err)
		}
	}
	if err := a.Images.DeleteAll(ctx); err != nil {
		return fmt.Errorf("deleting images: %w", err)
	}
	if err := a.Profiles.Delete(ctx); err != nil {
		return fmt.Errorf("deleting profile: %w", err)
	}
	if a.Notices != nil {
		userID, _ := apictx.UserID(ctx)
		// The data is already gone; a caller hanging up now mustn't cancel
		// the notice.
		if err := a.Notices.Notify(context.WithoutCancel(ctx), userID); err != nil {
			log.FromContext(ctx).Error(NoticeFailed, log.OwnerID, userID, log.Error, err)
		}
	}
	return nil
}
