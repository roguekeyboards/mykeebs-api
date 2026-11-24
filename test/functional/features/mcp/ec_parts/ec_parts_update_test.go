package ec_parts_test

import (
	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("Updating an EC part over MCP", func() {
	var (
		client  *api.MCPClient
		result  *sdkmcp.CallToolResult
		err     error
		ownerID string
		partID  string
	)

	BeforeEach(func(ctx SpecContext) {
		result = nil
		err = nil
		partID = "ec-part-" + uuid.NewString()
		client, ownerID = api.NewAuthenticatedMCPClient(ctx)
	})

	Context("given the caller owns the EC part", func() {
		BeforeEach(func(ctx SpecContext) {
			Expect(db.SeedECPart(ctx, ownerID, partID, "private")).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteECPart(ctx, ownerID, partID)).To(Succeed())
		})

		When("the update_ec_part tool replaces it with housings", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "update_ec_part", map[string]any{
					"ec_part_id": partID, "brand": "DynaCap", "name": "Housings", "part_type": "housings",
					"housings": map[string]any{"size": "1U"}, "visibility": "public",
				})
			})

			It("replaces every field, clearing the omitted purchases", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeFalse())

				out := decodePart(result)
				Expect(out.PartType).To(Equal("housings"))
				Expect(out.Housings).NotTo(BeNil())
				Expect(*out.Housings.Size).To(Equal("1U"))
				Expect(out.Purchases).To(BeNil())
			})
		})
	})

	Context("given the EC part does not exist", func() {
		When("the update_ec_part tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "update_ec_part", map[string]any{
					"ec_part_id": partID, "brand": "DynaCap", "name": "Heavy", "part_type": "domes", "visibility": "private",
				})
			})

			It("returns a tool error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeTrue())
			})
		})
	})
})
