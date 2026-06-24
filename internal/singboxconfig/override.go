package singboxconfig

import (
	"encoding/json"
	"fmt"
)

// Override parses the input config into an editable tree, invokes fn to modify
// it, validates the result against the schema, and returns the marshalled JSON.
//
// The callback receives the whole config tree as map[string]any and should edit
// it in place. It returns true when the edit is considered valid by the caller.
// Override returns ok=true only when the callback returned true and the schema
// validation found no unknown or removed fields. The detailed validation result
// is available through the parser's Result() method.
func (p *ConfigParser) Override(input []byte, fn func(tree map[string]any) bool) ([]byte, bool, error) {
	var tree map[string]any
	if err := json.Unmarshal(input, &tree); err != nil {
		return nil, false, fmt.Errorf("unmarshal config: %w", err)
	}
	if !fn(tree) {
		return nil, false, nil
	}
	output, err := json.Marshal(tree)
	if err != nil {
		return nil, false, fmt.Errorf("marshal config: %w", err)
	}
	if _, err := p.Parse(output); err != nil {
		return output, false, fmt.Errorf("validate config: %w", err)
	}
	res := p.Result()
	for _, e := range res.Errors {
		if e.Replacement == "unknown field" {
			return output, false, nil
		}
	}
	return output, true, nil
}

// Override is a convenience helper that validates against the latest known
// sing-box version.
func Override(input []byte, fn func(tree map[string]any) bool) ([]byte, bool, error) {
	return NewConfigParser().Override(input, fn)
}
