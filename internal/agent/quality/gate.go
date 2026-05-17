package quality

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

// Gate evaluates LLM responses for quality before sending them to the user.
type Gate struct {
	client llm.Client
	log    logger.Logger
}

// NewGate creates a new quality gate.
func NewGate(client llm.Client, log logger.Logger) *Gate {
	return &Gate{client: client, log: log}
}

// Evaluation is the result of a quality check.
type Evaluation struct {
	Regenerate bool   `json:"regenerate"`
	Reason     string `json:"reason"`
	Scores     Scores `json:"scores"`
}

// Scores holds the quality dimensions.
type Scores struct {
	Completeness int `json:"completeness"`
	Correctness  int `json:"correctness"`
	Safety       int `json:"safety"`
}

// ShouldEvaluate determines if the quality gate should run for this agent/response.
func ShouldEvaluate(agent *model.Agent, response string) bool {
	if agent == nil {
		return false
	}
	// Only evaluate coder and architect responses
	if agent.Role != model.RoleCoder && agent.Role != model.RoleArchitect {
		return false
	}
	// Only evaluate responses that contain code-like content
	if !containsCode(response) {
		return false
	}
	return true
}

func containsCode(response string) bool {
	lower := strings.ToLower(response)
	return strings.Contains(lower, "```") ||
		strings.Contains(lower, "func ") ||
		strings.Contains(lower, "def ") ||
		strings.Contains(lower, "class ") ||
		strings.Contains(lower, "import ") ||
		strings.Contains(lower, "const ") ||
		strings.Contains(lower, "let ") ||
		strings.Contains(lower, "var ") ||
		strings.Contains(lower, "<") && strings.Contains(lower, ">") ||
		strings.Contains(lower, "{") && strings.Contains(lower, "}")
}

// Evaluate runs the quality gate on a response.
func (g *Gate) Evaluate(ctx context.Context, request string, response string) (*Evaluation, error) {
	if g.client == nil {
		return &Evaluation{Regenerate: false}, nil
	}

	prompt := fmt.Sprintf(`You are a code reviewer. Evaluate this response on a scale of 1-10 for:
- Completeness: Does it fully solve what was asked?
- Correctness: Is the code syntactically valid?
- Safety: Does it handle errors?

User request: %q

Response to evaluate:
%s

Output ONLY a JSON object with this exact shape, no explanation:
{"regenerate": false, "reason": "", "scores": {"completeness": 8, "correctness": 9, "safety": 7}}

If any score is below 7, set regenerate to true and provide a concise reason (1 sentence).`, request, truncate(response, 4000))

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	resp, err := g.client.Complete(ctx, llm.Request{
		Model:       "qwen2.5-coder:3b",
		Messages:    []llm.Message{{Role: "user", Content: prompt}},
		Temperature: 0.0,
		MaxTokens:   200,
		Stream:      false,
	})
	if err != nil {
		g.log.Warn("quality gate: LLM evaluation failed", logger.Error(err))
		return &Evaluation{Regenerate: false}, nil
	}

	eval, err := parseEvaluation(resp)
	if err != nil {
		g.log.Warn("quality gate: failed to parse evaluation", logger.Error(err), logger.String("raw", resp))
		return &Evaluation{Regenerate: false}, nil
	}

	g.log.Info("quality gate: evaluation complete",
		logger.Int("completeness", eval.Scores.Completeness),
		logger.Int("correctness", eval.Scores.Correctness),
		logger.Int("safety", eval.Scores.Safety),
		logger.Bool("regenerate", eval.Regenerate),
		logger.String("reason", eval.Reason))

	return eval, nil
}

func parseEvaluation(raw string) (*Evaluation, error) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON object found")
	}
	var eval Evaluation
	if err := json.Unmarshal([]byte(raw[start:end+1]), &eval); err != nil {
		return nil, fmt.Errorf("unmarshal evaluation: %w", err)
	}
	return &eval, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
