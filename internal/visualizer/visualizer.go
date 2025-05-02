package visualizer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/DaiYamask/gitlab-ci-visualizer/internal/parser"
	"github.com/fatih/color"
)

type Options struct {
	StageFilter string // Filter jobs by stage
	RuleFilter  string // Filter jobs by rule condition
}

func Visualize(pipeline *parser.Pipeline, opts Options) {
	titleColor := color.New(color.FgHiWhite, color.Bold)
	sectionColor := color.New(color.FgYellow, color.Bold)
	
	titleColor.Println("GitLab CI Pipeline Visualization")
	titleColor.Println("===============================")

	sectionColor.Println("\nStages:")
	for i, stage := range pipeline.Stages {
		fmt.Printf("  %d. %s\n", i+1, stage)
	}

	if workflow, ok := pipeline.Workflow["rules"]; ok {
		if rules, ok := workflow.([]interface{}); ok && len(rules) > 0 {
			sectionColor.Println("\nWorkflow Rules:")
			for i, rule := range rules {
				if ruleMap, ok := rule.(map[string]interface{}); ok {
					fmt.Printf("  %d. ", i+1)
					parts := []string{}
					
					if ifCond, ok := ruleMap["if"].(string); ok {
						parts = append(parts, fmt.Sprintf("if: %s", color.GreenString(ifCond)))
					}
					if when, ok := ruleMap["when"].(string); ok {
						parts = append(parts, fmt.Sprintf("when: %s", color.MagentaString(when)))
					}
					
					fmt.Println(strings.Join(parts, ", "))
				}
			}
		}
	}

	filteredJobs := make(map[string]*parser.Job)
	for name, job := range pipeline.Jobs {
		if opts.StageFilter != "" && job.Stage != opts.StageFilter {
			continue
		}
		filteredJobs[name] = job
	}

	sectionColor.Println("\nJobs by Rules:")
	
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
	
	if len(jobsWithoutRules) > 0 && opts.RuleFilter == "" {
		fmt.Println("\n  " + color.HiCyanString("Always Run (No Rules):"))
		
		sort.Slice(jobsWithoutRules, func(i, j int) bool {
			if jobsWithoutRules[i].Stage != jobsWithoutRules[j].Stage {
				return getStageIndex(pipeline.Stages, jobsWithoutRules[i].Stage) < 
				       getStageIndex(pipeline.Stages, jobsWithoutRules[j].Stage)
			}
			return jobsWithoutRules[i].Name < jobsWithoutRules[j].Name
		})
		
		for _, job := range jobsWithoutRules {
			printJob(job)
		}
	}
	
	var conditions []string
	for condition := range jobsByCondition {
		conditions = append(conditions, condition)
	}
	sort.Strings(conditions)
	
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
		
		fmt.Printf("\n  Rule Condition: %s\n", color.GreenString(condition))
		for _, job := range jobs {
			printJob(job)
		}
	}
	
	if opts.StageFilter != "" || opts.RuleFilter != "" {
		fmt.Println("\nFilters applied:")
		if opts.StageFilter != "" {
			fmt.Printf("  Stage: %s\n", color.YellowString(opts.StageFilter))
		}
		if opts.RuleFilter != "" {
			fmt.Printf("  Rule: %s\n", color.YellowString(opts.RuleFilter))
		}
	}
}

// getStageIndex returns the index of a stage in the stages slice
// If the stage is not found, it returns a large number to put it at the end
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
}
