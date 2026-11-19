package builds_test

import (
	"encoding/json"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support/api"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

type ecBuildOutput struct {
	Build struct {
		ID        string `json:"id"`
		BuildType string `json:"build_type"`
		ECParts   []struct {
			ECPart string `json:"ec_part"`
			Count  int    `json:"count"`
		} `json:"ec_parts"`
	} `json:"build"`
}

func decodeECBuild(result *sdkmcp.CallToolResult) ecBuildOutput {
	GinkgoHelper()
	raw, err := json.Marshal(result.StructuredContent)
	Expect(err).NotTo(HaveOccurred())
	var out ecBuildOutput
	Expect(json.Unmarshal(raw, &out)).To(Succeed())
	return out
}

var _ = Describe("EC builds over MCP", func() {
	var (
		client     *api.MCPClient
		result     *sdkmcp.CallToolResult
		err        error
		ownerID    string
		keyboardID string
		partID     string
		createdID  string
	)

	BeforeEach(func(ctx SpecContext) {
		result = nil
		err = nil
		createdID = ""
		client, ownerID = api.NewAuthenticatedMCPClient(ctx)

		keyboardID = "ec-build-keyboard-" + uuid.NewString()
		partID = "ec-build-domes-" + uuid.NewString()
		Expect(db.SeedKeyboard(ctx, ownerID, keyboardID, "private")).To(Succeed())
		Expect(db.SeedECPart(ctx, ownerID, partID, "private")).To(Succeed())
	})

	AfterEach(func(ctx SpecContext) {
		if createdID != "" {
			del, delErr := client.CallTool(ctx, "delete_build", map[string]any{"build_id": createdID})
			Expect(delErr).NotTo(HaveOccurred())
			Expect(del.IsError).To(BeFalse())
		}
		Expect(db.DeleteECPart(ctx, ownerID, partID)).To(Succeed())
		Expect(db.DeleteKeyboard(ctx, ownerID, keyboardID)).To(Succeed())
	})

	Context("given an EC build listing the caller's EC part", func() {
		When("the create_build tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "create_build", map[string]any{
					"keyboard": keyboardID, "build_type": "EC", "visibility": "private",
					"ec_parts": []map[string]any{{"ec_part": partID, "count": 60}},
				})
				if err == nil && !result.IsError {
					createdID = decodeECBuild(result).Build.ID
				}
			})

			It("stores the build type and EC parts", func(ctx SpecContext) {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeFalse())

				got, getErr := client.CallTool(ctx, "get_build", map[string]any{"build_id": createdID})
				Expect(getErr).NotTo(HaveOccurred())
				Expect(got.IsError).To(BeFalse())
				out := decodeECBuild(got)
				Expect(out.Build.BuildType).To(Equal("EC"))
				Expect(out.Build.ECParts).To(HaveLen(1))
				Expect(out.Build.ECParts[0].ECPart).To(Equal(partID))
				Expect(out.Build.ECParts[0].Count).To(Equal(60))
			})
		})
	})

	Context("given an EC build that also lists switches", func() {
		When("the create_build tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "create_build", map[string]any{
					"keyboard": keyboardID, "build_type": "EC", "visibility": "private",
					"switches": []map[string]any{{"switch": "sw-" + uuid.NewString(), "count": 60}},
				})
			})

			It("returns a tool error naming switches", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeTrue())
				Expect(result.Content[0].(*sdkmcp.TextContent).Text).To(ContainSubstring("switches"))
			})
		})
	})
})
