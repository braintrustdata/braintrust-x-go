package eval

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Result contains the results and statistics from running an evaluation.
type Result struct {
	ExperimentID string
	TotalCases   int
	SuccessCount int
	ErrorCount   int
	StartTime    time.Time
	EndTime      time.Time
	Spans        []SpanData
	Scores       map[string][]float64 // scorer name -> list of scores
	Errors       []string
	Cases        []CaseResult // individual case results for table display
}

// CaseResult represents the result of evaluating a single case.
type CaseResult struct {
	Index    int
	Input    interface{}
	Expected interface{}
	Output   interface{}
	Scores   map[string]float64
	Duration time.Duration
	Status   string // "success", "task_error", "scorer_error"
	Err      error
}

// SpanData contains extracted information from OpenTelemetry spans.
type SpanData struct {
	Name       string
	Type       string // "eval", "task", "score"
	Status     codes.Code
	Error      string
	Duration   time.Duration
	Attributes map[string]interface{}
	Scores     map[string]float64
	SpanID     string
	ParentID   string
}

// String returns a formatted summary of the evaluation results.
func (r *Result) String() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Evaluation Results for %s\n", r.ExperimentID))
	b.WriteString(strings.Repeat("=", 50))
	b.WriteString("\n\n")

	// Summary statistics
	b.WriteString("Summary:\n")
	b.WriteString(fmt.Sprintf("  Total Cases: %d\n", r.TotalCases))
	b.WriteString(fmt.Sprintf("  Successful: %d\n", r.SuccessCount))
	b.WriteString(fmt.Sprintf("  Failed: %d\n", r.ErrorCount))

	if !r.EndTime.IsZero() && !r.StartTime.IsZero() {
		duration := r.EndTime.Sub(r.StartTime)
		b.WriteString(fmt.Sprintf("  Duration: %v\n", duration.Round(time.Millisecond)))
	}

	// Score statistics
	if len(r.Scores) > 0 {
		b.WriteString("\nScores:\n")

		for scorerName, scores := range r.Scores {
			if len(scores) == 0 {
				continue
			}

			// Calculate statistics
			var sum float64
			minScore := scores[0]
			maxScore := scores[0]

			for _, score := range scores {
				sum += score
				if score < minScore {
					minScore = score
				}
				if score > maxScore {
					maxScore = score
				}
			}

			avg := sum / float64(len(scores))

			b.WriteString(fmt.Sprintf("  %s: avg=%.3f, min=%.3f, max=%.3f, count=%d\n",
				scorerName, avg, minScore, maxScore, len(scores)))
		}
	}

	// Error summary
	if len(r.Errors) > 0 {
		b.WriteString("\nErrors:\n")
		errorCounts := make(map[string]int)
		for _, err := range r.Errors {
			errorCounts[err]++
		}

		// Sort errors by frequency
		type errorCount struct {
			error string
			count int
		}
		var sortedErrors []errorCount
		for err, count := range errorCounts {
			sortedErrors = append(sortedErrors, errorCount{err, count})
		}
		sort.Slice(sortedErrors, func(i, j int) bool {
			return sortedErrors[i].count > sortedErrors[j].count
		})

		for _, ec := range sortedErrors {
			if ec.count > 1 {
				b.WriteString(fmt.Sprintf("  %s (×%d)\n", ec.error, ec.count))
			} else {
				b.WriteString(fmt.Sprintf("  %s\n", ec.error))
			}
		}
	}

	// Case results table
	if len(r.Cases) > 0 {
		b.WriteString("\nCase Results:\n")
		r.writeTable(&b)
	}

	return b.String()
}

