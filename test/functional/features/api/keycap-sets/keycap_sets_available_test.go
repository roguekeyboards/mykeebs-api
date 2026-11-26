package keycapsets_test

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("Whether a keycap set is available", func() {
	var (
		resp       *http.Response
		client     *api.KeycapSetsClient
		ownerID    string
		ownerToken string
		keyboardID string
		setID      string
		baseKitID  string
	)

	createdID := func(r *http.Response, err error, field string) string {
		GinkgoHelper()
		Expect(err).NotTo(HaveOccurred())
		Expect(r.StatusCode).To(Equal(http.StatusCreated))
		var created map[string]any
		Expect(json.NewDecoder(r.Body).Decode(&created)).To(Succeed())
		id, _ := created[field].(string)
		return id
	}

	available := func() bool {
		GinkgoHelper()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		var got struct {
			Available *bool `json:"available"`
		}
		Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
		Expect(got.Available).NotTo(BeNil())
		return *got.Available
	}

	BeforeEach(func(ctx SpecContext) {
		resp = nil
		client = api.NewKeycapSetsClient()

		var err error
		ownerToken, ownerID, err = api.NewAuthIdentity(ctx)
		Expect(err).NotTo(HaveOccurred())

		keyboardID = "available-keyboard-" + uuid.NewString()
		Expect(db.SeedKeyboard(ctx, ownerID, keyboardID, "public")).To(Succeed())

		r, err := client.Create(ctx, ownerID, ownerToken, `{"brand":"GMK","name":"Olivia","visibility":"public"}`)
		setID = createdID(r, err, "id")
		r, err = client.CreateKit(ctx, ownerID, setID, ownerToken, `{"name":"Base","purchase":{"order_status":"Delivered"}}`)
		baseKitID = createdID(r, err, "kit_id")
		r, err = client.CreateKit(ctx, ownerID, setID, ownerToken, `{"name":"Spacebars","purchase":{"order_status":"Shipped"}}`)
		createdID(r, err, "kit_id")
	})

	AfterEach(func(ctx SpecContext) {
		r, err := api.NewKeyboardsClient().DeleteWithOnDelete(ctx, ownerID, keyboardID, ownerToken, "cascade")
		Expect(err).NotTo(HaveOccurred())
		Expect(r.StatusCode).To(Equal(http.StatusOK))
		r, err = client.Delete(ctx, ownerID, setID, ownerToken)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.StatusCode).To(Equal(http.StatusNoContent))
	})

	Context("given a delivered base kit and an add-on still shipping", func() {
		Context("given the caller is anonymous", func() {
			When("getting the keycap set", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, setID, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("reports the set available", func() {
					Expect(available()).To(BeTrue())
				})
			})
		})
	})

	Context("given the base kit is on a private build that's its keyboard's newest", func() {
		BeforeEach(func(ctx SpecContext) {
			builds := api.NewBuildsClient()
			for _, body := range []string{
				`{"keyboard":"` + keyboardID + `","build_date":"2026-01-01","visibility":"public"}`,
				`{"keyboard":"` + keyboardID + `","build_date":"2026-06-01","visibility":"private",` +
					`"keycap_kits":[{"keycap_set":"` + setID + `","kit":"` + baseKitID + `"}]}`,
			} {
				r, err := builds.Create(ctx, ownerID, ownerToken, body)
				createdID(r, err, "id")
			}
		})

		Context("given the caller is anonymous", func() {
			When("listing the keycap sets", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.List(ctx, ownerID, "", 100)
					Expect(err).NotTo(HaveOccurred())
				})

				It("reports the set unavailable", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))
					var got struct {
						Items []struct {
							ID        string `json:"id"`
							Available bool   `json:"available"`
						} `json:"items"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.Items).To(HaveLen(1))
					Expect(got.Items[0].ID).To(Equal(setID))
					Expect(got.Items[0].Available).To(BeFalse())
				})
			})
		})
	})
})
