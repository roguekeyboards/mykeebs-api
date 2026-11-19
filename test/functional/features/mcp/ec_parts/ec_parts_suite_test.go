package ec_parts_test

import (
	"encoding/json"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestECParts(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MCP EC Parts Suite")
}

type ecPart struct {
	ID         string `json:"id"`
	Brand      string `json:"brand"`
	PartType   string `json:"part_type"`
	Visibility string `json:"visibility"`
	Rings      *struct {
		Material  *string  `json:"material"`
		Thickness *float64 `json:"thickness"`
	} `json:"rings"`
	Housings *struct {
		Size *string `json:"size"`
	} `json:"housings"`
	Purchase *struct {
		Vendor *string `json:"vendor"`
	} `json:"purchase"`
}

func decodePart(result *sdkmcp.CallToolResult) ecPart {
	GinkgoHelper()

	raw, err := json.Marshal(result.StructuredContent)
	Expect(err).NotTo(HaveOccurred())

	var out struct {
		ECPart ecPart `json:"ec_part"`
	}
	Expect(json.Unmarshal(raw, &out)).To(Succeed())
	return out.ECPart
}

func decodePartIDs(result *sdkmcp.CallToolResult) []string {
	GinkgoHelper()

	raw, err := json.Marshal(result.StructuredContent)
	Expect(err).NotTo(HaveOccurred())

	var out struct {
		ECParts []ecPart `json:"ec_parts"`
	}
	Expect(json.Unmarshal(raw, &out)).To(Succeed())
	ids := make([]string, len(out.ECParts))
	for i, p := range out.ECParts {
		ids[i] = p.ID
	}
	return ids
}
