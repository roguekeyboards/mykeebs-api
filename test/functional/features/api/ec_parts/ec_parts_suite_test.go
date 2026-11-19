package ec_parts_test

import (
	"encoding/json"
	"net/http"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestECParts(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "REST EC Parts Suite")
}

type ecPart struct {
	ID         string  `json:"id"`
	Brand      string  `json:"brand"`
	PartType   string  `json:"part_type"`
	Visibility *string `json:"visibility"`
	Domes      *struct {
		Weight   *float64 `json:"weight"`
		Material *string  `json:"material"`
	} `json:"domes"`
	Sliders *struct {
		Size *string `json:"size"`
		Stem *string `json:"stem"`
	} `json:"sliders"`
	Purchase *struct {
		Vendor *string  `json:"vendor"`
		Price  *float64 `json:"price"`
	} `json:"purchase"`
	Image *struct {
		URL    *string `json:"url"`
		Status *string `json:"status"`
	} `json:"image"`
}

func decodePart(r *http.Response) ecPart {
	GinkgoHelper()

	var got ecPart
	Expect(json.NewDecoder(r.Body).Decode(&got)).To(Succeed())
	return got
}

func decodePartIDs(r *http.Response) []string {
	GinkgoHelper()

	var page struct {
		Items []ecPart `json:"items"`
	}
	Expect(json.NewDecoder(r.Body).Decode(&page)).To(Succeed())
	ids := make([]string, len(page.Items))
	for i, p := range page.Items {
		ids[i] = p.ID
	}
	return ids
}
