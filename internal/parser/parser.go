package parser

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath" // Added for path manipulation

	"gopkg.in/yaml.v3"
)

type Job struct {
	Name         string            `yaml:"-"` // Name is the key in the map
	Stage        string            `yaml:"stage,omitempty"`
	Script       []string          `yaml:"script,omitempty"`
	Rules        []Rule            `yaml:"rules,omitempty"`
	Only         *OnlyExcept       `yaml:"only,omitempty"`
	Except       *OnlyExcept       `yaml:"except,omitempty"`
	When         string            `yaml:"when,omitempty"`
	AllowFailure bool              `yaml:"allow_failure,omitempty"`
	Dependencies []string          `yaml:"dependencies,omitempty"`
	Needs        []string          `yaml:"needs,omitempty"` // In GitLab, `needs` can be complex. This parser simplifies to []string.
	Tags         []string          `yaml:"tags,omitempty"`
	Image        interface{}       `yaml:"image,omitempty"` // Added for completeness, can be string or map
	Variables    map[string]string `yaml:"variables,omitempty"` // Added for completeness
	Artifacts    map[string]interface{} `yaml:"artifacts,omitempty"` // Added for completeness
	Cache        map[string]interface{} `yaml:"cache,omitempty"`     // Added for completeness
	Retry        map[string]interface{} `yaml:"retry,omitempty"`     // Added for completeness
	Services     []interface{}          `yaml:"services,omitempty"`  // Added for completeness
	Timeout      string                 `yaml:"timeout,omitempty"`   // Added for completeness
	BeforeScript []string               `yaml:"before_script,omitempty"`// Added for completeness
	AfterScript  []string               `yaml:"after_script,omitempty"` // Added for completeness
}

type Rule struct {
	If           string            `yaml:"if,omitempty"`
	When         string            `yaml:"when,omitempty"`
	Changes      []string          `yaml:"changes,omitempty"`
	Variables    map[string]string `yaml:"variables,omitempty"`
	AllowFailure *bool             `yaml:"allow_failure,omitempty"` // Pointer to distinguish between false and not set
	Exists       []string          `yaml:"exists,omitempty"`        // Added for completeness
}

type OnlyExcept struct {
	Refs       []string `yaml:"refs,omitempty"`
	Variables  []string `yaml:"variables,omitempty"`
	Changes    []string `yaml:"changes,omitempty"`
	Kubernetes bool     `yaml:"kubernetes,omitempty"`
}

type Pipeline struct {
	Jobs       map[string]*Job        `yaml:"-"` // Jobs are top-level keys after processing
	Stages     []string               `yaml:"stages,omitempty"`
	Variables  map[string]string      `yaml:"variables,omitempty"`
	Workflow   map[string]interface{} `yaml:"workflow,omitempty"`
	Default    map[string]interface{} `yaml:"default,omitempty"`
	Includes   []string               `yaml:"include,omitempty"` // Reflects original include, not for resolved output
}

