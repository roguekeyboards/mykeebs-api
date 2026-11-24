package ec_parts_test

import (
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("Getting an EC part", func() {
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
	})

	AfterEach(func(ctx SpecContext) {
		Expect(db.DeleteECPart(ctx, ownerID, partID)).To(Succeed())
	})

	Context("given a public EC part", func() {
		BeforeEach(func(ctx SpecContext) {
			partID = "public-ec-part-" + uuid.NewString()
			Expect(db.SeedECPart(ctx, ownerID, partID, "public")).To(Succeed())
		})

		Context("given the caller is anonymous", func() {
			When("getting the part", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, partID, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the part without its price or visibility", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					got := decodePart(resp)
					Expect(got.ID).To(Equal(partID))
					Expect(got.PartType).To(Equal("domes"))
					Expect(got.Visibility).To(BeNil())
					Expect(got.Purchases).To(HaveLen(1))
					Expect(got.Purchases[0].Vendor).To(HaveValue(Equal("Omnitype")))
					Expect(got.Purchases[0].Price).To(BeNil())
				})
			})
		})

		Context("given the caller is the owner", func() {
			When("getting the part", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, partID, ownerToken)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the part with its price and visibility", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					got := decodePart(resp)
					Expect(got.Visibility).NotTo(BeNil())
					Expect(*got.Visibility).To(Equal("public"))
					Expect(got.Purchases).To(HaveLen(1))
					Expect(got.Purchases[0].Price).To(HaveValue(Equal(36.0)))
				})
			})
		})
	})

	Context("given a private EC part", func() {
		BeforeEach(func(ctx SpecContext) {
			partID = "private-ec-part-" + uuid.NewString()
			Expect(db.SeedECPart(ctx, ownerID, partID, "private")).To(Succeed())
		})

		Context("given the caller is anonymous", func() {
			When("getting the part", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, partID, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 404", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})
	})
})
