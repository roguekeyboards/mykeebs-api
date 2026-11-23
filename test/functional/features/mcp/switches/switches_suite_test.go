package switches_test

import (
	"encoding/json"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// approvedType/approvedStem/approvedSpringMaterial/approvedVendor are real
// values from internal/lookup/data/.
const (
	approvedType             = "Linear"
	approvedStem             = "POM"
	approvedSpringMaterial   = "Stainless Steel"
	approvedVendor           = "Amazon"
	approvedImageContentType = "image/webp"
)

func TestSwitches(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MCP Switches Suite")
}

type getOutput struct {
	Switch struct {
		ID         string `json:"id"`
		Brand      string `json:"brand"`
		Name       string `json:"name"`
		Type       string `json:"type"`
		Visibility string `json:"visibility"`
		HasImage   bool   `json:"has_image"`
		Spring     *struct {
			Length *float64 `json:"length"`
		} `json:"spring"`
		Purchase *struct {
			Vendor *string  `json:"vendor"`
			Price  *float64 `json:"price"`
		} `json:"purchase"`
	} `json:"switch"`
}

func decodeGetOutput(result *sdkmcp.CallToolResult) getOutput {
	GinkgoHelper()

	raw, err := json.Marshal(result.StructuredContent)
	Expect(err).NotTo(HaveOccurred())

	var out getOutput
	Expect(json.Unmarshal(raw, &out)).To(Succeed())

	return out
}

type setImageOutput struct {
	UploadURL string `json:"upload_url"`
}

func decodeSetImageOutput(result *sdkmcp.CallToolResult) setImageOutput {
	GinkgoHelper()

	raw, err := json.Marshal(result.StructuredContent)
	Expect(err).NotTo(HaveOccurred())

	var out setImageOutput
	Expect(json.Unmarshal(raw, &out)).To(Succeed())

	return out
}
