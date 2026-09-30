package websocket

import "testing"

func TestDecodeLocation(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		valid         bool
	}{
		{"origin", `{"latitude":0,"longitude":0}`, true},
		{"bounds", `{"latitude":-90,"longitude":180}`, true},
		{"missing latitude", `{"longitude":0}`, false},
		{"missing longitude", `{"latitude":0}`, false},
		{"null", `{"latitude":null,"longitude":0}`, false},
		{"range", `{"latitude":91,"longitude":0}`, false},
		{"overflow", `{"latitude":1e400,"longitude":0}`, false},
		{"string", `{"latitude":"10","longitude":0}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := decodeLocation([]byte(tc.payload))
			if ok != tc.valid {
				t.Fatalf("valid=%v, want %v", ok, tc.valid)
			}
		})
	}
}
