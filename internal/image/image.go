package image

import (
	"fmt"
	"os"
	"strings"

	"github.com/DaiYamask/gitlab-ci-visualizer/internal/parser"
	"github.com/ajstarks/svgo"
)

type Options struct {
	OutputPath string
	Format     string
	Width      int
	Height     int
}

func DefaultOptions() Options {
	return Options{
		OutputPath: "pipeline.svg",
		Format:     "svg",
		Width:      800,
		Height:     600,
	}
}

func GeneratePipelineImage(pipeline *parser.Pipeline, opts Options) error {
	switch strings.ToLower(opts.Format) {
	case "svg":
		return generateSVG(pipeline, opts)
	default:
		return fmt.Errorf("unsupported image format: %s", opts.Format)
	}
}

func generateSVG(pipeline *parser.Pipeline, opts Options) error {
	f, err := os.Create(opts.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer f.Close()

	canvas := svg.New(f)
	canvas.Start(opts.Width, opts.Height)
	defer canvas.End()

	stageColor := "#4B7BEC"
	jobColor := "#45AAF2"
	ruleColor := "#26DE81"
	textColor := "#2C3E50"
	bgColor := "#F5F6FA"

	canvas.Rect(0, 0, opts.Width, opts.Height, "fill:"+bgColor)

	canvas.Text(opts.Width/2, 40, "GitLab CI Pipeline Visualization", 
		"text-anchor:middle;font-size:24px;font-weight:bold;fill:"+textColor)

	stageWidth := 120
	stageHeight := 60
	jobWidth := 180
	jobHeight := 40
	xMargin := 50
	yMargin := 100
	xSpacing := 200
	ySpacing := 80

	for i, stage := range pipeline.Stages {
		x := xMargin + i*xSpacing
		y := yMargin
		
		canvas.Rect(x, y, stageWidth, stageHeight, 
			"fill:"+stageColor+";stroke:none;rx:5")
		
		canvas.Text(x+stageWidth/2, y+stageHeight/2, stage, 
			"text-anchor:middle;font-size:14px;fill:white;dominant-baseline:middle")
		
		stageJobs := []*parser.Job{}
		for _, job := range pipeline.Jobs {
			if job.Stage == stage {
				stageJobs = append(stageJobs, job)
			}
		}
		
		for j, job := range stageJobs {
			jobX := x + stageWidth/2 - jobWidth/2
			jobY := y + stageHeight + ySpacing + j*jobHeight*2
			
			canvas.Rect(jobX, jobY, jobWidth, jobHeight, 
				"fill:"+jobColor+";stroke:none;rx:5")
			
			canvas.Text(jobX+jobWidth/2, jobY+jobHeight/2, job.Name, 
				"text-anchor:middle;font-size:12px;fill:white;dominant-baseline:middle")
			
			if j == 0 {
				canvas.Line(x+stageWidth/2, y+stageHeight, 
					x+stageWidth/2, jobY, 
					"stroke:#A4B0BE;stroke-width:2")
			}
			
			for k, rule := range job.Rules {
				ruleX := jobX + jobWidth + 20
				ruleY := jobY + k*20
				
				ruleText := ""
				if rule.If != "" {
					ruleText = "if: " + rule.If
				}
				if rule.When != "" {
					if ruleText != "" {
						ruleText += ", "
					}
					ruleText += "when: " + rule.When
				}
				
				canvas.Text(ruleX, ruleY+10, ruleText, 
					"font-size:10px;fill:"+ruleColor+";dominant-baseline:middle")
				
				canvas.Line(jobX+jobWidth, jobY+jobHeight/2, 
					ruleX-5, ruleY+10, 
					"stroke:#A4B0BE;stroke-width:1;stroke-dasharray:3,3")
			}
		}
	}

	return nil
}
