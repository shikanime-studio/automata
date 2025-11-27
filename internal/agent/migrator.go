package agent

import (
	"context"
	"fmt"

	"github.com/shikanime-studio/automata/internal/config"
	igithub "github.com/shikanime-studio/automata/internal/github"
	ikio "github.com/shikanime-studio/automata/internal/kio"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/genai"
)

// MigratorConfig controls the migrator agent setup.
type MigratorConfig struct {
	Name        string
	Description string
	Instruction string
	ModelName   string
}

// NewMigratorAgent constructs a migrator agent with tools to update workflows.
func NewMigratorAgent(
	ctx context.Context,
	appcfg *config.Config,
	cfg MigratorConfig,
) (agent.Agent, error) {
	apiKey := appcfg.GoogleAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("GOOGLE_API_KEY is not set")
	}
	modelName := cfg.ModelName
	if modelName == "" {
		modelName = appcfg.ModelName()
	}
	m, err := gemini.NewModel(ctx, modelName, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, err
	}

	type updateArgs struct {
		Path string `json:"path"`
	}
	type updateResult struct {
		Result string `json:"result"`
	}
	updateTool, err := functiontool.New[updateArgs, updateResult](functiontool.Config{
		Name:        "update_github_workflows",
		Description: "Update GitHub Actions versions in .github/workflows under the given path",
	}, func(tc tool.Context, in updateArgs) (updateResult, error) {
		u := igithub.NewUpdater(igithub.NewClient(ctx, appcfg))
		if err := ikio.UpdateGitHubWorkflows(tc, u, in.Path).Execute(); err != nil {
			return updateResult{}, err
		}
		return updateResult{Result: fmt.Sprintf("updated workflows at %s", in.Path)}, nil
	})
	if err != nil {
		return nil, err
	}

	name := cfg.Name
	if name == "" {
		name = "migrator"
	}
	desc := cfg.Description
	if desc == "" {
		desc = "Checks upgrades and applies corrections for new versions"
	}
	instr := cfg.Instruction
	if instr == "" {
		instr = "You are a migration agent. Use tools to update and fix compatibility for newer versions."
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        name,
		Model:       m,
		Description: desc,
		Instruction: instr,
		Tools:       []tool.Tool{updateTool},
	})
	if err != nil {
		return nil, err
	}
	return a, nil
}
