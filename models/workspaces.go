package models

import "encoding/json"

// WorkspaceCategoryRequest is the body for setting a workspace's pricing category.
// A null category clears it, so the workspace pays the default rate.
type WorkspaceCategoryRequest struct {
	Category *string `json:"category" extensions:"x-nullable"`

	present bool
}

// UnmarshalJSON records whether the category key was sent, so a missing key is not
// mistaken for an explicit null that clears the category.
func (r *WorkspaceCategoryRequest) UnmarshalJSON(data []byte) error {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return err
	}
	_, r.present = keys["category"]

	type plain WorkspaceCategoryRequest
	return json.Unmarshal(data, (*plain)(r))
}

// HasCategory reports whether the request body included the category key.
func (r WorkspaceCategoryRequest) HasCategory() bool {
	return r.present
}
