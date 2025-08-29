package apptype

// Entity represents a node in the knowledge graph
type Entity struct {
	Name         string    `json:"name"`
	EntityType   string    `json:"entityType"`
	Observations []string  `json:"observations"`
	Embedding    []float32 `json:"embedding,omitempty"`
}

// Relation represents a directed relationship between two entities
type Relation struct {
	From         string `json:"from"`
	To           string `json:"to"`
	RelationType string `json:"relationType"`
}

// SearchResult represents the result of a similarity search
type SearchResult struct {
	Entity   Entity  `json:"entity"`
	Distance float64 `json:"distance"`
}

// UpdateEntitySpec supports partial updates to entities
type UpdateEntitySpec struct {
	Name                string     `json:"name"`
	EntityType          *string    `json:"entityType,omitempty"`
	Embedding           *[]float32 `json:"embedding,omitempty"`
	MergeObservations   []string   `json:"mergeObservations,omitempty"`
	ReplaceObservations []string   `json:"replaceObservations,omitempty"`
}

// UpdateRelationChange represents a relation tuple update
type UpdateRelationChange struct {
	From            string `json:"from"`
	To              string `json:"to"`
	RelationType    string `json:"relationType"`
	NewFrom         string `json:"newFrom,omitempty"`
	NewTo           string `json:"newTo,omitempty"`
	NewRelationType string `json:"newRelationType,omitempty"`
}

