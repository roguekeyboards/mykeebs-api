package profiles_test

import (
	"context"
	"net/http"

	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support/account"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
)

func expectItemsGone(ctx context.Context, items account.Items, ownerID, token string) {
	keyboard, err := api.NewKeyboardsClient().Get(ctx, ownerID, items.KeyboardID, token)
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = keyboard.Body.Close() }()
	Expect(keyboard.StatusCode).To(Equal(http.StatusNotFound), "keyboard")

	sw, err := api.NewSwitchesClient().Get(ctx, ownerID, items.SwitchID, token)
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = sw.Body.Close() }()
	Expect(sw.StatusCode).To(Equal(http.StatusNotFound), "switch")

	part, err := api.NewECPartsClient().Get(ctx, ownerID, items.ECPartID, token)
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = part.Body.Close() }()
	Expect(part.StatusCode).To(Equal(http.StatusNotFound), "EC part")

	set, err := api.NewKeycapSetsClient().Get(ctx, ownerID, items.KeycapSetID, token)
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = set.Body.Close() }()
	Expect(set.StatusCode).To(Equal(http.StatusNotFound), "keycap set")

	build, err := api.NewBuildsClient().Get(ctx, ownerID, items.BuildID, token)
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = build.Body.Close() }()
	Expect(build.StatusCode).To(Equal(http.StatusNotFound), "build")
}
