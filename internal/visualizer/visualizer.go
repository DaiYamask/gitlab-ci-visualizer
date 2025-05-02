package visualizer

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/DaiYamask/gitlab-ci-visualizer/internal/parser"
	"github.com/fatih/color"
	"gopkg.in/yaml.v3"
)

type OutputFormat string

const (
	FormatText  OutputFormat = "text"
	FormatJSON  OutputFormat = "json"
	FormatYAML  OutputFormat = "yaml"
	FormatTable OutputFormat = "table"
)

type Options struct {
	StageFilter      string            // Filter jobs by stage
	RuleFilter       string            // Filter jobs by rule condition
	Format           OutputFormat      // Output format (text, json, yaml, table)
	ShowDependencies bool              // Show job dependencies
	ExpandVars       bool              // Expand environment variables
	CIVars           map[string]string // User-provided values for CI_ variables
}

type PipelineOutput struct {
	Name         string                 `json:"name" yaml:"name"`
	Stages       []string               `json:"stages" yaml:"stages"`
	WorkflowRules []map[string]string   `json:"workflow_rules,omitempty" yaml:"workflow_rules,omitempty"`
	JobsByRule   map[string][]JobOutput `json:"jobs_by_rule" yaml:"jobs_by_rule"`
	Filters      map[string]string      `json:"filters,omitempty" yaml:"filters,omitempty"`
}

