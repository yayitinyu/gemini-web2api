package gemini

import (
	"fmt"
	"strings"
)

type Model struct {
	ID           string `json:"id"`
	HexID        string `json:"-"`
	Mode         int    `json:"-"`
	Description  string `json:"description"`
	RequiresAuth bool   `json:"requires_auth"`
}

var modelCatalog = []Model{
	{ID: "gemini-3.7-flash", HexID: "fbb127bbb056c959", Mode: 1, Description: "Latest all-around Gemini Web model"},
	{ID: "gemini-3.6-flash", HexID: "fbb127bbb056c959", Mode: 1, Description: "Previous generation Gemini Web Flash model"},
	{ID: "gemini-3.5-flash-lite", HexID: "cf41b0e0dd7d53e5", Mode: 6, Description: "Fastest lightweight Gemini Web model"},
	{ID: "gemini-3.1-pro", HexID: "9d8ca3786ebdfbea", Mode: 3, Description: "Most capable model; requires a signed-in Google account", RequiresAuth: true},
}

func Models(authenticated bool) []Model {
	models := make([]Model, 0, len(modelCatalog))
	for _, model := range modelCatalog {
		if model.RequiresAuth && !authenticated {
			continue
		}
		models = append(models, model)
	}
	return models
}

func ModelIDs() []string {
	ids := make([]string, 0, len(modelCatalog))
	for _, model := range modelCatalog {
		ids = append(ids, model.ID)
	}
	return ids
}

func ResolveModel(name string, authenticated bool) (Model, error) {
	if index := strings.Index(name, "@think="); index >= 0 {
		name = name[:index]
	}
	for _, model := range modelCatalog {
		if model.ID != name {
			continue
		}
		if model.RequiresAuth && !authenticated {
			return Model{}, fmt.Errorf("%s requires a signed-in Google account cookie; anonymous upstream requests are silently downgraded", name)
		}
		return model, nil
	}
	return Model{}, fmt.Errorf("unknown model %q", name)
}
