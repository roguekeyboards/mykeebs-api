package switches_test

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

type listItem struct {
	ID        string `json:"id"`
	Brand     string `json:"brand"`
	Purchases []struct {
		Vendor      *string  `json:"vendor"`
		OrderStatus *string  `json:"order_status"`
		Price       *float64 `json:"price"`
	} `json:"purchases"`
}

func (i listItem) price() *float64 {
	if len(i.Purchases) == 0 {
		return nil
	}

	return i.Purchases[0].Price
}

func itemByID(items []listItem, id string) *listItem {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}

	return nil
}

var _ = Describe("Listing switches", func() {
	var (
		resp       *http.Response
		client     *api.SwitchesClient
		ownerID    string
		ownerToken string
	)

	BeforeEach(func(ctx SpecContext) {
		resp = nil
		client = api.NewSwitchesClient()

		// Derived from a freshly minted token, not a fixed fixture subject:
		// AuthToken mints a real subject from the oidc-testkit-signed token
		// rather than a fixed fixture subject string, so the owner used
		// to seed fixture data below must match whatever subject this
		// environment's token actually carries.
		var err error
		ownerToken, ownerID, err = api.NewAuthIdentity(ctx)
		Expect(err).NotTo(HaveOccurred())
	})

	itemIDs := func(r *http.Response) []string {
		var page struct {
			Items []listItem `json:"items"`
		}
		Expect(json.NewDecoder(r.Body).Decode(&page)).To(Succeed())
		ids := make([]string, len(page.Items))
		for i, item := range page.Items {
			ids[i] = item.ID
		}
		return ids
	}

	Context("given the owner has switches at every visibility tier", func() {
		var publicID, authenticatedID, privateID string

		BeforeEach(func(ctx SpecContext) {
			// Fresh IDs per spec run, not fixed literals: AuthToken is a
			// shared, real emulator identity in CI (provisioning one per
			// spec isn't practical there), so specs sharing an identity
			// must namespace their own data to stay collision-proof under
			// concurrent/out-of-order runs.
			publicID = "public-switch-" + uuid.NewString()
			authenticatedID = "authenticated-switch-" + uuid.NewString()
			privateID = "private-switch-" + uuid.NewString()

			Expect(db.SeedSwitch(ctx, ownerID, publicID, "public")).To(Succeed())
			Expect(db.SeedSwitch(ctx, ownerID, authenticatedID, "authenticated")).To(Succeed())
			Expect(db.SeedSwitch(ctx, ownerID, privateID, "private")).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteSwitch(ctx, ownerID, publicID)).To(Succeed())
			Expect(db.DeleteSwitch(ctx, ownerID, authenticatedID)).To(Succeed())
			Expect(db.DeleteSwitch(ctx, ownerID, privateID)).To(Succeed())
		})

		Context("given the caller is the owner", func() {
			When("listing switches", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.List(ctx, ownerID, ownerToken, -1)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns switches at every visibility tier", func() {
					By("returning 200 OK")
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					body, err := io.ReadAll(resp.Body)
					Expect(err).NotTo(HaveOccurred())

					var page struct {
						Items []listItem `json:"items"`
					}
					Expect(json.Unmarshal(body, &page)).To(Succeed())
					ids := make([]string, len(page.Items))
					for i, item := range page.Items {
						ids[i] = item.ID
					}

					By("including all three seeded switches")
					Expect(ids).To(ContainElements(publicID, authenticatedID, privateID))

					By("returning each switch in full, the same shape as a single-item GET")
					publicItem := itemByID(page.Items, publicID)
					Expect(publicItem).NotTo(BeNil())
					Expect(publicItem.Brand).NotTo(BeEmpty())
					Expect(publicItem.Purchases).To(HaveLen(1))
					Expect(publicItem.Purchases[0].Vendor).NotTo(BeNil())
					Expect(publicItem.Purchases[0].OrderStatus).To(HaveValue(Equal("Delivered")))

					By("omitting the image key on image-less switches rather than emitting it as null")
					var raw struct {
						Items []map[string]json.RawMessage `json:"items"`
					}
					Expect(json.Unmarshal(body, &raw)).To(Succeed())
					for _, item := range raw.Items {
						Expect(item).NotTo(HaveKey("image"))
					}

					By("including the price, since the caller owns these switches")
					Expect(publicItem.price()).NotTo(BeNil())
					Expect(*publicItem.price()).To(BeNumerically("==", 0.35))
				})
			})
		})

		Context("given the caller is anonymous", func() {
			When("listing switches", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.List(ctx, ownerID, "", -1)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns only the public switch", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))
					ids := itemIDs(resp)
					Expect(ids).To(ContainElement(publicID))
					Expect(ids).NotTo(ContainElement(authenticatedID))
					Expect(ids).NotTo(ContainElement(privateID))
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

			When("listing switches", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.List(ctx, ownerID, token, -1)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the public and authenticated switches, but not the private one", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var page struct {
						Items []listItem `json:"items"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&page)).To(Succeed())
					ids := make([]string, len(page.Items))
					for i, item := range page.Items {
						ids[i] = item.ID
					}
					Expect(ids).To(ContainElements(publicID, authenticatedID))
					Expect(ids).NotTo(ContainElement(privateID))

					By("omitting the price, since the caller doesn't own these switches")
					item := itemByID(page.Items, publicID)
					Expect(item).NotTo(BeNil())
					Expect(item.price()).To(BeNil())
				})
			})
		})
	})

	Context("given the owner has a switch with a purchase price and show_price_to_me is false", func() {
		var switchID, profileUsername string

		BeforeEach(func(ctx SpecContext) {
			switchID = "priced-switch-" + uuid.NewString()
			profileUsername = "u" + uuid.NewString()[:8]
			Expect(db.SeedSwitch(ctx, ownerID, switchID, "public")).To(Succeed())
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
			When("listing switches", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.List(ctx, ownerID, ownerToken, -1)
					Expect(err).NotTo(HaveOccurred())
				})

				It("omits the price", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var page struct {
						Items []listItem `json:"items"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&page)).To(Succeed())
					item := itemByID(page.Items, switchID)
					Expect(item).NotTo(BeNil())
					Expect(item.price()).To(BeNil())
				})
			})
		})
	})

	Context("given the owner has a switch with a purchase price and show_price_to_others is true", func() {
		var switchID, profileUsername string

		BeforeEach(func(ctx SpecContext) {
			switchID = "priced-switch-" + uuid.NewString()
			profileUsername = "u" + uuid.NewString()[:8]
			Expect(db.SeedSwitch(ctx, ownerID, switchID, "public")).To(Succeed())
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

			When("listing switches", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.List(ctx, ownerID, token, -1)
					Expect(err).NotTo(HaveOccurred())
				})

				It("includes the price", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var page struct {
						Items []listItem `json:"items"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&page)).To(Succeed())
					item := itemByID(page.Items, switchID)
					Expect(item).NotTo(BeNil())
					Expect(item.price()).NotTo(BeNil())
					Expect(*item.price()).To(BeNumerically("==", 0.35))
				})
			})
		})
	})

	DescribeTable("given an invalid limit",
		func(ctx SpecContext, limit int) {
			var err error
			resp, err = client.List(ctx, ownerID, ownerToken, limit)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
		},
		Entry("below the minimum", 0),
		Entry("above the maximum", 101),
	)

	Context("given a non-numeric limit", func() {
		When("listing switches", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.ListWithRawLimit(ctx, ownerID, ownerToken, "abc")
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 400 with a problem+json body", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})
	})
})
