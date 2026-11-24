package ec_parts_test

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("Creating an EC part", func() {
	var (
		resp       *http.Response
		client     *api.ECPartsClient
		ownerID    string
		ownerToken string
		created    ecPart
	)

	BeforeEach(func(ctx SpecContext) {
		resp = nil
		client = api.NewECPartsClient()
		created = ecPart{}

		var err error
		ownerToken, ownerID, err = api.NewAuthIdentity(ctx)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func(ctx SpecContext) {
		if created.ID != "" {
			Expect(db.DeleteECPart(ctx, ownerID, created.ID)).To(Succeed())
		}
	})

	create := func(ctx SpecContext, owner, token, body string) {
		var err error
		resp, err = client.Create(ctx, owner, token, body)
		Expect(err).NotTo(HaveOccurred())
		if resp.StatusCode == http.StatusCreated {
			created = decodePart(resp)
		}
	}

	Context("given the caller is the owner", func() {
		Context("given domes with approved values", func() {
			When("creating an EC part", func() {
				BeforeEach(func(ctx SpecContext) {
					create(ctx, ownerID, ownerToken,
						`{"brand":"DynaCap","name":"Heavy","part_type":"domes",`+
							`"domes":{"weight":55,"material":"Silicone","color":"black"},`+
							`"purchases":[{"vendor":"Omnitype","price":36,"quantity":72}],"visibility":"private"}`)
				})

				It("creates the part with its dome details", func() {
					By("returning 201 Created")
					Expect(resp.StatusCode).To(Equal(http.StatusCreated))

					By("returning a server-generated id and the domes object")
					Expect(created.ID).NotTo(BeEmpty())
					Expect(created.PartType).To(Equal("domes"))
					Expect(created.Domes).NotTo(BeNil())
					Expect(*created.Domes.Weight).To(Equal(55.0))
					Expect(*created.Domes.Material).To(Equal("Silicone"))

					By("recording the purchase")
					Expect(created.Purchases).To(HaveLen(1))
					Expect(created.Purchases[0].Vendor).To(HaveValue(Equal("Omnitype")))
					Expect(created.TotalQuantity).To(HaveValue(Equal(72)))
				})
			})
		})

		Context("given several purchases", func() {
			When("creating an EC part", func() {
				BeforeEach(func(ctx SpecContext) {
					create(ctx, ownerID, ownerToken,
						`{"brand":"DynaCap","name":"Heavy","part_type":"domes","visibility":"private","purchases":[`+
							`{"vendor":"Omnitype","order_date":"2026-11-23","quantity":72,"price":28.8,"order_status":"Ordered"},`+
							`{"vendor":"Omnitype","order_date":"2026-09-21","quantity":72,"price":36,"order_status":"Delivered"}]}`)
				})

				It("keeps every purchase", func() {
					By("returning 201 Created")
					Expect(resp.StatusCode).To(Equal(http.StatusCreated))

					By("returning both purchases in order")
					Expect(created.Purchases).To(HaveLen(2))
					Expect(created.Purchases[0].OrderDate).To(HaveValue(Equal("2026-11-23")))
					Expect(created.Purchases[1].OrderDate).To(HaveValue(Equal("2026-09-21")))

					By("totalling their quantities")
					Expect(created.TotalQuantity).To(HaveValue(Equal(144)))

					By("deriving the least-progressed order status")
					Expect(created.OrderStatus).To(HaveValue(Equal("Ordered")))

					By("totalling their prices")
					Expect(created.TotalCost).To(HaveValue(Equal(64.8)))
				})
			})
		})

		Context("given another part type's details", func() {
			When("creating an EC part", func() {
				BeforeEach(func(ctx SpecContext) {
					create(ctx, ownerID, ownerToken,
						`{"brand":"DynaCap","name":"Heavy","part_type":"domes","sliders":{"size":"1U"},"visibility":"private"}`)
				})

				It("returns 400 with a problem+json body", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})

		Context("given an unknown part type", func() {
			When("creating an EC part", func() {
				BeforeEach(func(ctx SpecContext) {
					create(ctx, ownerID, ownerToken,
						`{"brand":"DynaCap","name":"Heavy","part_type":"keycaps","visibility":"private"}`)
				})

				It("returns 400 with a problem+json body", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})

		Context("given an unapproved material", func() {
			When("creating an EC part", func() {
				BeforeEach(func(ctx SpecContext) {
					create(ctx, ownerID, ownerToken,
						`{"brand":"DynaCap","name":"Heavy","part_type":"domes","domes":{"material":"Steel"},"visibility":"private"}`)
				})

				It("returns 400 with a problem+json body", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
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

		When("creating an EC part in the owner's collection", func() {
			BeforeEach(func(ctx SpecContext) {
				create(ctx, ownerID, otherToken,
					`{"brand":"DynaCap","name":"Heavy","part_type":"domes","visibility":"private"}`)
			})

			It("returns 404", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
			})
		})
	})
})
