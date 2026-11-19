package ec_parts_test

import (
	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("Listing EC parts over MCP", func() {
	var (
		client  *api.MCPClient
		result  *sdkmcp.CallToolResult
		err     error
		ownerID string
	)

	BeforeEach(func(ctx SpecContext) {
		result = nil
		err = nil
		client, ownerID = api.NewAuthenticatedMCPClient(ctx)
	})

	Context("given the caller owns domes and housings", func() {
		var domesID, housingsID string

		BeforeEach(func(ctx SpecContext) {
			domesID = "domes-" + uuid.NewString()
			housingsID = "housings-" + uuid.NewString()
			Expect(db.SeedECPartOfType(ctx, ownerID, domesID, "private", "domes")).To(Succeed())
			Expect(db.SeedECPartOfType(ctx, ownerID, housingsID, "private", "housings")).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteECPart(ctx, ownerID, domesID)).To(Succeed())
			Expect(db.DeleteECPart(ctx, ownerID, housingsID)).To(Succeed())
		})

		Context("given no part_type", func() {
			When("the list_ec_parts tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_ec_parts", map[string]any{})
				})

				It("returns both parts", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())
					Expect(decodePartIDs(result)).To(ConsistOf(domesID, housingsID))
				})
			})
		})

		Context("given part_type housings", func() {
			When("the list_ec_parts tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_ec_parts", map[string]any{"part_type": "housings"})
				})

				It("returns only the housings", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())
					Expect(decodePartIDs(result)).To(ConsistOf(housingsID))
				})
			})
		})
	})
})
