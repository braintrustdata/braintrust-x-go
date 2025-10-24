# Score Metadata Example

This example demonstrates how to add metadata to scores for debugging and annotation purposes in the Braintrust Go SDK.

## Overview

The Score struct now supports an optional `Metadata` field that allows scorers to attach arbitrary debug information, reasoning, or intermediate results to each score. This metadata is automatically logged to the evaluation spans and visible in the Braintrust dashboard.

## Features

- **Debug Information**: Attach reasoning, intermediate calculations, or analysis details to scores
- **Backward Compatible**: The metadata field is optional (uses `omitempty` JSON tag)
- **Rich Annotations**: Store any JSON-serializable data (strings, numbers, objects, arrays)
- **Automatic Logging**: Metadata is automatically included in span attributes

## Running the Example

### Prerequisites

1. Set up your Braintrust API key:
   ```bash
   export BRAINTRUST_API_KEY="your-api-key-here"
   ```

2. Ensure Go 1.23+ is installed

### Run the Example

```bash
cd examples/score-metadata
go run .
```

Or from the repository root:

```bash
make examples  # Runs all examples including this one
```

## What This Example Does

The example runs an evaluation with three test cases that convert strings to uppercase. It includes three scorers that demonstrate different ways to use metadata:

### 1. **Exact Match Scorer** (with metadata)
- Checks if the result exactly matches the expected output
- Includes metadata with:
  - Reasoning explanation
  - Input length
  - Expected and result values
  - Case metadata from the test case
  - Match status

### 2. **Length Similarity Scorer** (with metadata)
- Calculates similarity based on string length
- Includes metadata with:
  - Expected and result lengths
  - Length difference
  - Detailed calculation information

### 3. **Character Analysis Scorer** (with metadata)
- Performs character-by-character comparison
- Includes metadata with:
  - Number of matched characters
  - Match percentage
  - First mismatch location and context
  - Detailed analysis information

## Score Structure

```go
type Score struct {
    Name     string         `json:"name"`
    Score    float64        `json:"score"`
    Metadata map[string]any `json:"metadata,omitempty"`
}
```

## Usage Example

```go
eval.NewScorer("my_scorer", func(ctx context.Context, input, expected, result string, caseMeta eval.Metadata) (eval.Scores, error) {
    // Your scoring logic here
    score := 0.95
    
    return eval.Scores{
        {
            Name:  "accuracy",
            Score: score,
            Metadata: map[string]any{
                "reasoning":    "Result matches expected output with minor differences",
                "debug_info":   map[string]any{
                    "tokens_used": 150,
                    "model":       "gpt-4",
                },
                "case_metadata": caseMeta,
            },
        },
    }, nil
})
```

## Viewing Results

After running the example:

1. Go to the Braintrust dashboard
2. Navigate to the "go-sdk-examples" project
3. Find the "score-metadata-example" experiment
4. Click on individual dataset entries to see:
   - The scores (in `braintrust.scores`)
   - The detailed score information with metadata (in `braintrust.score_details`)

## Metadata in Spans

The metadata is logged to OpenTelemetry spans in two ways:

1. **`braintrust.scores`**: Simple map of score name → score value (for quick reference)
2. **`braintrust.score_details`**: Detailed information including metadata for scores that have it

Example span attributes:
```json
{
  "braintrust.scores": {
    "exact_match": 1.0,
    "length_similarity": 1.0,
    "character_match": 1.0
  },
  "braintrust.score_details": {
    "exact_match": {
      "score": 1.0,
      "metadata": {
        "reasoning": "Checked if result exactly matches expected output",
        "input_length": 11,
        "expected": "HELLO WORLD",
        "result": "HELLO WORLD",
        "match_status": true
      }
    }
  }
}
```

## Use Cases

- **Debugging**: Include intermediate calculations and decision logic
- **Auditing**: Store information about how scores were calculated
- **Analysis**: Attach contextual information for later review
- **Tracing**: Include model names, parameters, or other configuration
- **Annotations**: Add human-readable explanations for scores

## Notes

- Metadata is optional - scores without metadata work exactly as before
- Metadata must be JSON-serializable (no functions, channels, etc.)
- Large metadata objects may impact performance - use judiciously
- Metadata is visible in the Braintrust dashboard and exported with experiment data

