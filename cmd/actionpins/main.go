package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var immutableRevision = regexp.MustCompile(`^[0-9a-f]{40}$`)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: actionpins WORKFLOW...")
		os.Exit(2)
	}
	invalid := false
	for _, path := range os.Args[1:] {
		if err := checkWorkflow(path); err != nil {
			fmt.Fprintln(os.Stderr, err)
			invalid = true
		}
	}
	if invalid {
		os.Exit(1)
	}
}

func checkWorkflow(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	var problems []string
	for {
		var document yaml.Node
		err := decoder.Decode(&document)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("%s: invalid YAML: %w", path, err)
		}
		findUses(path, &document, &problems)
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "\n"))
	}
	return nil
}

func findUses(path string, node *yaml.Node, problems *[]string) {
	if node.Kind == yaml.AliasNode {
		*problems = append(*problems, fmt.Sprintf("%s:%d: YAML aliases are not supported in workflow action references", path, node.Line))
		return
	}
	if node.Kind == yaml.MappingNode {
		for index := 0; index+1 < len(node.Content); index += 2 {
			key, value := node.Content[index], node.Content[index+1]
			findUses(path, key, problems)
			if key.Kind == yaml.ScalarNode && key.Value == "uses" {
				if problem := validateUses(value); problem != "" {
					*problems = append(*problems, fmt.Sprintf("%s:%d: %s", path, value.Line, problem))
				}
			}
			findUses(path, value, problems)
		}
		return
	}
	for _, child := range node.Content {
		findUses(path, child, problems)
	}
}

func validateUses(node *yaml.Node) string {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return "action reference must be a plain or quoted string, not an alias or structured value"
	}
	reference := strings.TrimSpace(node.Value)
	if strings.HasPrefix(reference, "./") {
		return ""
	}
	separator := strings.LastIndexByte(reference, '@')
	if separator <= 0 || separator == len(reference)-1 || !immutableRevision.MatchString(reference[separator+1:]) {
		return fmt.Sprintf("mutable or unsupported action reference: %s", reference)
	}
	return ""
}
