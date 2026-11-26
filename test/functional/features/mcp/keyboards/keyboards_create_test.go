package keyboards_test

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("Creating a keyboard over MCP", func() {
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

	captureCreatedID := func(r *sdkmcp.CallToolResult) {
		if r == nil || r.IsError {
			return
		}
		createdID = decodeGetOutput(r).Keyboard.ID
	}

	AfterEach(func(ctx SpecContext) {
		if createdID != "" {
			Expect(db.DeleteKeyboard(ctx, ownerID, createdID)).To(Succeed())
		}
	})

	Context("given a valid bearer token", func() {
		BeforeEach(func(ctx SpecContext) {
			client, ownerID = api.NewAuthenticatedMCPClient(ctx)
		})

		Context("given valid arguments", func() {
			When("the create_keyboard tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_keyboard", map[string]any{
						"brand":      "Mode",
						"name":       "Sixty",
						"size":       approvedSize,
						"layout":     approvedLayout,
						"visibility": "private",
					})
					captureCreatedID(result)
				})

				It("creates the keyboard in the caller's own collection", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeGetOutput(result)
					Expect(out.Keyboard.ID).NotTo(BeEmpty(), "the server assigns the id")
					Expect(out.Keyboard.Brand).To(Equal("Mode"))
					Expect(out.Keyboard.Visibility).NotTo(BeNil())
					Expect(*out.Keyboard.Visibility).To(Equal("private"))
				})
			})
		})

		Context("given a size that is not an approved lookup value", func() {
			When("the create_keyboard tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_keyboard", map[string]any{
						"brand":      "Mode",
						"name":       "Sixty",
						"size":       "NotApproved",
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

		// Both values are individually approved; only the pairing is wrong.
		Context("given a size that is approved but not valid for the layout", func() {
			When("the create_keyboard tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_keyboard", map[string]any{
						"brand":      "Mode",
						"name":       "Sixty",
						"size":       approvedOtherSize,
						"layout":     approvedLayout,
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

		// REST rejects this at the door via openapi.yaml's `format: date`;
		// MCP has no such validator, and a stored bad date makes every later
		// REST read of the row fail its date parse.
		Context("given a malformed purchase date", func() {
			When("the create_keyboard tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_keyboard", map[string]any{
						"brand":      "Mode",
						"name":       "Sixty",
						"visibility": "private",
						"purchase":   map[string]any{"order_date": "next tuesday"},
					})
					captureCreatedID(result)
				})

				It("returns an MCP tool error result", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})

		Context("given an estimated delivery date before the order date", func() {
			When("the create_keyboard tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_keyboard", map[string]any{
						"brand":      "Mode",
						"name":       "Sixty",
						"visibility": "private",
						"purchase":   map[string]any{"order_date": "2026-03-01", "estimated_delivery_date": "2026-02-01"},
					})
					captureCreatedID(result)
				})

				It("returns an MCP tool error result", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})

		Context("given an invalid visibility", func() {
			When("the create_keyboard tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_keyboard", map[string]any{
						"brand":      "Mode",
						"name":       "Sixty",
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
			When("the create_keyboard tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_keyboard", map[string]any{
						"brand":      "",
						"name":       "Sixty",
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
	})

	Context("given no bearer token", func() {
		BeforeEach(func() {
			client = api.NewMCPClient(support.BaseURL()+"/mcp", "")
		})

		When("the create_keyboard tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "create_keyboard", map[string]any{
					"brand":      "Mode",
					"name":       "Sixty",
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