// ParseFile parses a GitLab CI configuration file and returns a Pipeline object.
// visited is used to detect circular dependencies.
func ParseFile(filePath string, visited map[string]bool) (*Pipeline, error) {
	if visited[filePath] {
		return nil, fmt.Errorf("circular dependency detected: %s", filePath)
	}
	visited[filePath] = true
	// Make sure to create a new map for the next level of recursion
	// to avoid issues with parallel parsing paths if that were ever implemented.
	// For sequential, it's mainly to correctly mark files in the current path.
	defer delete(visited, filePath) // Remove from visited when returning from this path

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	content, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	var data map[string]interface{}
	if err := yaml.Unmarshal(content, &data); err != nil {
		return nil, fmt.Errorf("failed to parse YAML from %s: %w", filePath, err)
	}

	pipeline := &Pipeline{
		Jobs:      make(map[string]*Job),
		Variables: make(map[string]string),
		// Stages will be populated from the current file or merged.
	}

	// Process includes first
	if includeInterface, ok := data["include"]; ok {
		// Create a copy of visited for parseIncludeDirective to prevent
		// sibling includes from affecting each other's visited state directly.
		// ParseFile manages its own entry in the visited map for the current filePath.
		visitedCopyForIncludes := make(map[string]bool)
		for k, v := range visited {
			visitedCopyForIncludes[k] = v
		}

		baseDir := filepath.Dir(filePath)
		includedPipelines, err := parseIncludeDirective(includeInterface, baseDir, visitedCopyForIncludes)
		if err != nil {
			return nil, fmt.Errorf("error processing includes for %s: %w", filePath, err)
		}

		// Merge included pipelines. The main pipeline's definitions will take precedence (handled in mergePipelines).
		// Order of includes matters if they define the same elements not present in the main config.
		for _, includedPipe := range includedPipelines {
			pipeline = mergePipelines(pipeline, includedPipe)
		}
	}

	// Populate current pipeline from data (definitions in this file override/append to included ones)
	if stagesInterface, ok := data["stages"]; ok {
		if stagesSlice, ok := stagesInterface.([]interface{}); ok {
			for _, stage := range stagesSlice {
				if stageStr, ok := stage.(string); ok {
					pipeline.Stages = append(pipeline.Stages, stageStr)
				}
			}
		}
	}

	if varsInterface, ok := data["variables"]; ok {
		if varsMap, ok := varsInterface.(map[string]interface{}); ok {
			for k, v := range varsMap {
				if vStr, ok := v.(string); ok {
					pipeline.Variables[k] = vStr
				}
			}
		}
	}
	
	if workflowInterface, ok := data["workflow"]; ok {
		if workflowMap, ok := workflowInterface.(map[string]interface{}); ok {
			pipeline.Workflow = workflowMap
		}
	}
	
	if defaultInterface, ok := data["default"]; ok {
		if defaultMap, ok := defaultInterface.(map[string]interface{}); ok {
			pipeline.Default = defaultMap
		}
	}

	for key, value := range data {
		// Skip 'include' as it's handled above.
		// Also skip keys that are processed explicitly (stages, variables, workflow, default).
		if key == "include" || key == "stages" || key == "variables" || key == "workflow" || key == "default" {
			continue
		}

		if jobMap, ok := value.(map[string]interface{}); ok {
			if _, hasScript := jobMap["script"]; hasScript {
				job := &Job{
					Name: key,
				}

				if stageInterface, ok := jobMap["stage"]; ok {
					if stageStr, ok := stageInterface.(string); ok {
						job.Stage = stageStr
					}
				}

				if scriptInterface, ok := jobMap["script"]; ok {
					if scriptSlice, ok := scriptInterface.([]interface{}); ok {
						for _, script := range scriptSlice {
							if scriptStr, ok := script.(string); ok {
								job.Script = append(job.Script, scriptStr)
							}
						}
					} else if scriptStr, ok := scriptInterface.(string); ok {
						job.Script = append(job.Script, scriptStr)
					}
				}
				
				if depsInterface, ok := jobMap["dependencies"]; ok {
					if depsSlice, ok := depsInterface.([]interface{}); ok {
						for _, dep := range depsSlice {
							if depStr, ok := dep.(string); ok {
								job.Dependencies = append(job.Dependencies, depStr)
							}
						}
					}
				}
				
				if needsInterface, ok := jobMap["needs"]; ok {
					if needsSlice, ok := needsInterface.([]interface{}); ok {
						for _, need := range needsSlice {
							if needStr, ok := need.(string); ok {
								job.Needs = append(job.Needs, needStr)
							} else if needMap, ok := need.(map[string]interface{}); ok {
								if jobName, ok := needMap["job"].(string); ok {
									job.Needs = append(job.Needs, jobName)
								}
							}
						}
					} else if needStr, ok := needsInterface.(string); ok {
						job.Needs = append(job.Needs, needStr)
					}
				}

				if rulesInterface, ok := jobMap["rules"]; ok {
					if rulesSlice, ok := rulesInterface.([]interface{}); ok {
						for _, ruleInterface := range rulesSlice {
							if ruleMap, ok := ruleInterface.(map[string]interface{}); ok {
								rule := Rule{}

								if ifInterface, ok := ruleMap["if"]; ok {
									if ifStr, ok := ifInterface.(string); ok {
										rule.If = ifStr
									}
								}

								if whenInterface, ok := ruleMap["when"]; ok {
									if whenStr, ok := whenInterface.(string); ok {
										rule.When = whenStr
									}
								}

								if changesInterface, ok := ruleMap["changes"]; ok {
									if changesSlice, ok := changesInterface.([]interface{}); ok {
										for _, change := range changesSlice {
											if changeStr, ok := change.(string); ok {
												rule.Changes = append(rule.Changes, changeStr)
											}
										}
									}
								}

								job.Rules = append(job.Rules, rule)
							}
						}
					}
				}

				pipeline.Jobs[key] = job
			}
		}
	}

	return pipeline, nil
}

