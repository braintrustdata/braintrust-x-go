package devserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/braintrustdata/braintrust-x-go/braintrust"
	"github.com/braintrustdata/braintrust-x-go/braintrust/eval"
)

// handleRoot handles GET / - health check endpoint.
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hello, world!"))
}

// handleList handles GET /list - returns all available evaluators and their metadata.
func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Build response from registry
	response := make(map[string]interface{})

	for name, eval := range s.registry {
		evalInfo := map[string]interface{}{
			"parameters": map[string]interface{}{}, // TODO: Add parameter support later
			"scores":     []map[string]string{},    // Empty for now
		}

		// Add scorer names
		scorerNames := make([]map[string]string, len(eval.scorers))
		for i := range eval.scorers {
			// We don't have scorer names stored yet, so use generic names
			scorerNames[i] = map[string]string{"name": fmt.Sprintf("scorer_%d", i)}
		}
		evalInfo["scores"] = scorerNames

		response[name] = evalInfo
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleEval handles POST /eval - executes an evaluator with provided data.
func (s *Server) handleEval(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req evalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Look up evaluator in registry
	evaluator, ok := s.registry[req.Name]
	if !ok {
		http.Error(w, fmt.Sprintf("Evaluator %q not found", req.Name), http.StatusNotFound)
		return
	}

	// Load dataset
	cases, err := s.loadDataset(req.Data, evaluator.projectName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load dataset: %v", err), http.StatusBadRequest)
		return
	}

	// Determine experiment name
	experimentName := req.ExperimentName
	if experimentName == "" {
		experimentName = req.Name // Use evaluator name as default
	}

	// Wrap task and scorers for eval.Run
	wrappedTask := eval.Task[interface{}, interface{}](evaluator.task)
	wrappedScorers := make([]eval.Scorer[interface{}, interface{}], len(evaluator.scorers))
	for i, scorerFunc := range evaluator.scorers {
		wrappedScorers[i] = &typeErasedScorer{
			name:     fmt.Sprintf("scorer_%d", i),
			scorerFn: scorerFunc,
		}
	}

	// Run the evaluation
	log.Printf("Running eval %q with experiment %q", req.Name, experimentName)
	result, err := eval.Run(context.Background(), eval.Opts[interface{}, interface{}]{
		Project:    evaluator.projectName,
		Experiment: experimentName,
		Cases:      cases,
		Task:       wrappedTask,
		Scorers:    wrappedScorers,
		Quiet:      true, // Don't print to console
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Eval failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Get permalink and key from result
	permalink, _ := result.Permalink()
	key := result.Key()

	// Build project URL
	projectURL := ""
	config := braintrust.GetConfig()
	if config.AppURL != "" && config.OrgName != "" && key.ProjectID != "" {
		projectURL = fmt.Sprintf("%s/app/%s/p/%s", config.AppURL, config.OrgName, key.ProjectID)
	}

	// Build response
	response := evalResponse{
		ExperimentName: key.Name,
		ProjectName:    key.ProjectName,
		ProjectID:      key.ProjectID,
		ExperimentID:   key.ExperimentID,
		ExperimentURL:  permalink,
		ProjectURL:     projectURL,
		Scores:         map[string]interface{}{}, // TODO: populate from result
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// loadDataset loads evaluation cases based on the DataSpec
func (s *Server) loadDataset(spec dataSpec, projectName string) (eval.Cases[interface{}, interface{}], error) {
	// Option 1: Inline data
	if len(spec.Data) > 0 {
		cases := make([]eval.Case[interface{}, interface{}], len(spec.Data))
		for i, item := range spec.Data {
			cases[i] = eval.Case[interface{}, interface{}]{
				Input:    item["input"],
				Expected: item["expected"],
				Metadata: item, // Include all fields as metadata
			}
		}
		return eval.NewCases(cases), nil
	}

	// Option 2: By dataset name
	if spec.DatasetName != "" {
		return eval.QueryDataset[interface{}, interface{}](eval.DatasetOpts{
			ProjectName: projectName,
			DatasetName: spec.DatasetName,
		})
	}

	// Option 3: By dataset ID
	if spec.DatasetID != "" {
		return eval.QueryDataset[interface{}, interface{}](eval.DatasetOpts{
			DatasetID: spec.DatasetID,
		})
	}

	return nil, fmt.Errorf("no data source specified")
}

// typeErasedScorer wraps a type-erased scorer function
type typeErasedScorer struct {
	name     string
	scorerFn func(context.Context, interface{}, interface{}, interface{}, eval.Metadata) (eval.Scores, error)
}

func (s *typeErasedScorer) Name() string {
	return s.name
}

func (s *typeErasedScorer) Run(ctx context.Context, input, expected, result interface{}, meta eval.Metadata) (eval.Scores, error) {
	return s.scorerFn(ctx, input, expected, result, meta)
}
