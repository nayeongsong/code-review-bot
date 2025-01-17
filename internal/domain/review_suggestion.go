package domain

import (
	"github.com/invopop/jsonschema"
)

type ReviewSuggestion struct {
	Comment  string `json:"comment"`
	Position int    `json:"position"`
}

type ReviewSuggestionsResponse struct {
	ReviewSuggestions []ReviewSuggestion `json:"review_suggestions"`
}

func GenerateSchema[T any]() interface{} {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	schema := reflector.Reflect(v)
	return schema
}

// Generate the JSON schema at initialization time
var ReviewSuggestionSchema = GenerateSchema[ReviewSuggestionsResponse]()
