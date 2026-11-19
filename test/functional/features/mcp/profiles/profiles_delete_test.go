package profiles_test

import (
	"time"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/account"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/notices"
)

var _ = Describe("Deleting a profile over MCP", func() {
	var (
		client   *api.MCPClient
		result   *sdkmcp.CallToolResult
		err      error
		ownerID  string
		username string
	)

	BeforeEach(func(ctx SpecContext) {
		result = nil
		err = nil
		username = "u" + uuid.NewString()[:8]

		client, ownerID = api.NewAuthenticatedMCPClient(ctx)
	})

	Context("given the caller has a profile", func() {
		BeforeEach(func(ctx SpecContext) {
			Expect(db.SeedProfile(ctx, ownerID, db.SeedProfileOptions{
				Username:     username,
				Discoverable: true,
			})).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			_ = db.DeleteProfile(ctx, ownerID, username)
		})

		When("delete_profile is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "delete_profile", map[string]any{})
			})

			It("succeeds, the profile is gone, and the username is free again", func(ctx SpecContext) {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeFalse())

				By("no longer resolving the profile")
				got, getErr := client.CallTool(ctx, "get_profile", map[string]any{"identifier": ownerID})
				Expect(getErr).NotTo(HaveOccurred())
				Expect(got.IsError).To(BeTrue())

				By("letting another user claim the freed username")
				otherToken, otherID, subErr := api.NewAuthIdentity(ctx)
				Expect(subErr).NotTo(HaveOccurred())
				otherClient := api.NewMCPClient(support.BaseURL()+"/mcp", otherToken)

				created, createErr := otherClient.CallTool(ctx, "create_profile", map[string]any{
					"username": username, "discoverable": true,
				})
				Expect(createErr).NotTo(HaveOccurred())
				Expect(created.IsError).To(BeFalse())

				DeferCleanup(func(ctx SpecContext) {
					Expect(db.DeleteProfile(ctx, otherID, username)).To(Succeed())
				})
			})
		})
	})

	Context("given the caller has a profile and owns one of each item", func() {
		var (
			items    account.Items
			listener *notices.Listener
		)

		BeforeEach(func(ctx SpecContext) {
			var listenErr error
			listener, listenErr = notices.Listen(ctx)
			DeferCleanup(func(ctx SpecContext) { listener.Close(ctx) })
			Expect(listenErr).NotTo(HaveOccurred())
			Expect(db.SeedProfile(ctx, ownerID, db.SeedProfileOptions{
				Username:     username,
				Discoverable: true,
			})).To(Succeed())
			var seedErr error
			items, seedErr = account.Seed(ctx, ownerID)
			Expect(seedErr).NotTo(HaveOccurred())
		})

		AfterEach(func(ctx SpecContext) {
			_ = db.DeleteProfile(ctx, ownerID, username)
			items.CleanUp(ctx, ownerID)
		})

		When("delete_profile is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "delete_profile", map[string]any{})
			})

			It("succeeds and deletes every item and image", func(ctx SpecContext) {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeFalse())

				for tool, args := range map[string]map[string]any{
					"get_keyboard":   {"keyboard_id": items.KeyboardID},
					"get_switch":     {"switch_id": items.SwitchID},
					"get_ec_part":    {"ec_part_id": items.ECPartID},
					"get_keycap_set": {"keycap_set_id": items.KeycapSetID},
					"get_build":      {"build_id": items.BuildID},
				} {
					got, getErr := client.CallTool(ctx, tool, args)
					Expect(getErr).NotTo(HaveOccurred())
					Expect(got.IsError).To(BeTrue(), tool)
				}
				Expect(items.RemainingStrayImages(ctx)).To(BeEmpty())
				Eventually(func(ctx SpecContext) (bool, error) {
					return listener.Received(ctx, ownerID)
				}).WithContext(ctx).WithTimeout(30 * time.Second).Should(BeTrue())
			})
		})
	})

	Context("given the caller has no profile", func() {
		When("delete_profile is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "delete_profile", map[string]any{})
			})

			It("succeeds idempotently", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeFalse())
			})
		})
	})

	Context("given no bearer token", func() {
		BeforeEach(func() {
			client = api.NewMCPClient(support.BaseURL()+"/mcp", "")
		})

		When("delete_profile is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "delete_profile", map[string]any{})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
