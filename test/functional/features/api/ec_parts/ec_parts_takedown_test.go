package ec_parts_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/cdn"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/moderation"
)

var _ = Describe("Taking down an EC part image", func() {
	var (
		client     *api.ECPartsClient
		ownerID    string
		ownerToken string
		partID     string
		key        string
		imageURL   string
	)

	BeforeEach(func(ctx SpecContext) {
		client = api.NewECPartsClient()
		var err error
		ownerToken, ownerID, err = api.NewAuthIdentity(ctx)
		Expect(err).NotTo(HaveOccurred())

		partID = "takedown-ec-part-" + uuid.NewString()
		Expect(db.SeedECPart(ctx, ownerID, partID, "public")).To(Succeed())
		DeferCleanup(func(ctx SpecContext) {
			Expect(db.DeleteECPart(ctx, ownerID, partID)).To(Succeed())
		})
	})

	Context("given an approved image whose URL has been fetched", func() {
		BeforeEach(func(ctx SpecContext) {
			resp, err := client.SetImage(ctx, ownerID, partID, ownerToken, api.ImageUploadBody("image/webp"))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			var created struct {
				UploadURL string `json:"upload_url"`
			}
			Expect(json.NewDecoder(resp.Body).Decode(&created)).To(Succeed())

			putResp, err := api.DoBlob(ctx, http.MethodPut, created.UploadURL, "image/webp", bytes.NewReader(api.TestImageBytes))
			Expect(err).NotTo(HaveOccurred())
			Expect(putResp.StatusCode).To(Equal(http.StatusOK))

			Eventually(func() (db.ECPartImage, error) {
				return db.GetECPartImage(ctx, ownerID, partID)
			}).WithTimeout(90 * time.Second).WithPolling(time.Second).Should(HaveField("Status", "approved"))

			img, err := db.GetECPartImage(ctx, ownerID, partID)
			Expect(err).NotTo(HaveOccurred())
			key = img.Path

			getResp, err := client.Get(ctx, ownerID, partID, ownerToken)
			Expect(err).NotTo(HaveOccurred())
			part := decodePart(getResp)
			Expect(part.Image).NotTo(BeNil())
			Expect(part.Image.URL).NotTo(BeNil())
			imageURL = *part.Image.URL

			imgResp, err := api.DoBlob(ctx, http.MethodGet, imageURL, "", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(imgResp.StatusCode).To(Equal(http.StatusOK))
		})

		When("an operator takes it down", func() {
			BeforeEach(func(ctx SpecContext) {
				Expect(moderation.Reject(ctx, key, "takedown")).To(Succeed())
			})

			It("marks the image rejected", func(ctx SpecContext) {
				Expect(db.GetECPartImage(ctx, ownerID, partID)).To(HaveField("Status", "rejected"))
			})

			It("clears the image's cached copies", func(ctx SpecContext) {
				Expect(cdn.Invalidated(ctx, "/"+key)).To(BeTrue())
			})

			It("refuses the image's old URL", func(ctx SpecContext) {
				if !support.IsCI() {
					Skip("floci's bucket policies aren't offered s3:ExistingObjectTag, so the rejected-image deny never applies")
				}
				Eventually(func() (int, error) {
					resp, err := api.DoBlob(ctx, http.MethodGet, imageURL, "", nil)
					if err != nil {
						return 0, err
					}
					return resp.StatusCode, nil
				}).WithTimeout(2 * time.Minute).WithPolling(5 * time.Second).Should(Equal(http.StatusForbidden))
			})
		})
	})
})
