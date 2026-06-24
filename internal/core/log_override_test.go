package core

import (
	"testing"

	"sing-box-ez/internal/singboxconfig"
)

func TestApplyLogOverride(t *testing.T) {
	input := []byte(`{"inbounds":[{"type":"mixed","listen":"127.0.0.1","listen_port":1080}],"outbounds":[{"type":"direct"}]}`)
	parser := singboxconfig.NewConfigParser()
	output, ok, err := parser.Override(input, func(tree map[string]any) bool {
		tree["log"] = map[string]any{
			"timestamp": true,
			"level":     "error",
		}
		return true
	})
	if err != nil {
		t.Fatalf("override failed: %v", err)
	}
	if !ok {
		t.Fatalf("override did not validate: %v", parser.Result().Errors)
	}
	if string(output) == "" {
		t.Fatal("override returned empty output")
	}
}