// writeTable formats case results in a table format similar to the Braintrust UI
func (r *Result) writeTable(b *strings.Builder) {
	if len(r.Cases) == 0 {
		return
	}

	// Collect all unique scorer names
	scorerNames := make(map[string]struct{})
	for _, caseResult := range r.Cases {
		for scorerName := range caseResult.Scores {
			scorerNames[scorerName] = struct{}{}
		}
	}

	// Convert to sorted slice
	var sortedScorers []string
	for scorerName := range scorerNames {
		sortedScorers = append(sortedScorers, scorerName)
	}
	sort.Strings(sortedScorers)

	// Calculate column widths
	const minColWidth = 8
	idxWidth := max(3, len(fmt.Sprintf("%d", len(r.Cases))))
	inputWidth := minColWidth
	expectedWidth := minColWidth
	outputWidth := minColWidth
	durationWidth := 10 // "  123.45ms" or "   12.34s"
	errorWidth := 30    // Error message column

	// Score column widths (scorer name + percentage)
	scoreWidths := make(map[string]int)
	for _, scorer := range sortedScorers {
		scoreWidths[scorer] = max(len(scorer), 6) // min width for "100.0%"
	}

	// Sample some cases to estimate column widths
	sampleSize := min(len(r.Cases), 10)
	for i := 0; i < sampleSize; i++ {
		caseResult := r.Cases[i]
		inputWidth = max(inputWidth, min(len(fmt.Sprintf("%v", caseResult.Input)), 20))
		expectedWidth = max(expectedWidth, min(len(fmt.Sprintf("%v", caseResult.Expected)), 20))
		outputWidth = max(outputWidth, min(len(fmt.Sprintf("%v", caseResult.Output)), 25))
	}

	// Write header
	fmt.Fprintf(b, "%-*s │ %-*s │ %-*s │ %-*s",
		idxWidth, "#", inputWidth, "Input", expectedWidth, "Expected", outputWidth, "Output")

	for _, scorer := range sortedScorers {
		fmt.Fprintf(b, " │ %*s", scoreWidths[scorer], scorer)
	}
	fmt.Fprintf(b, " │ %*s │ %-*s\n", durationWidth, "Duration", errorWidth, "Error")

	// Write separator
	totalWidth := idxWidth + 3 + inputWidth + 3 + expectedWidth + 3 + outputWidth + 3
	for _, scorer := range sortedScorers {
		totalWidth += scoreWidths[scorer] + 3
	}
	totalWidth += durationWidth + 3 + errorWidth
	b.WriteString(strings.Repeat("─", totalWidth))
	b.WriteString("\n")

	// Write rows
	for _, caseResult := range r.Cases {
		// Truncate text if too long
		input := truncateString(fmt.Sprintf("%v", caseResult.Input), inputWidth)
		expected := truncateString(fmt.Sprintf("%v", caseResult.Expected), expectedWidth)
		output := truncateString(fmt.Sprintf("%v", caseResult.Output), outputWidth)
		duration := formatDuration(caseResult.Duration)

		fmt.Fprintf(b, "%-*d │ %-*s │ %-*s │ %-*s",
			idxWidth, caseResult.Index+1, inputWidth, input, expectedWidth, expected, outputWidth, output)

		// Write scores
		for _, scorer := range sortedScorers {
			score, exists := caseResult.Scores[scorer]
			var scoreStr string
			if exists {
				scoreStr = fmt.Sprintf("%.1f%%", score*100)
			} else {
				scoreStr = "─"
			}
			fmt.Fprintf(b, " │ %*s", scoreWidths[scorer], scoreStr)
		}

		// Show error information
		errorDisplay := ""
		if caseResult.Err != nil {
			errorDisplay = truncateString(caseResult.Err.Error(), errorWidth)
		}

		fmt.Fprintf(b, " │ %*s │ %-*s\n", durationWidth, duration, errorWidth, errorDisplay)
	}
}

// Helper functions
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return "   0.00ms"
	}
	if d < time.Second {
		return fmt.Sprintf("%8.2fms", float64(d.Nanoseconds())/1e6)
	}
	return fmt.Sprintf("%8.2fs", d.Seconds())
}

// AddSpan processes and adds span data to the result.
func (r *Result) AddSpan(span sdktrace.ReadOnlySpan) {
	spanData := SpanData{
		Name:       span.Name(),
		Status:     span.Status().Code,
		Duration:   span.EndTime().Sub(span.StartTime()),
		Attributes: make(map[string]interface{}),
		Scores:     make(map[string]float64),
		SpanID:     span.SpanContext().SpanID().String(),
		ParentID:   span.Parent().SpanID().String(),
	}

	if span.Status().Code == codes.Error {
		spanData.Error = span.Status().Description
		r.Errors = append(r.Errors, span.Status().Description)
	}

	var input, expected, output interface{}

	// Extract attributes
	for _, attr := range span.Attributes() {
		key := string(attr.Key)
		switch key {
		case "braintrust.span_attributes":
			// Parse the span type
			if typeAttr := attr.Value.AsString(); typeAttr != "" {
				// Try to parse JSON to get the type
				if strings.Contains(typeAttr, `"type":"`) {
					if strings.Contains(typeAttr, `"type":"eval"`) {
						spanData.Type = "eval"
					} else if strings.Contains(typeAttr, `"type":"task"`) {
						spanData.Type = "task"
					} else if strings.Contains(typeAttr, `"type":"score"`) {
						spanData.Type = "score"
					}
				}
			}
		case "braintrust.scores":
			// Parse scores JSON string
			scoresStr := attr.Value.AsString()
			if scoresStr != "" {
				var scores map[string]float64
				if err := json.Unmarshal([]byte(scoresStr), &scores); err == nil {
					for scoreName, score := range scores {
						spanData.Scores[scoreName] = score
						r.addScore(scoreName, score)
					}
				}
			}
		case "braintrust.input_json":
			// Parse input JSON
			inputStr := attr.Value.AsString()
			if inputStr != "" {
				if err := json.Unmarshal([]byte(inputStr), &input); err != nil {
					input = inputStr // fallback to raw string if JSON parsing fails
				}
				spanData.Attributes["braintrust.input_json"] = input
			}
		case "braintrust.expected":
			// Parse expected JSON
			expectedStr := attr.Value.AsString()
			if expectedStr != "" {
				if err := json.Unmarshal([]byte(expectedStr), &expected); err != nil {
					expected = expectedStr // fallback to raw string if JSON parsing fails
				}
				spanData.Attributes["braintrust.expected"] = expected
			}
		case "braintrust.output_json":
			// Parse output JSON
			outputStr := attr.Value.AsString()
			if outputStr != "" {
				if err := json.Unmarshal([]byte(outputStr), &output); err != nil {
					output = outputStr // fallback to raw string if JSON parsing fails
				}
				spanData.Attributes["braintrust.output_json"] = output
			}
		default:
			spanData.Attributes[key] = attr.Value.AsInterface()
		}
	}

	r.Spans = append(r.Spans, spanData)
}

