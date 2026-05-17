package model

// ClassificationResult holds the output of intent classification.
type ClassificationResult struct {
	Role AgentRole `json:"role"`
	Flow string    `json:"flow,omitempty"` // e.g. "onboarding", "proposal", "contract"
}
