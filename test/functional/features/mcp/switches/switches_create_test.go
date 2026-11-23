package switches_test

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("Creating a switch over MCP", func() {
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

	// Captured from BeforeEach rather than inside an It, so an assertion
	// failing partway through a spec still cleans up the created switch
	// instead of leaking it into later list specs.
	captureCreatedID := func(r *sdkmcp.CallToolResult) {
		if r == nil || r.IsError {
			return
		}
		createdID = decodeGetOutput(r).Switch.ID
	}

	AfterEach(func(ctx SpecContext) {
		if createdID != "" {
			Expect(db.DeleteSwitch(ctx, ownerID, createdID)).To(Succeed())
		}
	})

	Context("given a valid bearer token", func() {
		BeforeEach(func(ctx SpecContext) {
			client, ownerID = api.NewAuthenticatedMCPClient(ctx)
		})

		Context("given valid arguments", func() {
			When("the create_switch tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_switch", map[string]any{
						"brand":      "Gateron",
						"name":       "Yellow",
						"type":       approvedType,
						"visibility": "private",
					})
					captureCreatedID(result)
				})

				It("creates the switch in the caller's own collection", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeGetOutput(result)
					Expect(out.Switch.ID).NotTo(BeEmpty(), "the server assigns the id")
					Expect(out.Switch.Brand).To(Equal("Gateron"))
					Expect(out.Switch.Visibility).To(Equal("private"))
				})
			})
		})

		Context("given a type that is not an approved lookup value", func() {
			When("the create_switch tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_switch", map[string]any{
						"brand":      "Gateron",
						"name":       "Yellow",
						"type":       "NotApproved",
						"visibility": "private",
					})
					captureCreatedID(result)
				})

				It("returns an MCP tool error result", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})

		Context("given a negative purchase price", func() {
			When("the create_switch tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_switch", map[string]any{
						"brand":      "Gateron",
						"name":       "Yellow",
						"type":       approvedType,
						"visibility": "private",
						"purchase":   map[string]any{"price": -1},
					})
					captureCreatedID(result)
				})

				It("returns an MCP tool error result naming the price", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
					Expect(result.Content[0].(*sdkmcp.TextContent).Text).To(ContainSubstring("purchase.price"))
				})
			})
		})

		Context("given an invalid visibility", func() {
			When("the create_switch tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_switch", map[string]any{
						"brand":      "Gateron",
						"name":       "Yellow",
						"type":       approvedType,
						"visibility": "everyone",
					})
					captureCreatedID(result)
				})

				It("returns an MCP tool error result", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})

		Context("given a required field is present but blank", func() {
			When("the create_switch tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_switch", map[string]any{
						"brand":      "",
						"name":       "Yellow",
						"type":       approvedType,
						"visibility": "private",
					})
					captureCreatedID(result)
				})

				It("returns an MCP tool error result", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})

		Context("given a required field is missing", func() {
			When("the create_switch tool is called with no brand", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_switch", map[string]any{
						"name":       "Yellow",
						"type":       approvedType,
						"visibility": "private",
					})
					captureCreatedID(result)
				})

				It("is rejected by the tool's own input schema", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})
	})

	Context("given no bearer token", func() {
		BeforeEach(func() {
			client = api.NewMCPClient(support.BaseURL()+"/mcp", "")
		})

		When("the create_switch tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "create_switch", map[string]any{
					"brand":      "Gateron",
					"name":       "Yellow",
					"type":       approvedType,
					"visibility": "private",
				})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
