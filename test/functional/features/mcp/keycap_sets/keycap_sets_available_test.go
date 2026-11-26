package keycapsets_test

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

var _ = Describe("Listing available keycap sets over MCP", func() {
	var (
		client     *api.MCPClient
		result     *sdkmcp.CallToolResult
		err        error
		sets       *api.KeycapSetsClient
		ownerID    string
		ownerToken string
		keyboardID string
		usedID     string
		usedKitID  string
		freeID     string
	)

	created := func(r *http.Response, err error, field string) string {
		GinkgoHelper()
		Expect(err).NotTo(HaveOccurred())
		Expect(r.StatusCode).To(Equal(http.StatusCreated))
		var body map[string]any
		Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
		id, _ := body[field].(string)
		return id
	}

	createSet := func(ctx SpecContext) (setID, kitID string) {
		GinkgoHelper()
		r, err := sets.Create(ctx, ownerID, ownerToken, `{"brand":"GMK","name":"Olivia","visibility":"private"}`)
		setID = created(r, err, "id")
		r, err = sets.CreateKit(ctx, ownerID, setID, ownerToken, `{"name":"Base","purchase":{"order_status":"Delivered"}}`)
		return setID, created(r, err, "kit_id")
	}

	BeforeEach(func(ctx SpecContext) {
		result = nil
		sets = api.NewKeycapSetsClient()

		ownerToken, ownerID, err = api.NewAuthIdentity(ctx)
		Expect(err).NotTo(HaveOccurred())
		client = api.NewMCPClient(support.BaseURL()+"/mcp", ownerToken)

		keyboardID = "available-keyboard-" + uuid.NewString()
		Expect(db.SeedKeyboard(ctx, ownerID, keyboardID, "private")).To(Succeed())
		usedID, usedKitID = createSet(ctx)
		freeID, _ = createSet(ctx)
	})

	AfterEach(func(ctx SpecContext) {
		r, err := api.NewKeyboardsClient().DeleteWithOnDelete(ctx, ownerID, keyboardID, ownerToken, "cascade")
		Expect(err).NotTo(HaveOccurred())
		Expect(r.StatusCode).To(Equal(http.StatusOK))
		for _, id := range []string{usedID, freeID} {
			r, err := sets.Delete(ctx, ownerID, id, ownerToken)
			Expect(err).NotTo(HaveOccurred())
			Expect(r.StatusCode).To(Equal(http.StatusNoContent))
		}
	})

	Context("given a build uses one set's base kit", func() {
		BeforeEach(func(ctx SpecContext) {
			r, err := api.NewBuildsClient().Create(ctx, ownerID, ownerToken,
				`{"keyboard":"`+keyboardID+`","visibility":"private","keycap_kits":[{"keycap_set":"`+usedID+`","kit":"`+usedKitID+`"}]}`)
			created(r, err, "id")
		})

		When("the list_keycap_sets tool is called with available_only", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "list_keycap_sets", map[string]any{"available_only": true, "limit": 100})
			})

			It("returns only the set that's free", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeFalse())

				raw, err := json.Marshal(result.StructuredContent)
				Expect(err).NotTo(HaveOccurred())
				var out struct {
					KeycapSets []struct {
						ID        string `json:"id"`
						Available bool   `json:"available"`
					} `json:"keycap_sets"`
				}
				Expect(json.Unmarshal(raw, &out)).To(Succeed())
				Expect(out.KeycapSets).To(HaveLen(1))
				Expect(out.KeycapSets[0].ID).To(Equal(freeID))
				Expect(out.KeycapSets[0].Available).To(BeTrue())
			})
		})
	})
})
