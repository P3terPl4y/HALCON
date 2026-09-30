package models

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONNeverExposesCredentials(t *testing.T) {
	for _, model := range []any{User{Password: "SECRET"}, Halcon{Token: "SECRET"}, HalconAssignment{User: &User{Password: "SECRET"}, Halcon: &Halcon{Token: "SECRET"}}} {
		data, err := json.Marshal(model)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "SECRET") || strings.Contains(string(data), `"Password"`) || strings.Contains(string(data), `"Token"`) {
			t.Fatalf("credential leaked: %s", data)
		}
	}
}
