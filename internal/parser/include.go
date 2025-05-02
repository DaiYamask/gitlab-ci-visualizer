package parser

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

func processIncludeMap(includeMap map[string]interface{}, parentFilePath string) *Include {
	include := &Include{}
	
	if localPath, ok := includeMap["local"]; ok {
		if localPathStr, ok := localPath.(string); ok {
			include.Type = "local"
			include.Path = localPathStr
		}
	}
	
	if filePath, ok := includeMap["file"]; ok {
		if filePathStr, ok := filePath.(string); ok {
			include.Type = "file"
			include.Path = filePathStr
			include.File = filePathStr
		}
	}
	
	if remotePath, ok := includeMap["remote"]; ok {
		if remotePathStr, ok := remotePath.(string); ok {
			include.Type = "remote"
			include.Path = remotePathStr
		}
	}
	
	if templatePath, ok := includeMap["template"]; ok {
		if templatePathStr, ok := templatePath.(string); ok {
			include.Type = "template"
			include.Path = templatePathStr
		}
	}
	
	if projectPath, ok := includeMap["project"]; ok {
		if projectPathStr, ok := projectPath.(string); ok {
			include.Type = "project"
			include.Project = projectPathStr
			
			if ref, ok := includeMap["ref"]; ok {
				if refStr, ok := ref.(string); ok {
					include.Ref = refStr
				}
			}
			
			if file, ok := includeMap["file"]; ok {
				if fileStr, ok := file.(string); ok {
					include.File = fileStr
					include.Path = fileStr
				}
			}
		}
	}
	
	return include
}

func isAbsolutePath(path string) bool {
	return filepath.IsAbs(path)
}

func getDirectoryPath(filePath string) string {
	return filepath.Dir(filePath)
}

func getFileContent(filePath string) string {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Sprintf("Error reading file: %v", err)
	}
	return string(content)
}

func LoadIncludedTemplates(pipeline *Pipeline) error {
	for _, include := range pipeline.Includes {
		if include.Type == "local" {
			includePath := include.Path
			if !isAbsolutePath(includePath) {
				dir := getDirectoryPath(pipeline.FilePath)
				includePath = filepath.Join(dir, includePath)
			}
			
			if _, err := os.Stat(includePath); os.IsNotExist(err) {
				include.Content = fmt.Sprintf("Error: File '%s' does not exist", includePath)
				continue
			}
			
			includedPipeline, err := ParseFileWithIncludes(includePath, true)
			if err != nil {
				include.Content = fmt.Sprintf("Error parsing file: %v", err)
				continue
			}
			
			include.Content = getFileContent(includePath)
			include.Pipeline = includedPipeline
		} else if include.Type == "file" {
			includePath := include.Path
			if !isAbsolutePath(includePath) {
				dir := getDirectoryPath(pipeline.FilePath)
				includePath = filepath.Join(dir, includePath)
			}
			
			if _, err := os.Stat(includePath); os.IsNotExist(err) {
				include.Content = fmt.Sprintf("Error: File '%s' does not exist", includePath)
				continue
			}
			
			include.Content = getFileContent(includePath)
			includedPipeline, _ := ParseFileWithIncludes(includePath, true)
			include.Pipeline = includedPipeline
		} else if include.Type == "remote" || include.Type == "template" || include.Type == "project" {
			include.Content = fmt.Sprintf("Include type '%s' requires GitLab API access.\n", include.Type)
			if include.Type == "project" {
				include.Content += fmt.Sprintf("Project: %s\n", include.Project)
				if include.Ref != "" {
					include.Content += fmt.Sprintf("Ref: %s\n", include.Ref)
				}
				if include.File != "" {
					include.Content += fmt.Sprintf("File: %s\n", include.File)
				}
			} else {
				include.Content += fmt.Sprintf("Path: %s\n", include.Path)
			}
		}
	}
	
	return nil
}

func DisplayTemplateHierarchy(pipeline *Pipeline, indent string, visited map[string]bool) string {
	if pipeline == nil {
		return ""
	}
	
	if visited == nil {
		visited = make(map[string]bool)
	}
	
	if visited[pipeline.FilePath] {
		return indent + "└── " + filepath.Base(pipeline.FilePath) + " (circular reference)\n"
	}
	
	visited[pipeline.FilePath] = true
	
	result := indent + "└── " + filepath.Base(pipeline.FilePath) + "\n"
	
	if len(pipeline.Includes) > 0 {
		newIndent := indent + "    "
		for _, include := range pipeline.Includes {
			includeType := include.Type
			includePath := include.Path
			
			if include.Pipeline != nil {
				result += DisplayTemplateHierarchy(include.Pipeline, newIndent, visited)
			} else {
				result += newIndent + "└── " + includeType + ": " + includePath
				if include.Type == "project" {
					result += " (project: " + include.Project
					if include.Ref != "" {
						result += ", ref: " + include.Ref
					}
					if include.File != "" {
						result += ", file: " + include.File
					}
					result += ")"
				}
				result += "\n"
			}
		}
	}
	
	return result
}
