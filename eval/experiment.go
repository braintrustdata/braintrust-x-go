package eval

import (
	"context"
	"fmt"

	"github.com/braintrustdata/braintrust-x-go/api"
	"github.com/braintrustdata/braintrust-x-go/config"
	"github.com/braintrustdata/braintrust-x-go/internal/auth"
)

// registerExperiment creates or gets an experiment for the eval.
// This is an internal helper that uses the api package.
func registerExperiment(ctx context.Context, cfg *config.Config, session *auth.Session, name string, tags []string, metadata map[string]interface{}, update bool) (*api.Experiment, error) {
	if name == "" {
		return nil, fmt.Errorf("experiment name is required")
	}

	// First get or create the project
	projectName := cfg.DefaultProjectName
	if projectName == "" {
		return nil, fmt.Errorf("project name is required (set via WithProject option)")
	}

	project, err := api.RegisterProject(ctx, cfg, session, projectName)
	if err != nil {
		return nil, fmt.Errorf("failed to register project: %w", err)
	}

	// Then register the experiment
	experiment, err := api.RegisterExperiment(ctx, cfg, session, name, project.ID, api.RegisterExperimentOpts{
		Tags:     tags,
		Metadata: metadata,
		Update:   update,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to register experiment: %w", err)
	}

	return experiment, nil
}
