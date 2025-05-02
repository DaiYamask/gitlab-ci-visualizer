package parser

import (
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v3"
)

type Job struct {
	Name        string
	Stage       string
	Script      []string
	Rules       []Rule
	Only        *OnlyExcept
	Except      *OnlyExcept
	When        string
	AllowFailure bool
	Dependencies []string
	Needs        []string
	Tags        []string
}

type Rule struct {
	If          string
	When        string
	Changes     []string
	Variables   map[string]string
	AllowFailure *bool
}

type OnlyExcept struct {
	Refs       []string
	Variables  []string
	Changes    []string
	Kubernetes bool
}

type Include struct {
	Type     string // local, file, remote, template, project
	Path     string
	Project  string
	Ref      string
	File     string
	Content  string
	Pipeline *Pipeline
}

type Pipeline struct {
	Jobs       map[string]*Job
	Stages     []string
	Variables  map[string]string
	Workflow   map[string]interface{}
	Default    map[string]interface{}
	Includes   []*Include
	FilePath   string
}

func ParseFile(filePath string) (*Pipeline, error) {
	return ParseFileWithIncludes(filePath, true)
}

func ParseFileWithIncludes(filePath string, processIncludes bool) (*Pipeline, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	content, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var data map[string]interface{}
	if err := yaml.Unmarshal(content, &data); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	pipeline := &Pipeline{
		Jobs:      make(map[string]*Job),
		Variables: make(map[string]string),
		FilePath:  filePath,
	}

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
	
	if processIncludes {
		if includesInterface, ok := data["include"]; ok {
			if includeMap, ok := includesInterface.(map[string]interface{}); ok {
				include := processIncludeMap(includeMap, filePath)
				pipeline.Includes = append(pipeline.Includes, include)
			}
			
			if includesSlice, ok := includesInterface.([]interface{}); ok {
				for _, includeInterface := range includesSlice {
					if includeStr, ok := includeInterface.(string); ok {
						include := &Include{
							Type: "local",
							Path: includeStr,
						}
						pipeline.Includes = append(pipeline.Includes, include)
					}
					
					if includeMap, ok := includeInterface.(map[string]interface{}); ok {
						include := processIncludeMap(includeMap, filePath)
						pipeline.Includes = append(pipeline.Includes, include)
					}
				}
			}
			
			for _, include := range pipeline.Includes {
				if include.Type == "local" {
					includePath := include.Path
					if !isAbsolutePath(includePath) {
						dir := getDirectoryPath(filePath)
						includePath = dir + "/" + includePath
					}
					
					includedPipeline, err := ParseFileWithIncludes(includePath, true)
					if err == nil {
						include.Content = getFileContent(includePath)
						include.Pipeline = includedPipeline
					}
				}
			}
		}
	}

	for key, value := range data {
		if key == "stages" || key == "variables" || key == "workflow" || key == "default" {
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
