package lookups_test

import (
	"encoding/json"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
)

var _ = Describe("Listing lookup categories", func() {
	var (
		client *api.MCPClient
		result *sdkmcp.CallToolResult
		err    error
	)

	BeforeEach(func() {
		result = nil
		err = nil
	})

	Context("given a valid bearer token", func() {
		BeforeEach(func(ctx SpecContext) {
			token, _, tokenErr := api.NewAuthIdentity(ctx)
			Expect(tokenErr).NotTo(HaveOccurred())
			client = api.NewMCPClient(support.BaseURL()+"/mcp", token)
		})

		When("the list_lookups tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "list_lookups", map[string]any{})
			})

			It("succeeds and returns exactly the known categories", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeFalse())

				raw, err := json.Marshal(result.StructuredContent)
				Expect(err).NotTo(HaveOccurred())

				var out struct {
					Categories []string `json:"categories"`
				}
				Expect(json.Unmarshal(raw, &out)).To(Succeed())
				Expect(out.Categories).To(ConsistOf(
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

	Context("given no bearer token", func() {
		BeforeEach(func() {
			client = api.NewMCPClient(support.BaseURL()+"/mcp", "")
		})

		When("the list_lookups tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "list_lookups", map[string]any{})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
