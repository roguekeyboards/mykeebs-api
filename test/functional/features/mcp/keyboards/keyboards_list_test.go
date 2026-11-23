package keyboards_test

import (
	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("Listing keyboards over MCP", func() {
	var (
		client  *api.MCPClient
		result  *sdkmcp.CallToolResult
		err     error
		ownerID string
	)

	BeforeEach(func() {
		result = nil
		err = nil
	})

	Context("given a valid bearer token", func() {
		BeforeEach(func(ctx SpecContext) {
			client, ownerID = api.NewAuthenticatedMCPClient(ctx)
		})

		Context("given the caller owns a private keyboard", func() {
			var keyboardID string

			BeforeEach(func(ctx SpecContext) {
				keyboardID = "functional-test-keyboard-" + uuid.NewString()
				Expect(db.SeedKeyboard(ctx, ownerID, keyboardID, "private")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeyboard(ctx, ownerID, keyboardID)).To(Succeed())
			})

			When("the list_keyboards tool is called with no user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_keyboards", map[string]any{})
				})

				It("defaults to the caller's own collection and includes the private keyboard", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeListOutput(result)
					Expect(idsOf(out)).To(ContainElement(keyboardID))

					By("carrying order_status, which the summary lifts out of purchase")
					seeded := seededBy(out, keyboardID)
					Expect(seeded).NotTo(BeNil())
					Expect(seeded.OrderStatus).NotTo(BeNil())
					Expect(*seeded.OrderStatus).To(Equal("Delivered"))

					By("including the total cost, since the caller owns this keyboard")
					Expect(seeded.TotalCost).NotTo(BeNil())
					Expect(*seeded.TotalCost).To(BeNumerically("==", 369.99))

					By("including the visibility, since the caller owns this keyboard")
					Expect(seeded.Visibility).NotTo(BeNil())
					Expect(*seeded.Visibility).To(Equal("private"))
				})
			})

			DescribeTable("given an out-of-range limit",
				func(ctx SpecContext, limit int) {
					result, err = client.CallTool(ctx, "list_keyboards", map[string]any{"limit": limit})
					Expect(err).NotTo(HaveOccurred())

					By("rejecting the call, as REST does")
					Expect(result.IsError).To(BeTrue())
					Expect(result.Content[0].(*sdkmcp.TextContent).Text).To(ContainSubstring("limit"))
				},
				Entry("below the minimum", 0),
				Entry("above the maximum", 101),
			)
		})

		Context("given the caller owns a private keyboard and has show_price_to_me false", func() {
			var (
				keyboardID      string
				profileUsername string
			)

			BeforeEach(func(ctx SpecContext) {
				keyboardID = "functional-test-keyboard-" + uuid.NewString()
				profileUsername = "u" + uuid.NewString()[:8]
				Expect(db.SeedKeyboard(ctx, ownerID, keyboardID, "private")).To(Succeed())
				Expect(db.SeedProfile(ctx, ownerID, db.SeedProfileOptions{
					Username: profileUsername,
					Preferences: map[string]any{
						"currency": "USD", "show_price_to_me": false, "show_price_to_others": false,
					},
				})).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeyboard(ctx, ownerID, keyboardID)).To(Succeed())
				Expect(db.DeleteProfile(ctx, ownerID, profileUsername)).To(Succeed())
			})

			When("the list_keyboards tool is called with no user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_keyboards", map[string]any{})
				})

				It("omits the total cost", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeListOutput(result)
					seeded := seededBy(out, keyboardID)
					Expect(seeded).NotTo(BeNil())
					Expect(seeded.TotalCost).To(BeNil())
				})
			})
		})

		Context("given another user owns a private keyboard", func() {
			var (
				otherID         string
				otherKeyboardID string
			)

			BeforeEach(func(ctx SpecContext) {
				otherID = api.NewOtherUserID(ctx)

				otherKeyboardID = "functional-test-keyboard-" + uuid.NewString()
				Expect(db.SeedKeyboard(ctx, otherID, otherKeyboardID, "private")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeyboard(ctx, otherID, otherKeyboardID)).To(Succeed())
			})

			When("the list_keyboards tool is called with that user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_keyboards", map[string]any{"user_id": otherID})
				})

				It("omits the other user's private keyboard", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())
					Expect(idsOf(decodeListOutput(result))).NotTo(ContainElement(otherKeyboardID))
				})
			})
		})

		Context("given another user owns a public keyboard", func() {
			var (
				otherID         string
				otherKeyboardID string
			)

			BeforeEach(func(ctx SpecContext) {
				otherID = api.NewOtherUserID(ctx)

				otherKeyboardID = "functional-test-keyboard-" + uuid.NewString()
				Expect(db.SeedKeyboard(ctx, otherID, otherKeyboardID, "public")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeyboard(ctx, otherID, otherKeyboardID)).To(Succeed())
			})

			When("the list_keyboards tool is called with that user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_keyboards", map[string]any{"user_id": otherID})
				})

				It("includes the other user's public keyboard, without its total cost or visibility", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeListOutput(result)
					Expect(idsOf(out)).To(ContainElement(otherKeyboardID))

					By("omitting the total cost, since the caller doesn't own this keyboard")
					seeded := seededBy(out, otherKeyboardID)
					Expect(seeded).NotTo(BeNil())
					Expect(seeded.TotalCost).To(BeNil())

					By("omitting the visibility, since the caller doesn't own this keyboard")
					Expect(seeded.Visibility).To(BeNil())
				})
			})
		})

		Context("given another user owns a public keyboard and has show_price_to_others true", func() {
			var (
				otherID         string
				otherKeyboardID string
				profileUsername string
			)

			BeforeEach(func(ctx SpecContext) {
				otherID = api.NewOtherUserID(ctx)

				otherKeyboardID = "functional-test-keyboard-" + uuid.NewString()
				profileUsername = "u" + uuid.NewString()[:8]
				Expect(db.SeedKeyboard(ctx, otherID, otherKeyboardID, "public")).To(Succeed())
				Expect(db.SeedProfile(ctx, otherID, db.SeedProfileOptions{
					Username: profileUsername,
					Preferences: map[string]any{
						"currency": "USD", "show_price_to_me": true, "show_price_to_others": true,
					},
				})).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeyboard(ctx, otherID, otherKeyboardID)).To(Succeed())
				Expect(db.DeleteProfile(ctx, otherID, profileUsername)).To(Succeed())
			})

			When("the list_keyboards tool is called with that user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_keyboards", map[string]any{"user_id": otherID})
				})

				It("includes the other user's public keyboard with its total cost", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeListOutput(result)
					seeded := seededBy(out, otherKeyboardID)
					Expect(seeded).NotTo(BeNil())
					Expect(seeded.TotalCost).NotTo(BeNil())
					Expect(*seeded.TotalCost).To(BeNumerically("==", 369.99))
				})
			})
		})
	})

	Context("given no bearer token", func() {
		BeforeEach(func() {
			client = api.NewMCPClient(support.BaseURL()+"/mcp", "")
		})

		When("the list_keyboards tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "list_keyboards", map[string]any{})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
