package ec_parts_test

import (
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("Listing EC parts", func() {
	var (
		resp       *http.Response
		client     *api.ECPartsClient
		ownerID    string
		ownerToken string
	)

	BeforeEach(func(ctx SpecContext) {
		resp = nil
		client = api.NewECPartsClient()

		var err error
		ownerToken, ownerID, err = api.NewAuthIdentity(ctx)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("given the owner has public and private domes and public springs", func() {
		var publicDomesID, privateDomesID, springsID string

		BeforeEach(func(ctx SpecContext) {
			publicDomesID = "public-domes-" + uuid.NewString()
			privateDomesID = "private-domes-" + uuid.NewString()
			springsID = "public-springs-" + uuid.NewString()

			Expect(db.SeedECPartOfType(ctx, ownerID, publicDomesID, "public", "domes")).To(Succeed())
			Expect(db.SeedECPartOfType(ctx, ownerID, privateDomesID, "private", "domes")).To(Succeed())
			Expect(db.SeedECPartOfType(ctx, ownerID, springsID, "public", "springs")).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteECPart(ctx, ownerID, publicDomesID)).To(Succeed())
			Expect(db.DeleteECPart(ctx, ownerID, privateDomesID)).To(Succeed())
			Expect(db.DeleteECPart(ctx, ownerID, springsID)).To(Succeed())
		})

		Context("given the caller is the owner", func() {
			Context("given no part type filter", func() {
				When("listing EC parts", func() {
					BeforeEach(func(ctx SpecContext) {
						var err error
						resp, err = client.List(ctx, ownerID, ownerToken, "")
						Expect(err).NotTo(HaveOccurred())
					})

					It("returns every part", func() {
						Expect(resp.StatusCode).To(Equal(http.StatusOK))
						Expect(decodePartIDs(resp)).To(ConsistOf(publicDomesID, privateDomesID, springsID))
					})
				})
			})

			Context("given a part type filter of domes", func() {
				When("listing EC parts", func() {
					BeforeEach(func(ctx SpecContext) {
						var err error
						resp, err = client.List(ctx, ownerID, ownerToken, "domes")
						Expect(err).NotTo(HaveOccurred())
					})

					It("returns only the domes", func() {
						Expect(resp.StatusCode).To(Equal(http.StatusOK))
						Expect(decodePartIDs(resp)).To(ConsistOf(publicDomesID, privateDomesID))
					})
				})
			})
		})

		Context("given the caller is anonymous", func() {
			When("listing EC parts", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.List(ctx, ownerID, "", "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns only the public parts", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))
					Expect(decodePartIDs(resp)).To(ConsistOf(publicDomesID, springsID))
				})
			})
		})
	})

	Context("given an unknown part type", func() {
		When("listing EC parts", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.List(ctx, ownerID, ownerToken, "keycaps")
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 400", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})
	})
})
