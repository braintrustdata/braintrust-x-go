package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/braintrust/braintrust-x-go/braintrust"
)

// InvokeFunctionRequest represents the request payload for invoking a function
type InvokeFunctionRequest struct {
	Input          interface{}            `json:"input"`
	Expected       interface{}            `json:"expected,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Tags           []string               `json:"tags,omitempty"`
	Messages       []interface{}          `json:"messages,omitempty"`
	Parent         *TracingInfo           `json:"parent,omitempty"`
	Stream         *bool                  `json:"stream,omitempty"`
	Mode           *string                `json:"mode,omitempty"`
	Strict         *bool                  `json:"strict,omitempty"`
	Version        *string                `json:"version,omitempty"`
	GlobalFunction *string                `json:"global_function,omitempty"`
	ProjectName    *string                `json:"project_name,omitempty"`
	Slug           *string                `json:"slug,omitempty"`
}

// TracingInfo represents parent tracing information
type TracingInfo struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	ObjectID  string `json:"object_id,omitempty"`
	ComputeID string `json:"compute_id,omitempty"`
}

// InvokeFunctionResponse represents the response from invoking a function
type InvokeFunctionResponse struct {
	Output   interface{}            `json:"output,omitempty"`
	Error    *string                `json:"error,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Tags     []string               `json:"tags,omitempty"`
	Metrics  map[string]interface{} `json:"metrics,omitempty"`
}

// Function represents a function from the API
type Function struct {
	ID          string                 `json:"id"`
	ProjectID   string                 `json:"project_id,omitempty"`
	Name        string                 `json:"function_name"`
	Description string                 `json:"description,omitempty"`
	Slug        string                 `json:"slug,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ListFunctionsRequest represents query parameters for listing functions
type ListFunctionsRequest struct {
	Limit         *int    `url:"limit,omitempty"`
	StartingAfter *string `url:"starting_after,omitempty"`
	EndingBefore  *string `url:"ending_before,omitempty"`
	FunctionName  *string `url:"function_name,omitempty"`
	ProjectName   *string `url:"project_name,omitempty"`
	ProjectID     *string `url:"project_id,omitempty"`
	Slug          *string `url:"slug,omitempty"`
	Version       *string `url:"version,omitempty"`
}

// ListFunctionsResponse represents the response from listing functions
type ListFunctionsResponse struct {
	Objects []Function `json:"objects"`
}

// InvokeFunction invokes a function by ID via the Braintrust API
func InvokeFunction(functionID string, request InvokeFunctionRequest) (*InvokeFunctionResponse, error) {
	return invokeFunction("/v1/function/"+functionID+"/invoke", request)
}

// InvokeFunctionByName invokes a function by project name and slug via the Braintrust API
func InvokeFunctionByName(projectName, slug string, request InvokeFunctionRequest) (*InvokeFunctionResponse, error) {
	request.ProjectName = &projectName
	request.Slug = &slug
	return invokeFunction("/v1/function/invoke", request)
}

// InvokeGlobalFunction invokes a global function by name via the Braintrust API
func InvokeGlobalFunction(globalFunction string, request InvokeFunctionRequest) (*InvokeFunctionResponse, error) {
	request.GlobalFunction = &globalFunction
	return invokeFunction("/v1/function/invoke", request)
}

// invokeFunction is the internal helper that handles the actual HTTP request
func invokeFunction(endpoint string, request InvokeFunctionRequest) (*InvokeFunctionResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	config := braintrust.GetConfig()

	httpReq, err := http.NewRequest("POST", config.APIURL+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+config.APIKey)

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read the response body to get error details
		var errorBody []byte
		if resp.Body != nil {
			errorBody, _ = io.ReadAll(resp.Body)
		}
		return nil, fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(errorBody))
	}

	var result InvokeFunctionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return &result, nil
}

// ListFunctions lists available functions via the Braintrust API
func ListFunctions(request *ListFunctionsRequest) (*ListFunctionsResponse, error) {
	config := braintrust.GetConfig()

	// Build URL with query parameters
	baseURL := config.APIURL + "/v1/function"
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %w", err)
	}

	// Add query parameters if request is provided
	if request != nil {
		query := u.Query()
		v := reflect.ValueOf(*request)
		t := reflect.TypeOf(*request)

		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			fieldType := t.Field(i)
			tag := fieldType.Tag.Get("url")

			if tag == "" || tag == "-" {
				continue
			}

			// Extract the parameter name (before the comma)
			paramName := tag
			if idx := strings.Index(tag, ","); idx != -1 {
				paramName = tag[:idx]
			}

			// Add non-nil values to query
			if !field.IsNil() {
				switch field.Kind() {
				case reflect.Ptr:
					elem := field.Elem()
					switch elem.Kind() {
					case reflect.String:
						query.Add(paramName, elem.String())
					case reflect.Int:
						query.Add(paramName, strconv.Itoa(int(elem.Int())))
					}
				}
			}
		}
		u.RawQuery = query.Encode()
	}

	httpReq, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+config.APIKey)

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read the response body to get error details
		var errorBody []byte
		if resp.Body != nil {
			errorBody, _ = io.ReadAll(resp.Body)
		}
		return nil, fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(errorBody))
	}

	var result ListFunctionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return &result, nil
}
