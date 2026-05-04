package model

import "time"

import "github.com/google/uuid"

// ArtifactFile represents a single file within an artifact project.
type ArtifactFile struct {
	ID         uuid.UUID `json:"id"`
	ArtifactID uuid.UUID `json:"artifact_id"`
	Path       string    `json:"path"`
	Language   string    `json:"language,omitempty"`
	Content    string    `json:"content"`
	FileOrder  int       `json:"file_order"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ProjectFile is the simplified file view used in API responses.
type ProjectFile struct {
	Path     string `json:"path"`
	Language string `json:"language,omitempty"`
	Content  string `json:"content"`
}

// Artifact is a generated project attached to a conversation.
// For single-file artifacts, Content is populated and Files is empty.
// For multi-file projects, Files contains the individual files and Content may be empty.
type Artifact struct {
	ID             uuid.UUID      `json:"id"`
	ConversationID uuid.UUID      `json:"conversation_id"`
	MessageID      *uuid.UUID     `json:"message_id,omitempty"`
	Name           string         `json:"name"`
	Type           string         `json:"type"`
	Language       string         `json:"language,omitempty"`
	Content        string         `json:"content"`
	EntryFile      string         `json:"entry_file,omitempty"`
	IsComplete     bool           `json:"is_complete"`
	Version        int            `json:"version"`
	IsDeleted      bool           `json:"is_deleted"`
	Files          []ArtifactFile `json:"files,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// ToProject normalizes an artifact (single or multi-file) into a project view.
func (a *Artifact) ToProject() ArtifactProject {
	project := ArtifactProject{
		ID:             a.ID,
		ConversationID: a.ConversationID,
		Name:           a.Name,
		Type:           a.Type,
		EntryFile:      a.EntryFile,
		Version:        a.Version,
		IsComplete:     a.IsComplete,
		CreatedAt:      a.CreatedAt,
		UpdatedAt:      a.UpdatedAt,
		Files:          []ProjectFile{},
	}

	// If multi-file
	if len(a.Files) > 0 {
		for _, f := range a.Files {
			project.Files = append(project.Files, ProjectFile{
				Path:     f.Path,
				Language: f.Language,
				Content:  f.Content,
			})
		}
	} else {
		// Single-file fallback
		project.Files = append(project.Files, ProjectFile{
			Path:     a.EntryFile,
			Language: a.Language,
			Content:  a.Content,
		})
	}

	return project
}

// ArtifactProject is the normalized view returned by the API.
type ArtifactProject struct {
	ID             uuid.UUID     `json:"id"`
	ConversationID uuid.UUID     `json:"conversation_id"`
	Name           string        `json:"name"`
	Type           string        `json:"type"`
	EntryFile      string        `json:"entry_file"`
	Version        int           `json:"version"`
	IsComplete     bool          `json:"is_complete"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
	Files          []ProjectFile `json:"files"`
}

// ArtifactResponse is the metadata returned alongside a chat message.
type ArtifactResponse struct {
	ArtifactID   string `json:"artifactId,omitempty"`
	ArtifactType string `json:"artifactType,omitempty"`
}