// mergePipelines merges the includedPipeline into the mainPipeline according to GitLab CI rules.
// Main pipeline's definitions take precedence.
func mergePipelines(main, included *Pipeline) *Pipeline {
	if main == nil {
		return included
	}
	if included == nil {
		return main
	}

	// Merge Jobs: main takes precedence
	for jobName, job := range included.Jobs {
		if _, exists := main.Jobs[jobName]; !exists {
			main.Jobs[jobName] = job
		}
	}

	// Merge Stages: append new stages, maintaining order of main's stages first
	mainStagesSet := make(map[string]bool)
	for _, stage := range main.Stages {
		mainStagesSet[stage] = true
	}
	for _, stage := range included.Stages {
		if !mainStagesSet[stage] {
			main.Stages = append(main.Stages, stage)
			mainStagesSet[stage] = true // Add to set to prevent duplicates from multiple includes
		}
	}

	// Merge Variables: main takes precedence
	for varName, varValue := range included.Variables {
		if _, exists := main.Variables[varName]; !exists {
			main.Variables[varName] = varValue
		}
	}

	// Merge Workflow: main takes precedence
	if main.Workflow == nil && included.Workflow != nil {
		main.Workflow = included.Workflow
	}
	// If main.Workflow is not nil, it stays.

	// Merge Default: main takes precedence
	if main.Default == nil && included.Default != nil {
		main.Default = included.Default
	}
	// If main.Default is not nil, it stays.

	return main
}

// parseIncludeDirective processes the 'include' directive and parses the included files.
// It handles strings, lists of strings, and lists of maps (checking for 'local' key).
func parseIncludeDirective(includeData interface{}, basePath string, visited map[string]bool) ([]*Pipeline, error) {
	var includedPipelines []*Pipeline

	// parseAndAdd is a helper closure to parse a single included file.
	// It creates a deep copy of the visited map for each recursive call.
	parseAndAdd := func(localPath string) error {
		// Crucial: Make a deep copy of visited for this specific include path.
		// This ensures that sibling includes don't affect each other's "visited" state,
		// and that a file can be included multiple times if it's not part of a circular dependency
		// along a single inclusion chain.
		visitedForThisPath := make(map[string]bool)
		for k, v := range visited {
			visitedForThisPath[k] = v
		}

		resolvedPath, err := resolveIncludePath(basePath, localPath)
		if err != nil {
			return fmt.Errorf("error resolving path for include '%s': %w", localPath, err)
		}

		// ParseFile will check visitedForThisPath[resolvedPath] and add resolvedPath to it.
		includedPipeline, err := ParseFile(resolvedPath, visitedForThisPath)
		if err != nil {
			return fmt.Errorf("error parsing included file '%s': %w", resolvedPath, err)
		}
		if includedPipeline != nil {
			includedPipelines = append(includedPipelines, includedPipeline)
		}
		return nil
	}

	switch v := includeData.(type) {
	case string: // Single include: 'path/to/file.yml'
		if err := parseAndAdd(v); err != nil {
			return nil, err
		}
	case []interface{}: // List of includes
		for i, item := range v {
			switch itemTyped := item.(type) {
			case string: // e.g., "- 'path/to/file.yml'"
				if err := parseAndAdd(itemTyped); err != nil {
					return nil, fmt.Errorf("error processing include list item %d (string: '%s'): %w", i, itemTyped, err)
				}
			case map[string]interface{}: // e.g., "- local: 'path/to/file.yml'"
				if localPath, ok := itemTyped["local"].(string); ok {
					if err := parseAndAdd(localPath); err != nil {
						return nil, fmt.Errorf("error processing include list item %d (local map: '%s'): %w", i, localPath, err)
					}
				} else {
					// Silently ignore non-local includes as per current focus.
					// In a more complete parser, other keys like 'project', 'remote', 'template' would be handled here.
				}
			default:
				return nil, fmt.Errorf("unsupported include list item type at index %d: %T", i, item)
			}
		}
	case map[string]interface{}: // Single include as map: include: { local: 'path.yml' }
		if localPath, ok := v["local"].(string); ok {
			if err := parseAndAdd(localPath); err != nil {
				return nil, err
			}
		} else {
			// Silently ignore non-local include map if not 'local'.
		}
	default:
		return nil, fmt.Errorf("unsupported include type: %T. Must be a string, a list of strings/maps, or a map with a 'local' key", includeData)
	}
	return includedPipelines, nil
}

// resolveIncludePath resolves the includePath relative to the basePath.
// It cleans the path and makes it absolute.
func resolveIncludePath(basePath, includePath string) (string, error) {
	if filepath.IsAbs(includePath) {
		// This case might indicate a '/'-prefixed path in GitLab CI, meaning project root.
		// For a simple file system parser, treat it as an absolute FS path.
		// A more advanced parser would need a concept of 'project root' to handle this correctly.
		return filepath.Clean(includePath), nil
	}
	// Join basePath (directory of the current file) with the relative includePath.
	absPath := filepath.Join(basePath, includePath)
	// Clean the resulting path (e.g., resolve "..", ".").
	return filepath.Clean(absPath), nil
}
