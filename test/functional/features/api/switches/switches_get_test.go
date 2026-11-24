package switches_test

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("Getting a switch", func() {
	var (
		resp       *http.Response
		client     *api.SwitchesClient
		ownerID    string
		ownerToken string
	)

	BeforeEach(func(ctx SpecContext) {
		resp = nil
		client = api.NewSwitchesClient()

		var err error
		ownerToken, ownerID, err = api.NewAuthIdentity(ctx)
		Expect(err).NotTo(HaveOccurred())
	})

	seedSwitch := func(ctx SpecContext, visibility string) string {
		id := visibility + "-switch-" + uuid.NewString()
		Expect(db.SeedSwitch(ctx, ownerID, id, visibility)).To(Succeed())
		return id
	}

	Context("given a public switch", func() {
		var switchID string

		BeforeEach(func(ctx SpecContext) {
			switchID = seedSwitch(ctx, "public")
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteSwitch(ctx, ownerID, switchID)).To(Succeed())
		})

		Context("given the caller is anonymous", func() {
			When("getting the switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, switchID, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the switch without purchase prices", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var got struct {
						ID        string `json:"id"`
						Purchases []struct {
							Vendor *string  `json:"vendor"`
							Price  *float64 `json:"price"`
						} `json:"purchases"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.ID).To(Equal(switchID))

					By("still including non-price purchase fields")
					Expect(got.Purchases).To(HaveLen(1))
					Expect(got.Purchases[0].Vendor).To(HaveValue(Equal("NovelKeys")))

					By("omitting price")
					Expect(got.Purchases[0].Price).To(BeNil())
				})
			})
		})

		Context("given the caller is the owner", func() {
			When("getting the switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, switchID, ownerToken)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the switch with purchase prices", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var got struct {
						Purchases []struct {
							Price *float64 `json:"price"`
						} `json:"purchases"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())

					Expect(got.Purchases).To(HaveLen(1))
					Expect(got.Purchases[0].Price).To(HaveValue(Equal(0.35)))
				})
			})
		})
	})

	Context("given a public switch and the owner has show_price_to_others true", func() {
		var switchID, profileUsername string

		BeforeEach(func(ctx SpecContext) {
			switchID = seedSwitch(ctx, "public")
			profileUsername = "u" + uuid.NewString()[:8]
			Expect(db.SeedProfile(ctx, ownerID, db.SeedProfileOptions{
				Username: profileUsername,
				Preferences: map[string]any{
					"currency": "USD", "show_price_to_me": true, "show_price_to_others": true,
				},
			})).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteSwitch(ctx, ownerID, switchID)).To(Succeed())
			Expect(db.DeleteProfile(ctx, ownerID, profileUsername)).To(Succeed())
		})

		Context("given the caller is not the owner", func() {
			var token string

			BeforeEach(func(ctx SpecContext) {
				var err error
				token, _, err = api.NewAuthIdentity(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			When("getting the switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, switchID, token)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the switch with purchase prices", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var got struct {
						Purchases []struct {
							Price *float64 `json:"price"`
						} `json:"purchases"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())

					Expect(got.Purchases).To(HaveLen(1))
					Expect(got.Purchases[0].Price).To(HaveValue(Equal(0.35)))
				})
			})
		})
	})

	Context("given a private switch and the owner has show_price_to_me false", func() {
		var switchID, profileUsername string

		BeforeEach(func(ctx SpecContext) {
			switchID = seedSwitch(ctx, "private")
			profileUsername = "u" + uuid.NewString()[:8]
			Expect(db.SeedProfile(ctx, ownerID, db.SeedProfileOptions{
				Username: profileUsername,
				Preferences: map[string]any{
					"currency": "USD", "show_price_to_me": false, "show_price_to_others": false,
				},
			})).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteSwitch(ctx, ownerID, switchID)).To(Succeed())
			Expect(db.DeleteProfile(ctx, ownerID, profileUsername)).To(Succeed())
		})

		Context("given the caller is the owner", func() {
			When("getting the switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, switchID, ownerToken)
					Expect(err).NotTo(HaveOccurred())
				})

				It("still returns the switch with purchase prices - single-item GET always shows the owner their own price", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var got struct {
						Purchases []struct {
							Price *float64 `json:"price"`
						} `json:"purchases"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())

					Expect(got.Purchases).To(HaveLen(1))
					Expect(got.Purchases[0].Price).To(HaveValue(Equal(0.35)))
				})
			})
		})
	})

	Context("given an authenticated-only switch", func() {
		var switchID string

		BeforeEach(func(ctx SpecContext) {
			switchID = seedSwitch(ctx, "authenticated")
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteSwitch(ctx, ownerID, switchID)).To(Succeed())
		})

		Context("given the caller is a different authenticated user", func() {
			var token string

			BeforeEach(func(ctx SpecContext) {
				var err error
				token, _, err = api.NewAuthIdentity(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			When("getting the switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, switchID, token)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the switch", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var got struct {
						ID string `json:"id"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.ID).To(Equal(switchID))
				})
			})
		})

		Context("given the caller is anonymous", func() {
			When("getting the switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, switchID, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 404, not 403, to avoid revealing the item exists", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})
	})

	Context("given a private switch", func() {
		var switchID string

		BeforeEach(func(ctx SpecContext) {
			switchID = seedSwitch(ctx, "private")
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteSwitch(ctx, ownerID, switchID)).To(Succeed())
		})

		Context("given the caller is the owner", func() {
			When("getting the switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, switchID, ownerToken)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the switch", func() {
					By("returning 200 OK")
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					By("returning the switch's id")
					var got struct {
						ID string `json:"id"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.ID).To(Equal(switchID))
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

			When("getting the switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, switchID, token)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 404, not 403, to avoid revealing the item exists", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})

		Context("given the caller is anonymous", func() {
			When("getting the switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, switchID, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 404", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})
	})

	Context("given the switch does not exist", func() {
		When("getting the switch", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Get(ctx, ownerID, "no-such-switch-"+uuid.NewString(), ownerToken)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 404", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})
	})
})
