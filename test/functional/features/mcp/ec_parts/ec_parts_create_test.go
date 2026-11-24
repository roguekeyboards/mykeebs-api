package ec_parts_test

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("Creating an EC part over MCP", func() {
	var (
		client    *api.MCPClient
		result    *sdkmcp.CallToolResult
		err       error
		ownerID   string
		createdID string
	)

	BeforeEach(func() {
		result = nil
		err = nil
		createdID = ""
	})

	AfterEach(func(ctx SpecContext) {
		if createdID != "" {
			Expect(db.DeleteECPart(ctx, ownerID, createdID)).To(Succeed())
		}
	})

	call := func(ctx SpecContext, args map[string]any) {
		result, err = client.CallTool(ctx, "create_ec_part", args)
		if err == nil && !result.IsError {
			createdID = decodePart(result).ID
		}
	}

	Context("given a valid bearer token", func() {
		BeforeEach(func(ctx SpecContext) {
			client, ownerID = api.NewAuthenticatedMCPClient(ctx)
		})

		Context("given rings with approved values", func() {
			When("the create_ec_part tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					call(ctx, map[string]any{
						"brand": "DynaCap", "name": "Rings", "part_type": "rings",
						"rings": map[string]any{"material": "Silicone", "thickness": 0.5},
						"purchases": []map[string]any{
							{"vendor": "Omnitype", "quantity": 140, "price": 6, "order_status": "Delivered"},
							{"vendor": "Omnitype", "quantity": 70, "price": 4.8, "order_status": "Ordered"},
						},
						"visibility": "private",
					})
				})

				It("creates the part in the caller's own collection", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodePart(result)
					Expect(out.ID).NotTo(BeEmpty())
					Expect(out.PartType).To(Equal("rings"))
					Expect(out.Rings).NotTo(BeNil())
					Expect(*out.Rings.Thickness).To(Equal(0.5))
					Expect(out.Visibility).To(Equal("private"))

					By("keeping every purchase and totalling their quantities")
					Expect(out.Purchases).To(HaveLen(2))
					Expect(out.TotalQuantity).To(HaveValue(Equal(210)))

					By("deriving the least-progressed order status and totalling their prices")
					Expect(out.OrderStatus).To(HaveValue(Equal("Ordered")))
					Expect(out.TotalCost).To(HaveValue(Equal(10.8)))
				})
			})
		})

		Context("given another part type's details", func() {
			When("the create_ec_part tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					call(ctx, map[string]any{
						"brand": "DynaCap", "name": "Rings", "part_type": "rings",
						"domes":      map[string]any{"weight": 45},
						"visibility": "private",
					})
				})

				It("returns a tool error naming the field", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
					Expect(result.Content[0].(*sdkmcp.TextContent).Text).To(ContainSubstring("domes"))
				})
			})
		})
	})
})
