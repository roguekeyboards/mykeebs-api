package switches_test

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("A switch's available quantity", func() {
	var (
		resp       *http.Response
		switches   *api.SwitchesClient
		builds     *api.BuildsClient
		ownerID    string
		ownerToken string
		keyboardID string
		switchID   string
	)

	createID := func(r *http.Response, err error) string {
		GinkgoHelper()
		Expect(err).NotTo(HaveOccurred())
		Expect(r.StatusCode).To(Equal(http.StatusCreated))
		var created struct {
			ID string `json:"id"`
		}
		Expect(json.NewDecoder(r.Body).Decode(&created)).To(Succeed())
		return created.ID
	}

	createBuild := func(ctx SpecContext, buildDate, visibility string, count int) {
		GinkgoHelper()
		createID(builds.Create(ctx, ownerID, ownerToken,
			`{"keyboard":"`+keyboardID+`","build_date":"`+buildDate+`","visibility":"`+visibility+`",`+
				`"switches":[{"switch":"`+switchID+`","count":`+strconv.Itoa(count)+`}]}`))
	}

	BeforeEach(func(ctx SpecContext) {
		resp = nil
		switches = api.NewSwitchesClient()
		builds = api.NewBuildsClient()

		var err error
		ownerToken, ownerID, err = api.NewAuthIdentity(ctx)
		Expect(err).NotTo(HaveOccurred())

		keyboardID = "available-keyboard-" + uuid.NewString()
		Expect(db.SeedKeyboard(ctx, ownerID, keyboardID, "public")).To(Succeed())

		switchID = createID(switches.Create(ctx, ownerID, ownerToken,
			`{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"public","purchases":[`+
				`{"vendor":"NovelKeys","order_status":"Delivered","quantity":90},`+
				`{"vendor":"NovelKeys","order_status":"Shipped","quantity":70}]}`))
	})

	AfterEach(func(ctx SpecContext) {
		r, err := switches.DeleteWithOnDelete(ctx, ownerID, switchID, ownerToken, "cascade")
		Expect(err).NotTo(HaveOccurred())
		Expect(r.StatusCode).To(Equal(http.StatusOK))
		Expect(db.DeleteKeyboard(ctx, ownerID, keyboardID)).To(Succeed())
	})

	Context("given a public build of the keyboard and a newer private one", func() {
		BeforeEach(func(ctx SpecContext) {
			createBuild(ctx, "2026-01-01", "public", 80)
			createBuild(ctx, "2026-06-01", "private", 65)
		})

		Context("given the caller is anonymous", func() {
			When("getting the switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = switches.Get(ctx, ownerID, switchID, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("subtracts only the newest build from the Delivered purchases", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var got struct {
						AvailableQuantity int `json:"available_quantity"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.AvailableQuantity).To(Equal(25))
				})
			})

			When("listing the switches", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = switches.List(ctx, ownerID, "", 100)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the same available quantity", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var got struct {
						Items []struct {
							ID                string `json:"id"`
							AvailableQuantity int    `json:"available_quantity"`
						} `json:"items"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.Items).To(HaveLen(1))
					Expect(got.Items[0].ID).To(Equal(switchID))
					Expect(got.Items[0].AvailableQuantity).To(Equal(25))
				})
			})
		})
	})
})