type JobOutput struct {
	Name         string      `json:"name" yaml:"name"`
	Stage        string      `json:"stage" yaml:"stage"`
	Rules        []RuleOutput `json:"rules,omitempty" yaml:"rules,omitempty"`
	Dependencies []string    `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	Needs        []string    `json:"needs,omitempty" yaml:"needs,omitempty"`
}

type RuleOutput struct {
	If      string   `json:"if,omitempty" yaml:"if,omitempty"`
	When    string   `json:"when,omitempty" yaml:"when,omitempty"`
	Changes []string `json:"changes,omitempty" yaml:"changes,omitempty"`
}

func Visualize(pipeline *parser.Pipeline, opts Options) {
	// If expand-vars is enabled, create a copy of the pipeline with expanded variables
	if opts.ExpandVars {
		expandedPipeline := &parser.Pipeline{
			Jobs:      make(map[string]*parser.Job),
			Stages:    make([]string, len(pipeline.Stages)),
			Variables: make(map[string]string),
			Workflow:  pipeline.Workflow,
			Default:   pipeline.Default,
			Includes:  pipeline.Includes,
		}
		
		// Copy stages and variables
		copy(expandedPipeline.Stages, pipeline.Stages)
		for k, v := range pipeline.Variables {
			expandedPipeline.Variables[k] = v
		}
		
		// Copy jobs with expanded variables
		for name, job := range pipeline.Jobs {
			expandedJob := &parser.Job{
				Name:         job.Name,
				Stage:        job.Stage,
				Script:       make([]string, len(job.Script)),
				Rules:        make([]parser.Rule, len(job.Rules)),
				Only:         job.Only,
				Except:       job.Except,
				When:         job.When,
				AllowFailure: job.AllowFailure,
				Dependencies: make([]string, len(job.Dependencies)),
				Needs:        make([]string, len(job.Needs)),
				Tags:         make([]string, len(job.Tags)),
			}
			
			for i, script := range job.Script {
				expandedJob.Script[i] = expandVars(script, pipeline, opts.CIVars)
			}
			
			for i, rule := range job.Rules {
				expandedRule := parser.Rule{
					If:          expandVars(rule.If, pipeline, opts.CIVars),
					When:        rule.When,
					Changes:     make([]string, len(rule.Changes)),
					Variables:   make(map[string]string),
					AllowFailure: rule.AllowFailure,
				}
				
				for j, change := range rule.Changes {
					expandedRule.Changes[j] = expandVars(change, pipeline, opts.CIVars)
				}
				
				// Copy and expand variables
				for k, v := range rule.Variables {
					expandedRule.Variables[k] = expandVars(v, pipeline, opts.CIVars)
				}
				
				expandedJob.Rules[i] = expandedRule
			}
			
			copy(expandedJob.Dependencies, job.Dependencies)
			copy(expandedJob.Needs, job.Needs)
			copy(expandedJob.Tags, job.Tags)
			
			expandedPipeline.Jobs[name] = expandedJob
		}
		
		pipeline = expandedPipeline
	}
	
	filteredJobs := make(map[string]*parser.Job)
	for name, job := range pipeline.Jobs {
		if opts.StageFilter != "" && job.Stage != opts.StageFilter {
			continue
		}
		filteredJobs[name] = job
	}
	
	// Group jobs by rule condition
	jobsByCondition := make(map[string]map[string]bool) // condition -> job name -> exists
	jobsWithoutRules := []*parser.Job{}
	
	for _, job := range filteredJobs {
		if len(job.Rules) == 0 {
			jobsWithoutRules = append(jobsWithoutRules, job)
		} else {
			for _, rule := range job.Rules {
				condition := "default"
				if rule.If != "" {
					condition = rule.If
				}
				
				if opts.RuleFilter != "" && condition != opts.RuleFilter {
					continue
				}
				
				if _, ok := jobsByCondition[condition]; !ok {
					jobsByCondition[condition] = make(map[string]bool)
				}
				
				jobsByCondition[condition][job.Name] = true
			}
		}
	}
	
	sort.Slice(jobsWithoutRules, func(i, j int) bool {
		if jobsWithoutRules[i].Stage != jobsWithoutRules[j].Stage {
			return getStageIndex(pipeline.Stages, jobsWithoutRules[i].Stage) < 
				   getStageIndex(pipeline.Stages, jobsWithoutRules[j].Stage)
		}
		return jobsWithoutRules[i].Name < jobsWithoutRules[j].Name
	})
	
	var conditions []string
	for condition := range jobsByCondition {
		conditions = append(conditions, condition)
	}
	sort.Strings(conditions)
	
	// Prepare jobs by condition
	jobsByConditionSorted := make(map[string][]*parser.Job)
	for _, condition := range conditions {
		jobNames := jobsByCondition[condition]
		
		var jobs []*parser.Job
		for jobName := range jobNames {
			if job, ok := pipeline.Jobs[jobName]; ok {
				jobs = append(jobs, job)
			}
		}
		
		sort.Slice(jobs, func(i, j int) bool {
			if jobs[i].Stage != jobs[j].Stage {
				return getStageIndex(pipeline.Stages, jobs[i].Stage) < 
					   getStageIndex(pipeline.Stages, jobs[j].Stage)
			}
			return jobs[i].Name < jobs[j].Name
		})
		
		jobsByConditionSorted[condition] = jobs
	}
	
	var workflowRules []map[string]string
	if workflow, ok := pipeline.Workflow["rules"]; ok {
		if rules, ok := workflow.([]interface{}); ok && len(rules) > 0 {
			for _, rule := range rules {
				if ruleMap, ok := rule.(map[string]interface{}); ok {
					r := make(map[string]string)
					
					if ifCond, ok := ruleMap["if"].(string); ok {
						r["if"] = ifCond
					}
					if when, ok := ruleMap["when"].(string); ok {
						r["when"] = when
					}
					
					workflowRules = append(workflowRules, r)
				}
			}
		}
	}
	
	filters := make(map[string]string)
	if opts.StageFilter != "" {
		filters["stage"] = opts.StageFilter
	}
	if opts.RuleFilter != "" {
		filters["rule"] = opts.RuleFilter
	}
	if opts.ShowDependencies {
		filters["dependencies"] = "true"
	}
	if opts.ExpandVars {
		filters["expand-vars"] = "true"
	}
	
	switch opts.Format {
	case FormatJSON:
		outputJSON(pipeline, workflowRules, jobsWithoutRules, jobsByConditionSorted, filters)
	case FormatYAML:
		outputYAML(pipeline, workflowRules, jobsWithoutRules, jobsByConditionSorted, filters)
	case FormatTable:
		outputTable(pipeline, workflowRules, jobsWithoutRules, jobsByConditionSorted, filters)
	default:
		outputText(pipeline, workflowRules, jobsWithoutRules, jobsByConditionSorted, filters)
	}
}

// getStageIndex returns the index of a stage in the stages slice
// If the stage is not found, it returns a large number to put it at the end
func outputJSON(pipeline *parser.Pipeline, workflowRules []map[string]string, jobsWithoutRules []*parser.Job, jobsByCondition map[string][]*parser.Job, filters map[string]string) {
	output := buildPipelineOutput(pipeline, workflowRules, jobsWithoutRules, jobsByCondition, filters)
	
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Printf("Error generating JSON: %v\n", err)
		return
	}
	
	fmt.Println(string(jsonData))
}

func outputYAML(pipeline *parser.Pipeline, workflowRules []map[string]string, jobsWithoutRules []*parser.Job, jobsByCondition map[string][]*parser.Job, filters map[string]string) {
	output := buildPipelineOutput(pipeline, workflowRules, jobsWithoutRules, jobsByCondition, filters)
	
	yamlData, err := yaml.Marshal(output)
	if err != nil {
		fmt.Printf("Error generating YAML: %v\n", err)
		return
	}
	
	fmt.Println(string(yamlData))
}

func outputTable(pipeline *parser.Pipeline, workflowRules []map[string]string, jobsWithoutRules []*parser.Job, jobsByCondition map[string][]*parser.Job, filters map[string]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	
	if filters["dependencies"] != "" {
		fmt.Fprintln(w, "JOB NAME\tSTAGE\tRULE CONDITION\tWHEN\tNEEDS\tDEPENDENCIES")
		fmt.Fprintln(w, "--------\t-----\t--------------\t----\t-----\t------------")
	} else {
		fmt.Fprintln(w, "JOB NAME\tSTAGE\tRULE CONDITION\tWHEN")
		fmt.Fprintln(w, "--------\t-----\t--------------\t----")
	}
	
	for _, job := range jobsWithoutRules {
		stage := job.Stage
		if stage == "" {
			stage = "test"
		}
		
		if filters["dependencies"] != "" {
			needs := "none"
			if len(job.Needs) > 0 {
				needs = strings.Join(job.Needs, ", ")
			}
			
			deps := "none"
			if len(job.Dependencies) > 0 {
				deps = strings.Join(job.Dependencies, ", ")
			}
			
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", job.Name, stage, "No Rules", "always", needs, deps)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", job.Name, stage, "No Rules", "always")
		}
	}
	
	for condition, jobs := range jobsByCondition {
		for _, job := range jobs {
			stage := job.Stage
			if stage == "" {
				stage = "test"
			}
			
			when := "always"
			if len(job.Rules) > 0 && job.Rules[0].When != "" {
				when = job.Rules[0].When
			}
			
			if filters["dependencies"] != "" {
				needs := "none"
				if len(job.Needs) > 0 {
					needs = strings.Join(job.Needs, ", ")
				}
				
				deps := "none"
				if len(job.Dependencies) > 0 {
					deps = strings.Join(job.Dependencies, ", ")
				}
				
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", job.Name, stage, condition, when, needs, deps)
			} else {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", job.Name, stage, condition, when)
			}
		}
	}
	
	w.Flush()
	
	if len(workflowRules) > 0 {
		fmt.Println("\nWORKFLOW RULES:")
		fmt.Println("-------------")
		for i, rule := range workflowRules {
			parts := []string{}
			if ifCond, ok := rule["if"]; ok {
				parts = append(parts, fmt.Sprintf("if: %s", ifCond))
			}
			if when, ok := rule["when"]; ok {
				parts = append(parts, fmt.Sprintf("when: %s", when))
			}
			fmt.Printf("%d. %s\n", i+1, strings.Join(parts, ", "))
		}
	}
	
	if len(filters) > 0 {
		fmt.Println("\nFILTERS APPLIED:")
		fmt.Println("---------------")
		for k, v := range filters {
			fmt.Printf("%s: %s\n", k, v)
		}
	}
	
	if _, ok := filters["expand-vars"]; ok {
		fmt.Println("\nNOTE: Environment variables have been expanded.")
		fmt.Println("CI_ variables without provided values are shown as <VARIABLE_NAME>.")
	}
}

func expandVars(s string, pipeline *parser.Pipeline, ciVars map[string]string) string {
	for name, value := range pipeline.Variables {
		s = strings.ReplaceAll(s, "$"+name, value)
		s = strings.ReplaceAll(s, "${"+name+"}", value)
	}
	
	// Then, handle CI_ variables
	for name, value := range ciVars {
		s = strings.ReplaceAll(s, "$"+name, value)
		s = strings.ReplaceAll(s, "${"+name+"}", value)
	}
	
	re := regexp.MustCompile(`\$\{?(CI_[A-Z_]+)\}?`)
	s = re.ReplaceAllStringFunc(s, func(match string) string {
		varName := re.FindStringSubmatch(match)[1]
		return "<" + varName + ">"
	})
	
	return s
}

func outputText(pipeline *parser.Pipeline, workflowRules []map[string]string, jobsWithoutRules []*parser.Job, jobsByCondition map[string][]*parser.Job, filters map[string]string) {
	titleColor := color.New(color.FgHiWhite, color.Bold)
	sectionColor := color.New(color.FgYellow, color.Bold)
	
	titleColor.Println("GitLab CI Pipeline Visualization")
	titleColor.Println("===============================")

	sectionColor.Println("\nStages:")
	for i, stage := range pipeline.Stages {
		fmt.Printf("  %d. %s\n", i+1, stage)
	}

	if len(workflowRules) > 0 {
		sectionColor.Println("\nWorkflow Rules:")
		for i, rule := range workflowRules {
			fmt.Printf("  %d. ", i+1)
			parts := []string{}
			
			if ifCond, ok := rule["if"]; ok {
				parts = append(parts, fmt.Sprintf("if: %s", color.GreenString(ifCond)))
			}
			if when, ok := rule["when"]; ok {
				parts = append(parts, fmt.Sprintf("when: %s", color.MagentaString(when)))
			}
			
			fmt.Println(strings.Join(parts, ", "))
		}
	}

	sectionColor.Println("\nJobs by Rules:")
	
	if len(jobsWithoutRules) > 0 && filters["rule"] == "" {
		fmt.Println("\n  " + color.HiCyanString("Always Run (No Rules):"))
		for _, job := range jobsWithoutRules {
			printJob(job)
		}
	}
	
	for condition, jobs := range jobsByCondition {
		fmt.Printf("\n  Rule Condition: %s\n", color.GreenString(condition))
		for _, job := range jobs {
			printJob(job)
		}
	}
	
	if len(filters) > 0 {
		fmt.Println("\nFilters applied:")
		if stageFilter, ok := filters["stage"]; ok {
			fmt.Printf("  Stage: %s\n", color.YellowString(stageFilter))
		}
		if ruleFilter, ok := filters["rule"]; ok {
			fmt.Printf("  Rule: %s\n", color.YellowString(ruleFilter))
		}
		if _, ok := filters["dependencies"]; ok {
			fmt.Printf("  Dependencies: %s\n", color.YellowString("true"))
		}
		if _, ok := filters["expand-vars"]; ok {
			fmt.Printf("  Expand Variables: %s\n", color.YellowString("true"))
		}
	}
}

func buildPipelineOutput(pipeline *parser.Pipeline, workflowRules []map[string]string, jobsWithoutRules []*parser.Job, jobsByCondition map[string][]*parser.Job, filters map[string]string) PipelineOutput {
	output := PipelineOutput{
		Name:         "GitLab CI Pipeline",
		Stages:       pipeline.Stages,
		WorkflowRules: workflowRules,
		JobsByRule:   make(map[string][]JobOutput),
		Filters:      filters,
	}
	
	if len(jobsWithoutRules) > 0 {
		noRuleJobs := []JobOutput{}
		for _, job := range jobsWithoutRules {
			jobOutput := convertJobToOutput(job)
			noRuleJobs = append(noRuleJobs, jobOutput)
		}
		output.JobsByRule["No Rules"] = noRuleJobs
	}
	
	for condition, jobs := range jobsByCondition {
		conditionJobs := []JobOutput{}
		for _, job := range jobs {
			jobOutput := convertJobToOutput(job)
			conditionJobs = append(conditionJobs, jobOutput)
		}
		output.JobsByRule[condition] = conditionJobs
	}
	
	return output
}

func convertJobToOutput(job *parser.Job) JobOutput {
	stage := job.Stage
	if stage == "" {
		stage = "test" // Default stage in GitLab CI
	}
	
	jobOutput := JobOutput{
		Name:         job.Name,
		Stage:        stage,
		Rules:        []RuleOutput{},
		Dependencies: job.Dependencies,
		Needs:        job.Needs,
	}
	
	for _, rule := range job.Rules {
		ruleOutput := RuleOutput{
			If:      rule.If,
			When:    rule.When,
			Changes: rule.Changes,
		}
		jobOutput.Rules = append(jobOutput.Rules, ruleOutput)
	}
	
	return jobOutput
}

func getStageIndex(stages []string, stage string) int {
	if stage == "" {
		stage = "test" // Default stage in GitLab CI
	}
	
	for i, s := range stages {
		if s == stage {
			return i
		}
	}
	return len(stages) // Not found, put at the end
}

func printJob(job *parser.Job) {
	stage := job.Stage
	if stage == "" {
		stage = "test" // Default stage in GitLab CI
	}
	
	fmt.Printf("    - %s (%s)\n", color.BlueString(job.Name), stage)
	
	if len(job.Rules) > 0 {
		for i, rule := range job.Rules {
			fmt.Printf("      Rule %d: ", i+1)
			
			parts := []string{}
			if rule.If != "" {
				parts = append(parts, fmt.Sprintf("if: %s", rule.If))
			}
			if rule.When != "" {
				parts = append(parts, fmt.Sprintf("when: %s", color.MagentaString(rule.When)))
			}
			if len(rule.Changes) > 0 {
				parts = append(parts, fmt.Sprintf("changes: %s", strings.Join(rule.Changes, ", ")))
			}
			
			fmt.Println(strings.Join(parts, ", "))
		}
	}
	
	if len(job.Needs) > 0 {
		fmt.Printf("      Needs: %s\n", color.YellowString(strings.Join(job.Needs, ", ")))
	}
	
	if len(job.Dependencies) > 0 {
		fmt.Printf("      Dependencies: %s\n", color.CyanString(strings.Join(job.Dependencies, ", ")))
	}
}
