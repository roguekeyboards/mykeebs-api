package keyboards_test

import (
	"encoding/json"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("Creating a keyboard", func() {
	var (
		resp       *http.Response
		client     *api.KeyboardsClient
		ownerID    string
		ownerToken string
		createdID  string
	)

	BeforeEach(func(ctx SpecContext) {
		resp = nil
		client = api.NewKeyboardsClient()
		createdID = ""

		var err error
		ownerToken, ownerID, err = api.NewAuthIdentity(ctx)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func(ctx SpecContext) {
		if createdID != "" {
			Expect(db.DeleteKeyboard(ctx, ownerID, createdID)).To(Succeed())
		}
	})

	// captureCreatedID is called from BeforeEach, right after a successful
	// Create response is decoded - not from inside an It block after that
	// spec's own assertions. An assertion failing partway through an It
	// must not skip capturing this and leak the created keyboard into every
	// later spec that lists this owner's keyboards.
	captureCreatedID := func(r *http.Response) {
		var got struct {
			ID string `json:"id"`
		}
		Expect(json.NewDecoder(r.Body).Decode(&got)).To(Succeed())
		createdID = got.ID
	}

	Context("given the caller is the owner", func() {
		Context("given a valid request body", func() {
			When("creating a keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"Keychron","name":"Q1","visibility":"private"}`)
					Expect(err).NotTo(HaveOccurred())
					if resp.StatusCode == http.StatusCreated {
						captureCreatedID(resp)
					}
				})

				It("creates the keyboard", func() {
					By("returning 201 Created")
					Expect(resp.StatusCode).To(Equal(http.StatusCreated))

					By("returning a server-generated id")
					Expect(createdID).NotTo(BeEmpty())
				})
			})
		})

		Context("given a purchase with an estimated delivery date", func() {
			var estimatedDeliveryDate string

			When("creating a keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					estimatedDeliveryDate = ""

					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"Keychron","name":"Q1","visibility":"private",`+
							`"purchase":{"order_date":"2026-03-01","estimated_delivery_date":"2026-06-30"}}`)
					Expect(err).NotTo(HaveOccurred())
					if resp.StatusCode == http.StatusCreated {
						var got struct {
							ID       string `json:"id"`
							Purchase struct {
								EstimatedDeliveryDate string `json:"estimated_delivery_date"`
							} `json:"purchase"`
						}
						Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
						createdID = got.ID
						estimatedDeliveryDate = got.Purchase.EstimatedDeliveryDate
					}
				})

				It("creates the keyboard with the estimate", func() {
					By("returning 201 Created")
					Expect(resp.StatusCode).To(Equal(http.StatusCreated))

					By("returning the estimated delivery date")
					Expect(estimatedDeliveryDate).To(Equal("2026-06-30"))
				})
			})
		})

		Context("given purchase dates before the order date", func() {
			When("creating a keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"Keychron","name":"Q1","visibility":"private",`+
							`"purchase":{"order_date":"2026-03-01","delivery_date":"2026-02-28","estimated_delivery_date":"2026-02-01"}}`)
					Expect(err).NotTo(HaveOccurred())
					if resp.StatusCode == http.StatusCreated {
						captureCreatedID(resp)
					}
				})

				It("returns 400 naming both dates", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))

					var got struct {
						InvalidParams []struct {
							Name string `json:"name"`
						} `json:"invalid_params"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					names := make([]string, len(got.InvalidParams))
					for i, p := range got.InvalidParams {
						names[i] = p.Name
					}
					Expect(names).To(ConsistOf("purchase.delivery_date", "purchase.estimated_delivery_date"))
				})
			})
		})

		Context("given every open-vocabulary field has an approved lookup value", func() {
			When("creating a keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"Keychron","name":"Q1","visibility":"private",`+
							`"size":"`+approvedSize+`","layout":"`+approvedLayout+`",`+
							`"design":{"top_case":{"material":"`+approvedCaseMaterial+`"}},`+
							`"purchase":{"vendor":"`+approvedVendor+`"}}`)
					Expect(err).NotTo(HaveOccurred())
					if resp.StatusCode == http.StatusCreated {
						captureCreatedID(resp)
					}
				})

				It("creates the keyboard", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusCreated))
					Expect(createdID).NotTo(BeEmpty())
				})
			})
		})

		Context("given plates in the body", func() {
			When("creating a keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"Keychron","name":"Q1","visibility":"private","plates":[{"material":"AL"}]}`)
					Expect(err).NotTo(HaveOccurred())
					if resp.StatusCode == http.StatusCreated {
						captureCreatedID(resp)
					}
				})

				It("returns 400, since plates are added through their own route", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
				})
			})
		})

		Context("given an open-vocabulary field has an unapproved value", func() {
			When("creating a keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"Keychron","name":"Q1","visibility":"private","size":"NotApproved"}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 400 with a problem+json body", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})

		Context("given size is approved but not valid for the given layout", func() {
			When("creating a keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"Keychron","name":"Q1","visibility":"private","size":"`+approvedOtherSize+`","layout":"`+approvedLayout+`"}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 400 with a problem+json body naming only layout", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))

					var got struct {
						InvalidParams []struct {
							Name string `json:"name"`
						} `json:"invalid_params"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					names := make([]string, len(got.InvalidParams))
					for i, p := range got.InvalidParams {
						names[i] = p.Name
					}
					Expect(names).To(ConsistOf("layout"))
				})
			})
		})

		Context("given layout is not a recognized value", func() {
			When("creating a keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"Keychron","name":"Q1","visibility":"private","layout":"NotApproved"}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 400 with a problem+json body", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})

		Context("given required fields are missing", func() {
			When("creating a keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken, `{"brand":"Keychron"}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 400 with a problem+json body", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})
	})

	Context("given the caller is anonymous", func() {
		Context("given an otherwise-valid body", func() {
			When("creating a keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, "",
						`{"brand":"Keychron","name":"Q1","visibility":"private"}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 401", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})
		})

		// Auth middleware runs before OpenAPI body validation, so a bad
		// token must win over a bad body.
		Context("given a body missing required fields", func() {
			When("creating a keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, "", `{"brand":"Keychron"}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 401, not 400", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})
		})
	})

	Context("given the caller is a different authenticated user", func() {
		var token string

		BeforeEach(func(ctx SpecContext) {
			var err error
			token, _, err = api.NewAuthIdentity(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		When("creating a keyboard in the owner's collection", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Create(ctx, ownerID, token,
					`{"brand":"Keychron","name":"Q1","visibility":"private"}`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 404, not 403, to avoid revealing whose collection this is", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})
	})
})
