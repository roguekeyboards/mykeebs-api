package keycapsets_test

import (
	"bytes"
	"net/http"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("Listing keycap sets over MCP", func() {
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

		Context("given the owner has keycap sets at every visibility tier", func() {
			var publicID, authenticatedID, privateID string

			BeforeEach(func(ctx SpecContext) {
				publicID = "public-keycap-set-" + uuid.NewString()
				authenticatedID = "authenticated-keycap-set-" + uuid.NewString()
				privateID = "private-keycap-set-" + uuid.NewString()

				Expect(db.SeedKeycapSet(ctx, ownerID, publicID, "public")).To(Succeed())
				Expect(db.SeedKeycapSet(ctx, ownerID, authenticatedID, "authenticated")).To(Succeed())
				Expect(db.SeedKeycapSet(ctx, ownerID, privateID, "private")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeycapSet(ctx, ownerID, publicID)).To(Succeed())
				Expect(db.DeleteKeycapSet(ctx, ownerID, authenticatedID)).To(Succeed())
				Expect(db.DeleteKeycapSet(ctx, ownerID, privateID)).To(Succeed())
			})

			When("the list_keycap_sets tool is called with no user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_keycap_sets", map[string]any{})
				})

				It("defaults to the caller's own collection and includes all three tiers", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					ids := idsOf(decodeListOutput(result))
					Expect(ids).To(ContainElements(publicID, authenticatedID, privateID))
				})
			})

			DescribeTable("given an out-of-range limit",
				func(ctx SpecContext, limit int) {
					result, err = client.CallTool(ctx, "list_keycap_sets", map[string]any{"limit": limit})
					Expect(err).NotTo(HaveOccurred())

					By("rejecting the call, as REST does")
					Expect(result.IsError).To(BeTrue())
					Expect(result.Content[0].(*sdkmcp.TextContent).Text).To(ContainSubstring("limit"))
				},
				Entry("below the minimum", 0),
				Entry("above the maximum", 101),
			)
		})

		Context("given the owner has a keycap set with a primary kit that has an image", func() {
			var (
				keycapSetID string
				kitID       string
			)

			BeforeEach(func(ctx SpecContext) {
				keycapSetID = "primary-kit-image-keycap-set-" + uuid.NewString()
				kitID = "kit-" + uuid.NewString()
				Expect(db.SeedKeycapSetWithPrimaryKit(ctx, ownerID, keycapSetID, kitID, "public")).To(Succeed())

				setResult, setErr := client.CallTool(ctx, "set_keycap_kit_image", map[string]any{
					"keycap_set_id": keycapSetID,
					"kit_id":        kitID,
					"content_type":  approvedImageContentType,
					"size_bytes":    len(api.TestImageBytes),
				})
				Expect(setErr).NotTo(HaveOccurred())
				Expect(setResult.IsError).To(BeFalse())

				putResp, putErr := api.DoBlob(ctx, http.MethodPut, decodeUploadURL(setResult), approvedImageContentType, bytes.NewReader(api.TestImageBytes))
				Expect(putErr).NotTo(HaveOccurred())
				Expect(putResp.StatusCode).To(Equal(http.StatusOK))
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeycapSet(ctx, ownerID, keycapSetID)).To(Succeed())
			})

			When("the list_keycap_sets tool is called with no user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_keycap_sets", map[string]any{})
				})

				It("reports primary_kit_id and primary_kit_has_image true, without a URL", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeListOutput(result)
					found := false
					for _, ks := range out.KeycapSets {
						if ks.ID != keycapSetID {
							continue
						}
						found = true
						Expect(ks.PrimaryKitID).NotTo(BeNil())
						Expect(*ks.PrimaryKitID).To(Equal(kitID))
						Expect(ks.PrimaryKitHasImage).To(BeTrue())
					}
					Expect(found).To(BeTrue(), "expected to find seeded keycap set %q in the list", keycapSetID)
				})
			})
		})

		Context("given the owner has a keycap set whose primary kit no longer exists", func() {
			var keycapSetID string

			BeforeEach(func(ctx SpecContext) {
				keycapSetID = "dangling-primary-kit-keycap-set-" + uuid.NewString()
				Expect(db.SeedKeycapSetWithDanglingPrimaryKit(ctx, ownerID, keycapSetID, "public")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeycapSet(ctx, ownerID, keycapSetID)).To(Succeed())
			})

			When("the list_keycap_sets tool is called with no user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_keycap_sets", map[string]any{})
				})

				It("reports a nil primary_kit_id and primary_kit_has_image false", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeListOutput(result)
					found := false
					for _, ks := range out.KeycapSets {
						if ks.ID != keycapSetID {
							continue
						}
						found = true
						Expect(ks.PrimaryKitID).To(BeNil())
						Expect(ks.PrimaryKitHasImage).To(BeFalse())
					}
					Expect(found).To(BeTrue(), "expected to find seeded keycap set %q in the list", keycapSetID)
				})
			})
		})

		Context("given the owner has a keycap set with a priced kit", func() {
			var (
				keycapSetID string
				kitID       string
			)

			BeforeEach(func(ctx SpecContext) {
				keycapSetID = "priced-keycap-set-" + uuid.NewString()
				kitID = "kit-" + uuid.NewString()
				Expect(db.SeedKeycapSetWithPrimaryKit(ctx, ownerID, keycapSetID, kitID, "public")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeycapSet(ctx, ownerID, keycapSetID)).To(Succeed())
			})

			When("the list_keycap_sets tool is called with no user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_keycap_sets", map[string]any{})
				})

				It("includes the total_cost, since the caller owns this set", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeListOutput(result)
					seeded := seededBy(out, keycapSetID)
					Expect(seeded).NotTo(BeNil())
					Expect(seeded.TotalCost).NotTo(BeNil())
					Expect(*seeded.TotalCost).To(BeNumerically("==", 85.0))
				})
			})
		})

		Context("given the owner has a priced kit and show_price_to_me is false", func() {
			var (
				keycapSetID     string
				kitID           string
				profileUsername string
			)

			BeforeEach(func(ctx SpecContext) {
				keycapSetID = "priced-keycap-set-" + uuid.NewString()
				kitID = "kit-" + uuid.NewString()
				profileUsername = "u" + uuid.NewString()[:8]
				Expect(db.SeedKeycapSetWithPrimaryKit(ctx, ownerID, keycapSetID, kitID, "public")).To(Succeed())
				Expect(db.SeedProfile(ctx, ownerID, db.SeedProfileOptions{
					Username: profileUsername,
					Preferences: map[string]any{
						"currency": "USD", "show_price_to_me": false, "show_price_to_others": false,
					},
				})).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeycapSet(ctx, ownerID, keycapSetID)).To(Succeed())
				Expect(db.DeleteProfile(ctx, ownerID, profileUsername)).To(Succeed())
			})

			When("the list_keycap_sets tool is called with no user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_keycap_sets", map[string]any{})
				})

				It("omits the total_cost", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeListOutput(result)
					seeded := seededBy(out, keycapSetID)
					Expect(seeded).NotTo(BeNil())
					Expect(seeded.TotalCost).To(BeNil())
				})
			})
		})

		Context("given another user owns keycap sets at every visibility tier", func() {
			var (
				otherID         string
				publicID        string
				authenticatedID string
				privateID       string
			)

			BeforeEach(func(ctx SpecContext) {
				otherID = api.NewOtherUserID(ctx)

				publicID = "public-keycap-set-" + uuid.NewString()
				authenticatedID = "authenticated-keycap-set-" + uuid.NewString()
				privateID = "private-keycap-set-" + uuid.NewString()

				Expect(db.SeedKeycapSet(ctx, otherID, publicID, "public")).To(Succeed())
				Expect(db.SeedKeycapSet(ctx, otherID, authenticatedID, "authenticated")).To(Succeed())
				Expect(db.SeedKeycapSet(ctx, otherID, privateID, "private")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeycapSet(ctx, otherID, publicID)).To(Succeed())
				Expect(db.DeleteKeycapSet(ctx, otherID, authenticatedID)).To(Succeed())
				Expect(db.DeleteKeycapSet(ctx, otherID, privateID)).To(Succeed())
			})

			When("the list_keycap_sets tool is called with that user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_keycap_sets", map[string]any{"user_id": otherID})
				})

				It("returns the public and authenticated keycap sets, but not the private one", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					ids := idsOf(decodeListOutput(result))
					Expect(ids).To(ContainElements(publicID, authenticatedID))
					Expect(ids).NotTo(ContainElement(privateID))
				})
			})
		})

		Context("given another user owns a keycap set with a priced kit", func() {
			var (
				otherID     string
				keycapSetID string
				kitID       string
			)

			BeforeEach(func(ctx SpecContext) {
				otherID = api.NewOtherUserID(ctx)

				keycapSetID = "priced-keycap-set-" + uuid.NewString()
				kitID = "kit-" + uuid.NewString()
				Expect(db.SeedKeycapSetWithPrimaryKit(ctx, otherID, keycapSetID, kitID, "public")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeycapSet(ctx, otherID, keycapSetID)).To(Succeed())
			})

			When("the list_keycap_sets tool is called with that user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_keycap_sets", map[string]any{"user_id": otherID})
				})

				It("omits the total_cost, since the caller doesn't own this set", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeListOutput(result)
					seeded := seededBy(out, keycapSetID)
					Expect(seeded).NotTo(BeNil())
					Expect(seeded.TotalCost).To(BeNil())
				})
			})
		})

		Context("given another user owns a keycap set with a priced kit and has show_price_to_others true", func() {
			var (
				otherID         string
				keycapSetID     string
				kitID           string
				profileUsername string
			)

			BeforeEach(func(ctx SpecContext) {
				otherID = api.NewOtherUserID(ctx)

				keycapSetID = "priced-keycap-set-" + uuid.NewString()
				kitID = "kit-" + uuid.NewString()
				profileUsername = "u" + uuid.NewString()[:8]
				Expect(db.SeedKeycapSetWithPrimaryKit(ctx, otherID, keycapSetID, kitID, "public")).To(Succeed())
				Expect(db.SeedProfile(ctx, otherID, db.SeedProfileOptions{
					Username: profileUsername,
					Preferences: map[string]any{
						"currency": "USD", "show_price_to_me": true, "show_price_to_others": true,
					},
				})).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeycapSet(ctx, otherID, keycapSetID)).To(Succeed())
				Expect(db.DeleteProfile(ctx, otherID, profileUsername)).To(Succeed())
			})

			When("the list_keycap_sets tool is called with that user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_keycap_sets", map[string]any{"user_id": otherID})
				})

				It("includes the total_cost", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeListOutput(result)
					seeded := seededBy(out, keycapSetID)
					Expect(seeded).NotTo(BeNil())
					Expect(seeded.TotalCost).NotTo(BeNil())
					Expect(*seeded.TotalCost).To(BeNumerically("==", 85.0))
				})
			})
		})
	})

	Context("given no bearer token", func() {
		BeforeEach(func() {
			client = api.NewMCPClient(support.BaseURL()+"/mcp", "")
		})

		When("the list_keycap_sets tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "list_keycap_sets", map[string]any{})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
