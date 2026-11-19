package ec_parts_test

import (
	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("Getting an EC part over MCP", func() {
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

	Context("given the caller owns a private EC part", func() {
		BeforeEach(func(ctx SpecContext) {
			Expect(db.SeedECPart(ctx, ownerID, partID, "private")).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteECPart(ctx, ownerID, partID)).To(Succeed())
		})

		When("the get_ec_part tool is called with no user_id", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "get_ec_part", map[string]any{"ec_part_id": partID})
			})

			It("returns the part from the caller's own collection", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeFalse())

				out := decodePart(result)
				Expect(out.ID).To(Equal(partID))
				Expect(out.PartType).To(Equal("domes"))
				Expect(out.Visibility).To(Equal("private"))
			})
		})
	})

	Context("given another user owns a private EC part", func() {
		var otherID string

		BeforeEach(func(ctx SpecContext) {
			var tokenErr error
			_, otherID, tokenErr = api.NewAuthIdentity(ctx)
			Expect(tokenErr).NotTo(HaveOccurred())
			Expect(db.SeedECPart(ctx, otherID, partID, "private")).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteECPart(ctx, otherID, partID)).To(Succeed())
		})

		When("the get_ec_part tool is called with their user_id", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "get_ec_part", map[string]any{"ec_part_id": partID, "user_id": otherID})
			})

			It("returns a not-found tool error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeTrue())
			})
		})
	})
})
