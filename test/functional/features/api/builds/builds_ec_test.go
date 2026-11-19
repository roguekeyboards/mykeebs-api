package builds_test

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

type ecBuild struct {
	ID        string `json:"id"`
	BuildType string `json:"build_type"`
	ECParts   []struct {
		Count  int `json:"count"`
		ECPart struct {
			ID       string `json:"id"`
			PartType string `json:"part_type"`
		} `json:"ec_part"`
	} `json:"ec_parts"`
	InvalidParams []struct {
		Name string `json:"name"`
	} `json:"invalid_params"`
}

var _ = Describe("EC builds", func() {
	var (
		resp       *http.Response
		client     *api.BuildsClient
		ownerID    string
		ownerToken string
		keyboardID string
		domesID    string
		slidersID  string
		createdID  string
	)

	BeforeEach(func(ctx SpecContext) {
		resp = nil
		client = api.NewBuildsClient()
		createdID = ""

		var err error
		ownerToken, ownerID, err = api.NewAuthIdentity(ctx)
		Expect(err).NotTo(HaveOccurred())

		keyboardID = "ec-build-keyboard-" + uuid.NewString()
		domesID = "ec-build-domes-" + uuid.NewString()
		slidersID = "ec-build-sliders-" + uuid.NewString()
		Expect(db.SeedKeyboard(ctx, ownerID, keyboardID, "private")).To(Succeed())
		Expect(db.SeedECPartOfType(ctx, ownerID, domesID, "private", "domes")).To(Succeed())
		Expect(db.SeedECPartOfType(ctx, ownerID, slidersID, "private", "sliders")).To(Succeed())
	})

	AfterEach(func(ctx SpecContext) {
		if createdID != "" {
			delResp, err := client.Delete(ctx, ownerID, createdID, ownerToken)
			Expect(err).NotTo(HaveOccurred())
			Expect(delResp.StatusCode).To(Equal(http.StatusNoContent))
		}
		Expect(db.DeleteECPart(ctx, ownerID, domesID)).To(Succeed())
		Expect(db.DeleteECPart(ctx, ownerID, slidersID)).To(Succeed())
		Expect(db.DeleteKeyboard(ctx, ownerID, keyboardID)).To(Succeed())
	})

	create := func(ctx SpecContext, body string) ecBuild {
		GinkgoHelper()
		var err error
		resp, err = client.Create(ctx, ownerID, ownerToken, body)
		Expect(err).NotTo(HaveOccurred())
		var got ecBuild
		Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
		if resp.StatusCode == http.StatusCreated {
			createdID = got.ID
		}
		return got
	}

	Context("given the caller is the owner", func() {
		Context("given an EC build listing the caller's EC parts", func() {
			When("creating the build", func() {
				BeforeEach(func(ctx SpecContext) {
					create(ctx, `{"keyboard":"`+keyboardID+`","build_type":"EC","visibility":"private",`+
						`"ec_parts":[{"ec_part":"`+domesID+`","count":60},{"ec_part":"`+slidersID+`","count":60}]}`)
				})

				It("stores the build with its EC parts resolved", func(ctx SpecContext) {
					Expect(resp.StatusCode).To(Equal(http.StatusCreated))

					getResp, err := client.Get(ctx, ownerID, createdID, ownerToken)
					Expect(err).NotTo(HaveOccurred())
					var got ecBuild
					Expect(json.NewDecoder(getResp.Body).Decode(&got)).To(Succeed())
					Expect(got.BuildType).To(Equal("EC"))
					Expect(got.ECParts).To(HaveLen(2))
					Expect(got.ECParts[0].ECPart.ID).To(Equal(domesID))
					Expect(got.ECParts[0].ECPart.PartType).To(Equal("domes"))
					Expect(got.ECParts[0].Count).To(Equal(60))
					Expect(got.ECParts[1].ECPart.ID).To(Equal(slidersID))
				})
			})
		})

		Context("given an MX build listing EC parts", func() {
			When("creating the build", func() {
				var got ecBuild

				BeforeEach(func(ctx SpecContext) {
					got = create(ctx, `{"keyboard":"`+keyboardID+`","build_type":"MX","visibility":"private",`+
						`"ec_parts":[{"ec_part":"`+domesID+`","count":60}]}`)
				})

				It("returns 400 naming ec_parts", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
					Expect(got.InvalidParams).To(ContainElement(HaveField("Name", "ec_parts")))
				})
			})
		})

		Context("given an EC build listing an EC part that isn't in the caller's collection", func() {
			When("creating the build", func() {
				var got ecBuild

				BeforeEach(func(ctx SpecContext) {
					got = create(ctx, `{"keyboard":"`+keyboardID+`","build_type":"EC","visibility":"private",`+
						`"ec_parts":[{"ec_part":"missing-`+uuid.NewString()+`","count":60}]}`)
				})

				It("returns 400 naming the entry", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
					Expect(got.InvalidParams).To(ContainElement(HaveField("Name", "ec_parts[0].ec_part")))
				})
			})
		})
	})

	Context("given a build stored before build types existed", func() {
		var buildID string

		BeforeEach(func(ctx SpecContext) {
			buildID = "legacy-build-" + uuid.NewString()
			Expect(db.SeedBuild(ctx, ownerID, buildID, keyboardID, "private")).To(Succeed())
			DeferCleanup(func(ctx SpecContext) {
				Expect(db.DeleteBuild(ctx, ownerID, buildID, keyboardID)).To(Succeed())
			})
		})

		When("the owner gets it", func() {
			var got ecBuild

			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Get(ctx, ownerID, buildID, ownerToken)
				Expect(err).NotTo(HaveOccurred())
				Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
			})

			It("reports it as an MX build", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				Expect(got.BuildType).To(Equal("MX"))
			})
		})
	})
})
