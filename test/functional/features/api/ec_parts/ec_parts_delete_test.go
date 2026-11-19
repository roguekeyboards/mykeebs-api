package ec_parts_test

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("Deleting an EC part", func() {
	var (
		resp       *http.Response
		client     *api.ECPartsClient
		builds     *api.BuildsClient
		ownerID    string
		ownerToken string
		partID     string
	)

	BeforeEach(func(ctx SpecContext) {
		resp = nil
		client = api.NewECPartsClient()
		builds = api.NewBuildsClient()

		var err error
		ownerToken, ownerID, err = api.NewAuthIdentity(ctx)
		Expect(err).NotTo(HaveOccurred())

		partID = "delete-ec-part-" + uuid.NewString()
		Expect(db.SeedECPart(ctx, ownerID, partID, "private")).To(Succeed())
		DeferCleanup(func(ctx SpecContext) {
			Expect(db.DeleteECPart(ctx, ownerID, partID)).To(Succeed())
		})
	})

	Context("given no build uses the part", func() {
		When("the owner deletes it", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Delete(ctx, ownerID, partID, ownerToken, "")
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 204 and the part is gone", func(ctx SpecContext) {
				Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

				getResp, err := client.Get(ctx, ownerID, partID, ownerToken)
				Expect(err).NotTo(HaveOccurred())
				Expect(getResp.StatusCode).To(Equal(http.StatusNotFound))
			})
		})
	})

	Context("given an EC build uses the part", func() {
		var keyboardID, buildID string

		BeforeEach(func(ctx SpecContext) {
			keyboardID = "delete-ec-part-keyboard-" + uuid.NewString()
			Expect(db.SeedKeyboard(ctx, ownerID, keyboardID, "private")).To(Succeed())
			DeferCleanup(func(ctx SpecContext) {
				Expect(db.DeleteKeyboard(ctx, ownerID, keyboardID)).To(Succeed())
			})

			createResp, err := builds.Create(ctx, ownerID, ownerToken,
				`{"keyboard":"`+keyboardID+`","build_type":"EC","visibility":"private",`+
					`"ec_parts":[{"ec_part":"`+partID+`","count":60}]}`)
			Expect(err).NotTo(HaveOccurred())
			Expect(createResp.StatusCode).To(Equal(http.StatusCreated))
			var created struct {
				ID string `json:"id"`
			}
			Expect(json.NewDecoder(createResp.Body).Decode(&created)).To(Succeed())
			buildID = created.ID
			DeferCleanup(func(ctx SpecContext) {
				delResp, err := builds.Delete(ctx, ownerID, buildID, ownerToken)
				Expect(err).NotTo(HaveOccurred())
				Expect(delResp.StatusCode).To(Equal(http.StatusNoContent))
			})
		})

		Context("given on_delete is left at its default", func() {
			When("the owner deletes the part", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Delete(ctx, ownerID, partID, ownerToken, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 409 listing the build and keeps the part", func(ctx SpecContext) {
					Expect(resp.StatusCode).To(Equal(http.StatusConflict))
					var problem struct {
						BuildIDs []string `json:"blocking_build_ids"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&problem)).To(Succeed())
					Expect(problem.BuildIDs).To(ConsistOf(buildID))

					getResp, err := client.Get(ctx, ownerID, partID, ownerToken)
					Expect(err).NotTo(HaveOccurred())
					Expect(getResp.StatusCode).To(Equal(http.StatusOK))
				})
			})
		})

		Context("given on_delete is cascade", func() {
			When("the owner deletes the part", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Delete(ctx, ownerID, partID, ownerToken, "cascade")
					Expect(err).NotTo(HaveOccurred())
				})

				It("deletes the part and the build", func(ctx SpecContext) {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))
					var result struct {
						DeletedBuildIDs []string `json:"deleted_build_ids"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&result)).To(Succeed())
					Expect(result.DeletedBuildIDs).To(ConsistOf(buildID))

					buildResp, err := builds.Get(ctx, ownerID, buildID, ownerToken)
					Expect(err).NotTo(HaveOccurred())
					Expect(buildResp.StatusCode).To(Equal(http.StatusNotFound))

					partResp, err := client.Get(ctx, ownerID, partID, ownerToken)
					Expect(err).NotTo(HaveOccurred())
					Expect(partResp.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})
	})
})
