package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "shpilot [query]",
		Short: "shpilot suggests shell commands with AI (Gemini-powered)",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			userInput := args[0]
			fmt.Println("→ You typed:", userInput)

			// Context: inside Git repo?
			inGitRepo := false
			out, err := exec.Command("git", "rev-parse", "--is-inside-work-tree").Output()
			if err == nil && strings.TrimSpace(string(out)) == "true" {
				inGitRepo = true
				fmt.Println("→ Context: You are inside a Git repository.")
			} else {
				fmt.Println("→ Context: You are NOT inside a Git repository.")
			}

			// List current directory files
			filesList := []string{}
			entries, err := os.ReadDir(".")
			if err == nil {
				for _, entry := range entries {
					filesList = append(filesList, entry.Name())
				}
			}

			// Dockerfile check
			dockerfileExists := false
			if _, err := os.Stat("Dockerfile"); err == nil {
				dockerfileExists = true
				fmt.Println("→ Context: Found Dockerfile.")
			} else {
				fmt.Println("→ Context: No Dockerfile found.")
			}

			// Compose prompt
			contextMsg := fmt.Sprintf(
				"User input: %s. Context: inside a Git repo = %t. Files: %s. Dockerfile present = %t.",
				userInput, inGitRepo, strings.Join(filesList, ", "), dockerfileExists,
			)

			// Call Gemini API
			suggestion, err := getGeminiSuggestion(contextMsg)
			if err != nil {
				fmt.Println("Error contacting AI:", err)
				return
			}
			fmt.Println("→ AI Suggestion:", suggestion)
		},
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func getGeminiSuggestion(prompt string) (string, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY not set")
	}

	client := resty.New()
	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent?key=" + apiKey

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": "You are a helpful assistant that suggests accurate shell commands based on input and context."},
				},
				"role": "user",
			},
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
				"role": "user",
			},
		},
	}

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(requestBody).
		Post(url)

	if err != nil {
		return "", err
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	fmt.Println("→ Raw AI response body:", resp.String())

	err = json.Unmarshal(resp.Body(), &result)
	if err != nil {
		return "", err
	}

	if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
		return result.Candidates[0].Content.Parts[0].Text, nil
	}

	return "No suggestion available.", nil
}
