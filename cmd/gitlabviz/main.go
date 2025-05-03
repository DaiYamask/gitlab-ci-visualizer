package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/DaiYamask/gitlab-ci-visualizer/internal/parser"
	"github.com/DaiYamask/gitlab-ci-visualizer/internal/visualizer"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	Version = "0.1.0"
	
	noColor          bool
	stageFlag        string
	ruleFlag         string
	formatFlag       string
	dependenciesFlag bool
	templateFormatFlag string
)

func main() {
	var rootCmd = &cobra.Command{
		Use:     "gitlabviz",
		Short:   "GitLab CI Visualizer - A tool to visualize GitLab CI pipelines",
		Version: Version,
		Long: `GitLab CI Visualizer is a CLI tool that helps you visualize GitLab CI pipelines.
It parses .gitlab-ci.yml files and shows which jobs will be executed based on different rules.

Examples:
  gitlabviz show .gitlab-ci.yml
  gitlabviz show --no-color path/to/.gitlab-ci.yml
  gitlabviz show --stage test path/to/.gitlab-ci.yml
  gitlabviz show --rule '$CI_COMMIT_BRANCH == "main"' path/to/.gitlab-ci.yml
  gitlabviz show --dependencies path/to/.gitlab-ci.yml
  gitlabviz show --dependencies --format table path/to/.gitlab-ci.yml`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if noColor {
				color.NoColor = true
			}
		},
	}

	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable color output")

	var showCmd = &cobra.Command{
		Use:   "show [file]",
		Short: "Show visualization of GitLab CI pipeline",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			filePath := args[0]
			
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				fmt.Printf("Error: File '%s' does not exist\n", filePath)
				os.Exit(1)
			}
			
			pipeline, err := parser.ParseFile(filePath)
			if err != nil {
				fmt.Printf("Error parsing file: %v\n", err)
				os.Exit(1)
			}
			
			var format visualizer.OutputFormat
			switch formatFlag {
			case "json":
				format = visualizer.FormatJSON
			case "yaml":
				format = visualizer.FormatYAML
			case "table":
				format = visualizer.FormatTable
			default:
				format = visualizer.FormatText
			}
			
			opts := visualizer.Options{
				StageFilter:      stageFlag,
				RuleFilter:       ruleFlag,
				Format:           format,
				ShowDependencies: dependenciesFlag,
			}
			
			visualizer.Visualize(pipeline, opts)
		},
	}
	
	showCmd.Flags().StringVar(&stageFlag, "stage", "", "Filter jobs by stage")
	showCmd.Flags().StringVar(&ruleFlag, "rule", "", "Filter jobs by rule condition")
	showCmd.Flags().StringVar(&formatFlag, "format", "text", "Output format (text, json, yaml, table)")
	showCmd.Flags().BoolVar(&dependenciesFlag, "dependencies", false, "Show job dependencies")

	var templateCmd = &cobra.Command{
		Use:   "template [file]",
		Short: "Show included templates in GitLab CI pipeline",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			filePath := args[0]
			
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				fmt.Printf("Error: File '%s' does not exist\n", filePath)
				os.Exit(1)
			}
			
			pipeline, err := parser.ParseFile(filePath)
			if err != nil {
				fmt.Printf("Error parsing file: %v\n", err)
				os.Exit(1)
			}
			
			err = parser.LoadIncludedTemplates(pipeline)
			if err != nil {
				fmt.Printf("Error loading templates: %v\n", err)
				os.Exit(1)
			}
			
			switch templateFormatFlag {
			case "json":
				displayTemplateJSON(pipeline)
			case "yaml":
				displayTemplateYAML(pipeline)
			case "table":
				displayTemplateTable(pipeline)
			default:
				fmt.Println("GitLab CI Template Hierarchy")
				fmt.Println("============================")
				fmt.Println()
				fmt.Print(parser.DisplayTemplateHierarchy(pipeline, "", nil))
				fmt.Println()
				
				fmt.Println("Template Contents")
				fmt.Println("=================")
				fmt.Println()
				
				displayTemplateContents(pipeline, 0)
			}
		},
	}
	
	templateCmd.Flags().StringVar(&templateFormatFlag, "format", "text", "Output format (text, json, yaml, table)")
	
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(templateCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func displayTemplateContents(pipeline *parser.Pipeline, level int) {
	if pipeline == nil {
		return
	}
	
	for _, include := range pipeline.Includes {
		fmt.Printf("Template: %s (%s)\n", include.Path, include.Type)
		fmt.Println(strings.Repeat("-", len("Template: "+include.Path+" ("+include.Type+")")))
		fmt.Println(include.Content)
		fmt.Println()
		
		if include.Pipeline != nil {
			displayTemplateContents(include.Pipeline, level+1)
		}
	}
}

type TemplateInfo struct {
	Path     string            `json:"path" yaml:"path"`
	Type     string            `json:"type" yaml:"type"`
	Content  string            `json:"content" yaml:"content"`
	Includes []TemplateInfo    `json:"includes,omitempty" yaml:"includes,omitempty"`
}

func buildTemplateInfoTree(pipeline *parser.Pipeline) []TemplateInfo {
	if pipeline == nil || len(pipeline.Includes) == 0 {
		return nil
	}
	
	result := []TemplateInfo{}
	
	for _, include := range pipeline.Includes {
		info := TemplateInfo{
			Path:    include.Path,
			Type:    include.Type,
			Content: include.Content,
		}
		
		if include.Pipeline != nil {
			info.Includes = buildTemplateInfoTree(include.Pipeline)
		}
		
		result = append(result, info)
	}
	
	return result
}

func displayTemplateJSON(pipeline *parser.Pipeline) {
	rootInfo := TemplateInfo{
		Path:     pipeline.FilePath,
		Type:     "root",
		Content:  "",
		Includes: buildTemplateInfoTree(pipeline),
	}
	
	jsonData, err := json.MarshalIndent(rootInfo, "", "  ")
	if err != nil {
		fmt.Printf("Error generating JSON: %v\n", err)
		return
	}
	
	fmt.Println(string(jsonData))
}

func displayTemplateYAML(pipeline *parser.Pipeline) {
	rootInfo := TemplateInfo{
		Path:     pipeline.FilePath,
		Type:     "root",
		Content:  "",
		Includes: buildTemplateInfoTree(pipeline),
	}
	
	yamlData, err := yaml.Marshal(rootInfo)
	if err != nil {
		fmt.Printf("Error generating YAML: %v\n", err)
		return
	}
	
	fmt.Println(string(yamlData))
}

func displayTemplateTable(pipeline *parser.Pipeline) {
	fmt.Println("| Template Path | Type | Included From |")
	fmt.Println("|--------------|------|---------------|")
	
	displayTemplateTableRows(pipeline, "")
}

func displayTemplateTableRows(pipeline *parser.Pipeline, parent string) {
	if pipeline == nil {
		return
	}
	
	for _, include := range pipeline.Includes {
		fmt.Printf("| %s | %s | %s |\n", include.Path, include.Type, parent)
		
		if include.Pipeline != nil {
			displayTemplateTableRows(include.Pipeline, include.Path)
		}
	}
}
