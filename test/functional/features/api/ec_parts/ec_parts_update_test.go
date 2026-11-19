package ec_parts_test

import (
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("Updating an EC part", func() {
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

		partID = "ec-part-" + uuid.NewString()
		Expect(db.SeedECPart(ctx, ownerID, partID, "private")).To(Succeed())
	})

	AfterEach(func(ctx SpecContext) {
		Expect(db.DeleteECPart(ctx, ownerID, partID)).To(Succeed())
	})

	Context("given the caller is the owner", func() {
		Context("given a body changing the part to sliders", func() {
			When("updating the part", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Update(ctx, ownerID, partID, ownerToken,
						`{"brand":"DynaCap","name":"2U sliders","part_type":"sliders","sliders":{"size":"2U","stem":"OEM"},"visibility":"public"}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("replaces the part, dropping the omitted purchase", func(ctx SpecContext) {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					got, err := client.Get(ctx, ownerID, partID, ownerToken)
					Expect(err).NotTo(HaveOccurred())
					part := decodePart(got)
					Expect(part.PartType).To(Equal("sliders"))
					Expect(part.Sliders).NotTo(BeNil())
					Expect(*part.Sliders.Size).To(Equal("2U"))
					Expect(*part.Sliders.Stem).To(Equal("OEM"))
					Expect(part.Purchase).To(BeNil())
				})
			})
		})

		Context("given a body with another part type's details", func() {
			When("updating the part", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Update(ctx, ownerID, partID, ownerToken,
						`{"brand":"DynaCap","name":"Heavy","part_type":"domes","rings":{"thickness":0.5},"visibility":"private"}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 400", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
				})
			})
		})
	})

	Context("given the part does not exist", func() {
		When("the owner updates it", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Update(ctx, ownerID, "missing-"+uuid.NewString(), ownerToken,
					`{"brand":"DynaCap","name":"Heavy","part_type":"domes","visibility":"private"}`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 404", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
			})
		})
	})
})
