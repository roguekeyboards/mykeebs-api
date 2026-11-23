package switches_test

import (
	"encoding/json"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("Creating a switch", func() {
	var (
		resp       *http.Response
		client     *api.SwitchesClient
		ownerID    string
		ownerToken string
		createdID  string
	)

	BeforeEach(func(ctx SpecContext) {
		resp = nil
		client = api.NewSwitchesClient()
		createdID = ""

		var err error
		ownerToken, ownerID, err = api.NewAuthIdentity(ctx)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func(ctx SpecContext) {
		if createdID != "" {
			Expect(db.DeleteSwitch(ctx, ownerID, createdID)).To(Succeed())
		}
	})

	// captureCreatedID is called from BeforeEach, right after a successful
	// Create response is decoded - not from inside an It block after that
	// spec's own assertions. An assertion failing partway through an It
	// must not skip capturing this and leak the created switch into every
	// later spec that lists this owner's switches.
	captureCreatedID := func(r *http.Response) {
		var got struct {
			ID string `json:"id"`
		}
		Expect(json.NewDecoder(r.Body).Decode(&got)).To(Succeed())
		createdID = got.ID
	}

	Context("given the caller is the owner", func() {
		Context("given a valid request body", func() {
			When("creating a switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private"}`)
					Expect(err).NotTo(HaveOccurred())
					if resp.StatusCode == http.StatusCreated {
						captureCreatedID(resp)
					}
				})

				It("creates the switch", func() {
					By("returning 201 Created")
					Expect(resp.StatusCode).To(Equal(http.StatusCreated))

					By("returning a server-generated id")
					Expect(createdID).NotTo(BeEmpty())
				})
			})
		})

		Context("given every open-vocabulary field has an approved lookup value", func() {
			When("creating a switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private",`+
							`"material":{"stem":"`+approvedStem+`"},`+
							`"spring":{"material":"`+approvedSpringMaterial+`"},`+
							`"purchase":{"vendor":"`+approvedVendor+`"}}`)
					Expect(err).NotTo(HaveOccurred())
					if resp.StatusCode == http.StatusCreated {
						captureCreatedID(resp)
					}
				})

				It("creates the switch", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusCreated))
					Expect(createdID).NotTo(BeEmpty())
				})
			})
		})

		Context("given a spring length", func() {
			When("creating a switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"HMX","name":"Azalea","type":"Linear","visibility":"private","spring":{"length":18}}`)
					Expect(err).NotTo(HaveOccurred())
					if resp.StatusCode == http.StatusCreated {
						captureCreatedID(resp)
					}
				})

				It("stores the spring length", func(ctx SpecContext) {
					By("returning 201 Created")
					Expect(resp.StatusCode).To(Equal(http.StatusCreated))

					By("returning the spring length on a later get")
					getResp, err := client.Get(ctx, ownerID, createdID, ownerToken)
					Expect(err).NotTo(HaveOccurred())
					Expect(getResp.StatusCode).To(Equal(http.StatusOK))
					var got struct {
						Spring struct {
							Length *float64 `json:"length"`
						} `json:"spring"`
					}
					Expect(json.NewDecoder(getResp.Body).Decode(&got)).To(Succeed())
					Expect(got.Spring.Length).To(HaveValue(BeNumerically("==", 18)))
				})
			})
		})

		Context("given an open-vocabulary field has an unapproved value", func() {
			When("creating a switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","material":{"stem":"NotApproved"}}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 400 with a problem+json body", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})

		Context("given a required field is present but blank", func() {
			When("creating a switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"","name":"Yellow","type":"Linear","visibility":"private"}`)
					Expect(err).NotTo(HaveOccurred())
					if resp.StatusCode == http.StatusCreated {
						captureCreatedID(resp)
					}
				})

				It("returns 400 with a problem+json body", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})

		Context("given a required field is whitespace only", func() {
			When("creating a switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"   ","name":"Yellow","type":"Linear","visibility":"private"}`)
					Expect(err).NotTo(HaveOccurred())
					if resp.StatusCode == http.StatusCreated {
						captureCreatedID(resp)
					}
				})

				It("returns 400 with a problem+json body", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})

		Context("given required fields are missing", func() {
			When("creating a switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken, `{"brand":"Gateron"}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 400 with a problem+json body", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})

		Context("given the purchase price exceeds Money's maximum", func() {
			When("creating a switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","purchase":{"price":1e15}}`)
					Expect(err).NotTo(HaveOccurred())
					if resp.StatusCode == http.StatusCreated {
						captureCreatedID(resp)
					}
				})

				It("returns 400 with a problem+json body", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})

		Context("given the purchase price is negative", func() {
			When("creating a switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","purchase":{"price":-1}}`)
					Expect(err).NotTo(HaveOccurred())
					if resp.StatusCode == http.StatusCreated {
						captureCreatedID(resp)
					}
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
			When("creating a switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, "",
						`{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private"}`)
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
			When("creating a switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, "", `{"brand":"Gateron"}`)
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

		When("creating a switch in the owner's collection", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Create(ctx, ownerID, token,
					`{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private"}`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 404, not 403, to avoid revealing whose collection this is", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})
	})
})
