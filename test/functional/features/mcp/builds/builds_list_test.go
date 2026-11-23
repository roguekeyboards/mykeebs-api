package builds_test

import (
	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("Listing builds over MCP", func() {
	var (
		client     *api.MCPClient
		result     *sdkmcp.CallToolResult
		err        error
		ownerID    string
		keyboardID string
	)

	BeforeEach(func() {
		result = nil
		err = nil
	})

	Context("given a valid bearer token", func() {
		BeforeEach(func(ctx SpecContext) {
			client, ownerID = api.NewAuthenticatedMCPClient(ctx)

			keyboardID = "build-fixture-keyboard-" + uuid.NewString()
			Expect(db.SeedKeyboard(ctx, ownerID, keyboardID, "private")).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteKeyboard(ctx, ownerID, keyboardID)).To(Succeed())
		})

		Context("given the owner has builds at every visibility tier", func() {
			var publicID, authenticatedID, privateID string

			BeforeEach(func(ctx SpecContext) {
				publicID = "public-build-" + uuid.NewString()
				authenticatedID = "authenticated-build-" + uuid.NewString()
				privateID = "private-build-" + uuid.NewString()

				Expect(db.SeedBuild(ctx, ownerID, publicID, keyboardID, "public")).To(Succeed())
				Expect(db.SeedBuild(ctx, ownerID, authenticatedID, keyboardID, "authenticated")).To(Succeed())
				Expect(db.SeedBuild(ctx, ownerID, privateID, keyboardID, "private")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteBuild(ctx, ownerID, publicID, keyboardID)).To(Succeed())
				Expect(db.DeleteBuild(ctx, ownerID, authenticatedID, keyboardID)).To(Succeed())
				Expect(db.DeleteBuild(ctx, ownerID, privateID, keyboardID)).To(Succeed())
			})

			When("the list_builds tool is called with no user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_builds", map[string]any{})
				})

				It("defaults to the caller's own collection, includes all three tiers, and denormalizes the keyboard", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeListBuildsOutput(result)
					Expect(buildIDsOf(out)).To(ContainElements(publicID, authenticatedID, privateID))

					for _, b := range out.Builds {
						Expect(b.KeyboardID).To(Equal(keyboardID))
						Expect(b.Keyboard).NotTo(BeNil())
						Expect(b.Keyboard.Brand).To(Equal("Keychron"))
						Expect(b.Keyboard.Name).To(Equal("Q1"))
					}
				})
			})

			DescribeTable("given an out-of-range limit",
				func(ctx SpecContext, limit int) {
					result, err = client.CallTool(ctx, "list_builds", map[string]any{"limit": limit})
					Expect(err).NotTo(HaveOccurred())

					By("rejecting the call, as REST does")
					Expect(result.IsError).To(BeTrue())
					Expect(result.Content[0].(*sdkmcp.TextContent).Text).To(ContainSubstring("limit"))
				},
				Entry("below the minimum", 0),
				Entry("above the maximum", 101),
			)
		})

		Context("given the owner has builds referencing two different keyboards", func() {
			var otherKeyboardID, matchingBuildID, otherBuildID string

			BeforeEach(func(ctx SpecContext) {
				otherKeyboardID = "build-fixture-keyboard-" + uuid.NewString()
				matchingBuildID = "matching-build-" + uuid.NewString()
				otherBuildID = "other-build-" + uuid.NewString()

				Expect(db.SeedKeyboard(ctx, ownerID, otherKeyboardID, "private")).To(Succeed())
				Expect(db.SeedBuild(ctx, ownerID, matchingBuildID, keyboardID, "public")).To(Succeed())
				Expect(db.SeedBuild(ctx, ownerID, otherBuildID, otherKeyboardID, "public")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteBuild(ctx, ownerID, matchingBuildID, keyboardID)).To(Succeed())
				Expect(db.DeleteBuild(ctx, ownerID, otherBuildID, otherKeyboardID)).To(Succeed())
				Expect(db.DeleteKeyboard(ctx, ownerID, otherKeyboardID)).To(Succeed())
			})

			When("the list_builds tool is called with keyboard_id set to one of the keyboards", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_builds", map[string]any{"keyboard_id": keyboardID})
				})

				It("returns only the build referencing that keyboard", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					ids := buildIDsOf(decodeListBuildsOutput(result))
					Expect(ids).To(ContainElement(matchingBuildID))
					Expect(ids).NotTo(ContainElement(otherBuildID))
				})
			})
		})

		Context("given another user owns builds at every visibility tier", func() {
			var (
				otherID         string
				otherKeyboardID string
				publicID        string
				authenticatedID string
				privateID       string
			)

			BeforeEach(func(ctx SpecContext) {
				otherID = api.NewOtherUserID(ctx)
				otherKeyboardID = "build-fixture-keyboard-" + uuid.NewString()
				Expect(db.SeedKeyboard(ctx, otherID, otherKeyboardID, "private")).To(Succeed())

				publicID = "public-build-" + uuid.NewString()
				authenticatedID = "authenticated-build-" + uuid.NewString()
				privateID = "private-build-" + uuid.NewString()

				Expect(db.SeedBuild(ctx, otherID, publicID, otherKeyboardID, "public")).To(Succeed())
				Expect(db.SeedBuild(ctx, otherID, authenticatedID, otherKeyboardID, "authenticated")).To(Succeed())
				Expect(db.SeedBuild(ctx, otherID, privateID, otherKeyboardID, "private")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteBuild(ctx, otherID, publicID, otherKeyboardID)).To(Succeed())
				Expect(db.DeleteBuild(ctx, otherID, authenticatedID, otherKeyboardID)).To(Succeed())
				Expect(db.DeleteBuild(ctx, otherID, privateID, otherKeyboardID)).To(Succeed())
				Expect(db.DeleteKeyboard(ctx, otherID, otherKeyboardID)).To(Succeed())
			})

			When("the list_builds tool is called with that user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_builds", map[string]any{"user_id": otherID})
				})

				It("returns the public and authenticated builds, but not the private one", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					ids := buildIDsOf(decodeListBuildsOutput(result))
					Expect(ids).To(ContainElements(publicID, authenticatedID))
					Expect(ids).NotTo(ContainElement(privateID))
				})
			})
		})
	})

	Context("given no bearer token", func() {
		BeforeEach(func() {
			client = api.NewMCPClient(support.BaseURL()+"/mcp", "")
		})

		When("the list_builds tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "list_builds", map[string]any{})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
