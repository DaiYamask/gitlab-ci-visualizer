package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/DaiYamask/gitlab-ci-visualizer/internal/parser"
	"github.com/DaiYamask/gitlab-ci-visualizer/internal/visualizer"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	Version = "0.1.0"
	
	noColor          bool
	stageFlag        string
	ruleFlag         string
	formatFlag       string
	dependenciesFlag bool
	expandVarsFlag   bool
	ciVarsFlag       string
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
			
			ciVars := make(map[string]string)
			if ciVarsFlag != "" {
				for _, pair := range strings.Split(ciVarsFlag, ",") {
					parts := strings.SplitN(pair, "=", 2)
					if len(parts) == 2 {
						ciVars[parts[0]] = parts[1]
					}
				}
			}
			
			opts := visualizer.Options{
				StageFilter:      stageFlag,
				RuleFilter:       ruleFlag,
				Format:           format,
				ShowDependencies: dependenciesFlag,
				ExpandVars:       expandVarsFlag,
				CIVars:           ciVars,
			}
			
			visualizer.Visualize(pipeline, opts)
		},
	}
	
	showCmd.Flags().StringVar(&stageFlag, "stage", "", "Filter jobs by stage")
	showCmd.Flags().StringVar(&ruleFlag, "rule", "", "Filter jobs by rule condition")
	showCmd.Flags().StringVar(&formatFlag, "format", "text", "Output format (text, json, yaml, table)")
	showCmd.Flags().BoolVar(&dependenciesFlag, "dependencies", false, "Show job dependencies")
	showCmd.Flags().BoolVar(&expandVarsFlag, "expand-vars", false, "Expand environment variables in output")
	showCmd.Flags().StringVar(&ciVarsFlag, "ci-vars", "", "Comma-separated list of CI variable values (e.g. CI_COMMIT_BRANCH=main,CI_PIPELINE_SOURCE=push)")

	rootCmd.AddCommand(showCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
