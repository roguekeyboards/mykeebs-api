package switches_test

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("Listing switches by available quantity over MCP", func() {
	var (
		client     *api.MCPClient
		result     *sdkmcp.CallToolResult
		err        error
		switches   *api.SwitchesClient
		ownerID    string
		ownerToken string
		keyboardID string
		usedID     string
		freeID     string
	)

	createSwitch := func(ctx SpecContext) string {
		GinkgoHelper()
		r, err := switches.Create(ctx, ownerID, ownerToken,
			`{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private",`+
				`"purchases":[{"vendor":"NovelKeys","order_status":"Delivered","quantity":90}]}`)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.StatusCode).To(Equal(http.StatusCreated))
		var created struct {
			ID string `json:"id"`
		}
		Expect(json.NewDecoder(r.Body).Decode(&created)).To(Succeed())
		return created.ID
	}

	BeforeEach(func(ctx SpecContext) {
		result = nil
		switches = api.NewSwitchesClient()

		ownerToken, ownerID, err = api.NewAuthIdentity(ctx)
		Expect(err).NotTo(HaveOccurred())
		client = api.NewMCPClient(support.BaseURL()+"/mcp", ownerToken)

		keyboardID = "available-keyboard-" + uuid.NewString()
		Expect(db.SeedKeyboard(ctx, ownerID, keyboardID, "private")).To(Succeed())
		usedID = createSwitch(ctx)
		freeID = createSwitch(ctx)
	})

	AfterEach(func(ctx SpecContext) {
		for _, id := range []string{usedID, freeID} {
			r, err := switches.DeleteWithOnDelete(ctx, ownerID, id, ownerToken, "cascade")
			Expect(err).NotTo(HaveOccurred())
			Expect(r.StatusCode).To(Equal(http.StatusOK))
		}
		Expect(db.DeleteKeyboard(ctx, ownerID, keyboardID)).To(Succeed())
	})

	Context("given a build uses 65 of one switch", func() {
		BeforeEach(func(ctx SpecContext) {
			r, err := api.NewBuildsClient().Create(ctx, ownerID, ownerToken,
				`{"keyboard":"`+keyboardID+`","visibility":"private","switches":[{"switch":"`+usedID+`","count":65}]}`)
			Expect(err).NotTo(HaveOccurred())
			Expect(r.StatusCode).To(Equal(http.StatusCreated))
		})

		When("the list_switches tool is called with min_available_quantity", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "list_switches", map[string]any{"min_available_quantity": 30, "limit": 100})
			})

			It("returns only the switch with enough available", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeFalse())

				raw, err := json.Marshal(result.StructuredContent)
				Expect(err).NotTo(HaveOccurred())
				var out struct {
					Switches []struct {
						ID                string `json:"id"`
						AvailableQuantity int    `json:"available_quantity"`
					} `json:"switches"`
				}
				Expect(json.Unmarshal(raw, &out)).To(Succeed())
				Expect(out.Switches).To(HaveLen(1))
				Expect(out.Switches[0].ID).To(Equal(freeID))
				Expect(out.Switches[0].AvailableQuantity).To(Equal(90))
			})
		})
	})
})
