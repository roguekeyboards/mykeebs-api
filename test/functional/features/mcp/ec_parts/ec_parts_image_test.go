package ec_parts_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("An EC part's image over MCP", func() {
	var (
		client  *api.MCPClient
		result  *sdkmcp.CallToolResult
		err     error
		ownerID string
		partID  string
	)

	setImage := func(ctx SpecContext) string {
		GinkgoHelper()
		r, callErr := client.CallTool(ctx, "set_ec_part_image", map[string]any{
			"ec_part_id": partID, "content_type": "image/webp", "size_bytes": len(api.TestImageBytes),
		})
		Expect(callErr).NotTo(HaveOccurred())
		Expect(r.IsError).To(BeFalse())
		raw, marshalErr := json.Marshal(r.StructuredContent)
		Expect(marshalErr).NotTo(HaveOccurred())
		var out struct {
			UploadURL string `json:"upload_url"`
		}
		Expect(json.Unmarshal(raw, &out)).To(Succeed())
		return out.UploadURL
	}

	hasImage := func(ctx SpecContext) bool {
		GinkgoHelper()
		r, callErr := client.CallTool(ctx, "get_ec_part", map[string]any{"ec_part_id": partID})
		Expect(callErr).NotTo(HaveOccurred())
		Expect(r.IsError).To(BeFalse())
		return decodePart(r).HasImage
	}

	BeforeEach(func(ctx SpecContext) {
		result = nil
		err = nil
		client, ownerID = api.NewAuthenticatedMCPClient(ctx)
		partID = "image-ec-part-" + uuid.NewString()
		Expect(db.SeedECPart(ctx, ownerID, partID, "private")).To(Succeed())
		DeferCleanup(func(ctx SpecContext) {
			Expect(db.DeleteECPart(ctx, ownerID, partID)).To(Succeed())
		})
	})

	Context("given the caller owns an EC part without an image", func() {
		When("set_ec_part_image is called and the image uploaded to its URL", func() {
			BeforeEach(func(ctx SpecContext) {
				uploadURL := setImage(ctx)
				putResp, putErr := api.DoBlob(ctx, http.MethodPut, uploadURL, "image/webp", bytes.NewReader(api.TestImageBytes))
				Expect(putErr).NotTo(HaveOccurred())
				Expect(putResp.StatusCode).To(Equal(http.StatusOK))
			})

			It("stores an image that moderation approves", func(ctx SpecContext) {
				Eventually(func() (db.ECPartImage, error) {
					return db.GetECPartImage(ctx, ownerID, partID)
				}).WithTimeout(90 * time.Second).WithPolling(time.Second).Should(HaveField("Status", "approved"))
				Expect(hasImage(ctx)).To(BeTrue())
			})
		})
	})

	Context("given the caller owns an EC part with an image", func() {
		BeforeEach(func(ctx SpecContext) {
			setImage(ctx)
		})

		When("delete_ec_part_image is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "delete_ec_part_image", map[string]any{"ec_part_id": partID})
			})

			It("removes the image", func(ctx SpecContext) {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeFalse())
				Expect(hasImage(ctx)).To(BeFalse())
			})
		})
	})
})
