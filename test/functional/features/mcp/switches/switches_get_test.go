package switches_test

import (
	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("Getting a switch over MCP", func() {
	var (
		client   *api.MCPClient
		result   *sdkmcp.CallToolResult
		err      error
		ownerID  string
		switchID string
	)

	BeforeEach(func() {
		result = nil
		err = nil
		switchID = "functional-test-switch-" + uuid.NewString()
	})

	Context("given a valid bearer token", func() {
		BeforeEach(func(ctx SpecContext) {
			client, ownerID = api.NewAuthenticatedMCPClient(ctx)
		})

		Context("given the caller owns the switch", func() {
			BeforeEach(func(ctx SpecContext) {
				Expect(db.SeedSwitch(ctx, ownerID, switchID, "private")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteSwitch(ctx, ownerID, switchID)).To(Succeed())
			})

			When("the get_switch tool is called with no user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "get_switch", map[string]any{"switch_id": switchID})
				})

				It("defaults to the caller's own collection and returns the switch", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeGetOutput(result)
					Expect(out.Switch.ID).To(Equal(switchID))
					Expect(out.Switch.Brand).To(Equal("Gateron"))
					Expect(out.Switch.Visibility).To(Equal("private"))

					By("including the purchase price for the owner")
					Expect(out.Switch.Purchases).To(HaveLen(1))
					Expect(out.Switch.Purchases[0].Price).NotTo(BeNil())
					Expect(*out.Switch.Purchases[0].Price).To(Equal(0.35))
				})
			})
		})

		Context("given the caller owns the switch and has show_price_to_me false", func() {
			var profileUsername string

			BeforeEach(func(ctx SpecContext) {
				profileUsername = "u" + uuid.NewString()[:8]
				Expect(db.SeedSwitch(ctx, ownerID, switchID, "private")).To(Succeed())
				Expect(db.SeedProfile(ctx, ownerID, db.SeedProfileOptions{
					Username: profileUsername,
					Preferences: map[string]any{
						"currency": "USD", "show_price_to_me": false, "show_price_to_others": false,
					},
				})).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteSwitch(ctx, ownerID, switchID)).To(Succeed())
				Expect(db.DeleteProfile(ctx, ownerID, profileUsername)).To(Succeed())
			})

			When("the get_switch tool is called with no user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "get_switch", map[string]any{"switch_id": switchID})
				})

				It("still returns the purchase price - single-item get always shows the owner their own price", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeGetOutput(result)
					Expect(out.Switch.Purchases).To(HaveLen(1))
					Expect(out.Switch.Purchases[0].Price).NotTo(BeNil())
					Expect(*out.Switch.Purchases[0].Price).To(Equal(0.35))
				})
			})
		})

		Context("given the switch never existed", func() {
			When("the get_switch tool is called with that id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "get_switch", map[string]any{"switch_id": switchID})
				})

				It("returns an MCP tool error result, not a transport failure", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})

		Context("given another user owns a private switch", func() {
			var otherID string

			BeforeEach(func(ctx SpecContext) {
				otherID = api.NewOtherUserID(ctx)

				Expect(db.SeedSwitch(ctx, otherID, switchID, "private")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteSwitch(ctx, otherID, switchID)).To(Succeed())
			})

			When("the get_switch tool is called with that user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "get_switch", map[string]any{
						"switch_id": switchID,
						"user_id":   otherID,
					})
				})

				It("is indistinguishable from the switch not existing", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})

		Context("given another user owns a public switch", func() {
			var otherID string

			BeforeEach(func(ctx SpecContext) {
				otherID = api.NewOtherUserID(ctx)

				Expect(db.SeedSwitch(ctx, otherID, switchID, "public")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteSwitch(ctx, otherID, switchID)).To(Succeed())
			})

			When("the get_switch tool is called with that user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "get_switch", map[string]any{
						"switch_id": switchID,
						"user_id":   otherID,
					})
				})

				It("returns the switch with the purchase price omitted", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeGetOutput(result)
					Expect(out.Switch.ID).To(Equal(switchID))

					By("still including non-price purchase fields")
					Expect(out.Switch.Purchases).To(HaveLen(1))
					Expect(*out.Switch.Purchases[0].Vendor).To(Equal("NovelKeys"))

					By("omitting price")
					Expect(out.Switch.Purchases[0].Price).To(BeNil())
				})
			})
		})

		Context("given another user owns a public switch and has show_price_to_others true", func() {
			var (
				otherID         string
				profileUsername string
			)

			BeforeEach(func(ctx SpecContext) {
				otherID = api.NewOtherUserID(ctx)
				profileUsername = "u" + uuid.NewString()[:8]

				Expect(db.SeedSwitch(ctx, otherID, switchID, "public")).To(Succeed())
				Expect(db.SeedProfile(ctx, otherID, db.SeedProfileOptions{
					Username: profileUsername,
					Preferences: map[string]any{
						"currency": "USD", "show_price_to_me": true, "show_price_to_others": true,
					},
				})).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteSwitch(ctx, otherID, switchID)).To(Succeed())
				Expect(db.DeleteProfile(ctx, otherID, profileUsername)).To(Succeed())
			})

			When("the get_switch tool is called with that user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "get_switch", map[string]any{
						"switch_id": switchID,
						"user_id":   otherID,
					})
				})

				It("returns the switch with the purchase price", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeGetOutput(result)
					Expect(out.Switch.Purchases).To(HaveLen(1))
					Expect(out.Switch.Purchases[0].Price).NotTo(BeNil())
					Expect(*out.Switch.Purchases[0].Price).To(Equal(0.35))
				})
			})
		})
	})

	Context("given no bearer token", func() {
		BeforeEach(func() {
			client = api.NewMCPClient(support.BaseURL()+"/mcp", "")
		})

		When("the get_switch tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "get_switch", map[string]any{"switch_id": switchID})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
