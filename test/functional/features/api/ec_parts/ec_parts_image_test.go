package ec_parts_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("An EC part's image", func() {
	var (
		resp       *http.Response
		client     *api.ECPartsClient
		ownerID    string
		ownerToken string
		partID     string
	)

	BeforeEach(func(ctx SpecContext) {
		resp = nil
		client = api.NewECPartsClient()

		var err error
		ownerToken, ownerID, err = api.NewAuthIdentity(ctx)
		Expect(err).NotTo(HaveOccurred())

		partID = "image-ec-part-" + uuid.NewString()
		Expect(db.SeedECPart(ctx, ownerID, partID, "public")).To(Succeed())
		DeferCleanup(func(ctx SpecContext) {
			Expect(db.DeleteECPart(ctx, ownerID, partID)).To(Succeed())
		})
	})

	getPart := func(ctx SpecContext, token string) ecPart {
		GinkgoHelper()
		r, err := client.Get(ctx, ownerID, partID, token)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.StatusCode).To(Equal(http.StatusOK))
		return decodePart(r)
	}

	Context("given a public EC part owned by the caller", func() {
		Context("given an approved content_type", func() {
			When("the owner sets the part's image", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.SetImage(ctx, ownerID, partID, ownerToken, api.ImageUploadBody("image/webp"))
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns an upload URL whose image is moderated and then served", func(ctx SpecContext) {
					By("returning 201 with upload_url")
					Expect(resp.StatusCode).To(Equal(http.StatusCreated))
					var created struct {
						UploadURL string `json:"upload_url"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&created)).To(Succeed())

					By("accepting the image at the presigned URL")
					putResp, err := api.DoBlob(ctx, http.MethodPut, created.UploadURL, "image/webp", bytes.NewReader(api.TestImageBytes))
					Expect(err).NotTo(HaveOccurred())
					Expect(putResp.StatusCode).To(Equal(http.StatusOK))

					By("having the moderation function approve it")
					Eventually(func() (db.ECPartImage, error) {
						return db.GetECPartImage(ctx, ownerID, partID)
					}).WithTimeout(90 * time.Second).WithPolling(time.Second).Should(HaveField("Status", "approved"))

					By("serving the uploaded bytes to an anonymous caller")
					part := getPart(ctx, "")
					Expect(part.Image).NotTo(BeNil())
					Expect(part.Image.URL).NotTo(BeNil())
					imgResp, err := api.DoBlob(ctx, http.MethodGet, *part.Image.URL, "", nil)
					Expect(err).NotTo(HaveOccurred())
					Expect(imgResp.StatusCode).To(Equal(http.StatusOK))
					got, err := io.ReadAll(imgResp.Body)
					Expect(err).NotTo(HaveOccurred())
					Expect(got).To(Equal(api.TestImageBytes))
				})
			})
		})

		Context("given an unapproved content_type", func() {
			When("the owner sets the part's image", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.SetImage(ctx, ownerID, partID, ownerToken, api.ImageUploadBody("image/png"))
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 400", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
				})
			})
		})

		Context("given the caller is another user", func() {
			var otherToken string

			BeforeEach(func(ctx SpecContext) {
				var err error
				otherToken, _, err = api.NewAuthIdentity(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			When("they set the part's image", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.SetImage(ctx, ownerID, partID, otherToken, api.ImageUploadBody("image/webp"))
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 404", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})

		Context("given the part has an image", func() {
			BeforeEach(func(ctx SpecContext) {
				setResp, err := client.SetImage(ctx, ownerID, partID, ownerToken, api.ImageUploadBody("image/webp"))
				Expect(err).NotTo(HaveOccurred())
				Expect(setResp.StatusCode).To(Equal(http.StatusCreated))
			})

			When("the owner deletes the image", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.DeleteImage(ctx, ownerID, partID, ownerToken)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 204 and the part no longer has an image", func(ctx SpecContext) {
					Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
					Expect(getPart(ctx, ownerToken).Image).To(BeNil())
				})
			})
		})
	})
})
