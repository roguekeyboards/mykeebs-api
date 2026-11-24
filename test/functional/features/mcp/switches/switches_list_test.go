package switches_test

import (
	"encoding/json"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

type listSwitch struct {
	ID          string   `json:"id"`
	Brand       string   `json:"brand"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	OrderStatus *string  `json:"order_status"`
	Price       *float64 `json:"price"`
}

type listOutput struct {
	Switches   []listSwitch `json:"switches"`
	NextCursor string       `json:"next_cursor"`
}

func decodeListOutput(result *sdkmcp.CallToolResult) listOutput {
	GinkgoHelper()

	raw, err := json.Marshal(result.StructuredContent)
	Expect(err).NotTo(HaveOccurred())

	var out listOutput
	Expect(json.Unmarshal(raw, &out)).To(Succeed())

	return out
}

func idsOf(out listOutput) []string {
	ids := make([]string, 0, len(out.Switches))
	for _, sw := range out.Switches {
		ids = append(ids, sw.ID)
	}

	return ids
}

func seededBy(out listOutput, id string) *listSwitch {
	for i := range out.Switches {
		if out.Switches[i].ID == id {
			return &out.Switches[i]
		}
	}

	return nil
}

var _ = Describe("Listing switches over MCP", func() {
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

		Context("given the caller owns a private switch", func() {
			var switchID string

			BeforeEach(func(ctx SpecContext) {
				switchID = "functional-test-switch-" + uuid.NewString()
				Expect(db.SeedSwitch(ctx, ownerID, switchID, "private")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteSwitch(ctx, ownerID, switchID)).To(Succeed())
			})

			When("the list_switches tool is called with no user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_switches", map[string]any{})
				})

				It("defaults to the caller's own collection and includes the private switch", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeListOutput(result)
					Expect(idsOf(out)).To(ContainElement(switchID))

					By("carrying order_status, which the summary lifts out of purchases")
					seeded := seededBy(out, switchID)
					Expect(seeded).NotTo(BeNil())
					Expect(seeded.OrderStatus).NotTo(BeNil())
					Expect(*seeded.OrderStatus).To(Equal("Delivered"))

					By("including the price, since the caller owns this switch")
					Expect(seeded.Price).NotTo(BeNil())
					Expect(*seeded.Price).To(BeNumerically("==", 0.35))
				})
			})

			DescribeTable("given an out-of-range limit",
				func(ctx SpecContext, limit int) {
					result, err = client.CallTool(ctx, "list_switches", map[string]any{"limit": limit})
					Expect(err).NotTo(HaveOccurred())

					By("rejecting the call, as REST does")
					Expect(result.IsError).To(BeTrue())
					Expect(result.Content[0].(*sdkmcp.TextContent).Text).To(ContainSubstring("limit"))
				},
				Entry("below the minimum", 0),
				Entry("above the maximum", 101),
			)
		})

		Context("given the caller owns a private switch and has show_price_to_me false", func() {
			var (
				switchID        string
				profileUsername string
			)

			BeforeEach(func(ctx SpecContext) {
				switchID = "functional-test-switch-" + uuid.NewString()
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

			When("the list_switches tool is called with no user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_switches", map[string]any{})
				})

				It("omits the price", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeListOutput(result)
					seeded := seededBy(out, switchID)
					Expect(seeded).NotTo(BeNil())
					Expect(seeded.Price).To(BeNil())
				})
			})
		})

		Context("given another user owns a private switch", func() {
			var (
				otherID       string
				otherSwitchID string
			)

			BeforeEach(func(ctx SpecContext) {
				otherID = api.NewOtherUserID(ctx)

				otherSwitchID = "functional-test-switch-" + uuid.NewString()
				Expect(db.SeedSwitch(ctx, otherID, otherSwitchID, "private")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteSwitch(ctx, otherID, otherSwitchID)).To(Succeed())
			})

			When("the list_switches tool is called with that user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_switches", map[string]any{"user_id": otherID})
				})

				It("omits the other user's private switch", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())
					Expect(idsOf(decodeListOutput(result))).NotTo(ContainElement(otherSwitchID))
				})
			})
		})

		Context("given another user owns a public switch", func() {
			var (
				otherID       string
				otherSwitchID string
			)

			BeforeEach(func(ctx SpecContext) {
				otherID = api.NewOtherUserID(ctx)

				otherSwitchID = "functional-test-switch-" + uuid.NewString()
				Expect(db.SeedSwitch(ctx, otherID, otherSwitchID, "public")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteSwitch(ctx, otherID, otherSwitchID)).To(Succeed())
			})

			When("the list_switches tool is called with that user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_switches", map[string]any{"user_id": otherID})
				})

				It("includes the other user's public switch, without its price", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeListOutput(result)
					Expect(idsOf(out)).To(ContainElement(otherSwitchID))

					By("omitting the price, since the caller doesn't own this switch")
					seeded := seededBy(out, otherSwitchID)
					Expect(seeded).NotTo(BeNil())
					Expect(seeded.Price).To(BeNil())
				})
			})
		})

		Context("given another user owns a public switch and has show_price_to_others true", func() {
			var (
				otherID         string
				otherSwitchID   string
				profileUsername string
			)

			BeforeEach(func(ctx SpecContext) {
				otherID = api.NewOtherUserID(ctx)

				otherSwitchID = "functional-test-switch-" + uuid.NewString()
				profileUsername = "u" + uuid.NewString()[:8]
				Expect(db.SeedSwitch(ctx, otherID, otherSwitchID, "public")).To(Succeed())
				Expect(db.SeedProfile(ctx, otherID, db.SeedProfileOptions{
					Username: profileUsername,
					Preferences: map[string]any{
						"currency": "USD", "show_price_to_me": true, "show_price_to_others": true,
					},
				})).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteSwitch(ctx, otherID, otherSwitchID)).To(Succeed())
				Expect(db.DeleteProfile(ctx, otherID, profileUsername)).To(Succeed())
			})

			When("the list_switches tool is called with that user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_switches", map[string]any{"user_id": otherID})
				})

				It("includes the other user's public switch with its price", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeListOutput(result)
					seeded := seededBy(out, otherSwitchID)
					Expect(seeded).NotTo(BeNil())
					Expect(seeded.Price).NotTo(BeNil())
					Expect(*seeded.Price).To(BeNumerically("==", 0.35))
				})
			})
		})
	})

	Context("given no bearer token", func() {
		BeforeEach(func() {
			client = api.NewMCPClient(support.BaseURL()+"/mcp", "")
		})

		When("the list_switches tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "list_switches", map[string]any{})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