// BuildCaseResults processes all collected spans and builds case results by grouping related spans.
func (r *Result) BuildCaseResults() {
	// Group spans by parent ID to find related spans
	spansByParent := make(map[string][]SpanData)
	evalSpans := make([]SpanData, 0)

	for _, span := range r.Spans {
		if span.Type == "eval" {
			evalSpans = append(evalSpans, span)
		} else if span.ParentID != "" {
			spansByParent[span.ParentID] = append(spansByParent[span.ParentID], span)
		}
	}

	// Build case results from eval spans and their children
	for i, evalSpan := range evalSpans {
		caseResult := CaseResult{
			Index:    i,
			Scores:   make(map[string]float64),
			Duration: evalSpan.Duration,
			Status:   "success",
		}

		// Extract data from eval span attributes
		if input, ok := evalSpan.Attributes["braintrust.input_json"]; ok {
			caseResult.Input = input
		}
		if expected, ok := evalSpan.Attributes["braintrust.expected"]; ok {
			caseResult.Expected = expected
		}
		if output, ok := evalSpan.Attributes["braintrust.output_json"]; ok {
			caseResult.Output = output
		}

		// Handle eval span errors
		if evalSpan.Status == codes.Error {
			if strings.Contains(evalSpan.Error, "task run error") {
				caseResult.Status = "task_error"
			} else if strings.Contains(evalSpan.Error, "scorer error") {
				caseResult.Status = "scorer_error"
			} else {
				caseResult.Status = "error"
			}
			caseResult.Err = fmt.Errorf("%s", evalSpan.Error)
		}

		// Find child spans (task and score spans)
		childSpans := spansByParent[evalSpan.SpanID]
		for _, childSpan := range childSpans {
			switch childSpan.Type {
			case "task":
				// Task spans may have output data
				if output, ok := childSpan.Attributes["braintrust.output_json"]; ok {
					caseResult.Output = output
				}
			case "score":
				// Score spans have scoring results
				for scoreName, score := range childSpan.Scores {
					caseResult.Scores[scoreName] = score
				}
			}
		}

		r.Cases = append(r.Cases, caseResult)
	}
}

// addScore adds a score to the results.
func (r *Result) addScore(scorerName string, score float64) {
	if r.Scores == nil {
		r.Scores = make(map[string][]float64)
	}
	r.Scores[scorerName] = append(r.Scores[scorerName], score)
}

// SpanCollector captures spans during evaluation execution.
type SpanCollector struct {
	result *Result
}

// NewSpanCollector creates a new span collector.
func NewSpanCollector(result *Result) *SpanCollector {
	return &SpanCollector{result: result}
}

// OnEnd is called when a span ends, allowing us to collect its data.
func (sc *SpanCollector) OnEnd(span sdktrace.ReadOnlySpan) {
	if sc.result != nil {
		sc.result.AddSpan(span)
	}
}

// NewResult creates a new Result instance.
func NewResult(experimentID string) *Result {
	return &Result{
		ExperimentID: experimentID,
		StartTime:    time.Now(),
		Spans:        make([]SpanData, 0),
		Scores:       make(map[string][]float64),
		Errors:       make([]string, 0),
		Cases:        make([]CaseResult, 0),
	}
}
