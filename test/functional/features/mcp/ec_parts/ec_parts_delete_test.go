package ec_parts_test

import (
	"encoding/json"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

var _ = Describe("Deleting an EC part over MCP", func() {
	var (
		client  *api.MCPClient
		result  *sdkmcp.CallToolResult
		err     error
		ownerID string
		partID  string
	)

	BeforeEach(func(ctx SpecContext) {
		result = nil
		err = nil
		client, ownerID = api.NewAuthenticatedMCPClient(ctx)
		partID = "delete-ec-part-" + uuid.NewString()
		Expect(db.SeedECPart(ctx, ownerID, partID, "private")).To(Succeed())
		DeferCleanup(func(ctx SpecContext) {
			Expect(db.DeleteECPart(ctx, ownerID, partID)).To(Succeed())
		})
	})

	Context("given no build uses the part", func() {
		When("delete_ec_part is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "delete_ec_part", map[string]any{"ec_part_id": partID})
			})

			It("deletes the part", func(ctx SpecContext) {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeFalse())

				got, getErr := client.CallTool(ctx, "get_ec_part", map[string]any{"ec_part_id": partID})
				Expect(getErr).NotTo(HaveOccurred())
				Expect(got.IsError).To(BeTrue())
			})
		})
	})

	Context("given an EC build uses the part", func() {
		var buildID string

		BeforeEach(func(ctx SpecContext) {
			keyboardID := "delete-ec-part-keyboard-" + uuid.NewString()
			Expect(db.SeedKeyboard(ctx, ownerID, keyboardID, "private")).To(Succeed())
			DeferCleanup(func(ctx SpecContext) {
				Expect(db.DeleteKeyboard(ctx, ownerID, keyboardID)).To(Succeed())
			})

			created, createErr := client.CallTool(ctx, "create_build", map[string]any{
				"keyboard": keyboardID, "build_type": "EC", "visibility": "private",
				"ec_parts": []map[string]any{{"ec_part": partID, "count": 60}},
			})
			Expect(createErr).NotTo(HaveOccurred())
			Expect(created.IsError).To(BeFalse())
			raw, marshalErr := json.Marshal(created.StructuredContent)
			Expect(marshalErr).NotTo(HaveOccurred())
			var out struct {
				Build struct {
					ID string `json:"id"`
				} `json:"build"`
			}
			Expect(json.Unmarshal(raw, &out)).To(Succeed())
			buildID = out.Build.ID
			DeferCleanup(func(ctx SpecContext) {
				del, delErr := client.CallTool(ctx, "delete_build", map[string]any{"build_id": buildID})
				Expect(delErr).NotTo(HaveOccurred())
				Expect(del.IsError).To(BeFalse())
			})
		})

		When("delete_ec_part is called with on_delete cascade", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "delete_ec_part", map[string]any{"ec_part_id": partID, "on_delete": "cascade"})
			})

			It("deletes the part and reports the deleted build", func(ctx SpecContext) {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeFalse())
				raw, marshalErr := json.Marshal(result.StructuredContent)
				Expect(marshalErr).NotTo(HaveOccurred())
				var out struct {
					DeletedBuildIDs []string `json:"deleted_build_ids"`
				}
				Expect(json.Unmarshal(raw, &out)).To(Succeed())
				Expect(out.DeletedBuildIDs).To(ConsistOf(buildID))

				got, getErr := client.CallTool(ctx, "get_ec_part", map[string]any{"ec_part_id": partID})
				Expect(getErr).NotTo(HaveOccurred())
				Expect(got.IsError).To(BeTrue())
			})
		})
	})
})
