package lookups_test

import (
	"encoding/json"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
)

var _ = Describe("Listing categories", func() {
	var (
		resp   *http.Response
		client *api.LookupsClient
	)

	BeforeEach(func() {
		resp = nil
		client = api.NewLookupsClient()
	})

	When("listing categories", func() {
		BeforeEach(func(ctx SpecContext) {
			var err error
			resp, err = client.ListCategories(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		It("succeeds and returns exactly the known categories", func() {
			By("returning 200 OK")
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			By("returning every known category name, and no others")
			var categories []string
			Expect(json.NewDecoder(resp.Body).Decode(&categories)).To(Succeed())
			Expect(categories).To(ConsistOf(
				"build_case_mount_type",
				"build_durometer",
				"build_stabilizer",
				"build_stabilizer_mount_type",
				"build_type",
				"ec_dome_material",
				"ec_ring_material",
				"image_content_type",
				"keyboard_case_material",
				"keyboard_layout",
				"keyboard_pcb_assembly_type",
				"keyboard_pcb_connectivity_type",
				"keyboard_pcb_firmware",
				"keyboard_plate_material",
				"keyboard_size",
				"keyboard_weight_material",
				"keycap_material",
				"keycap_profile",
				"order_status",
				"switch_material",
				"switch_spring_material",
				"switch_type",
				"vendor",
				"visibility",
			))
		})
	})
})
