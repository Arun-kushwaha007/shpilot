package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"encoding/json"

	"github.com/go-resty/resty/v2"
	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "shpilot [query]",
		Short: "shpilot suggests shell commands with AI",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			userInput := args[0]
			fmt.Println("→ You typed:", userInput)

			// Gather context
			inGitRepo := false
			out, err := exec.Command("git", "rev-parse", "--is-inside-work-tree").Output()
			if err == nil && strings.TrimSpace(string(out)) == "true" {
				inGitRepo = true
				fmt.Println("→ Context: You are inside a Git repository.")
			} else {
				fmt.Println("→ Context: You are NOT inside a Git repository.")
			}

			// List files
			filesList := []string{}
			entries, err := os.ReadDir(".")
			if err == nil {
				for _, entry := range entries {
					filesList = append(filesList, entry.Name())
				}
			}

			// Check for Dockerfile
			dockerfileExists := false
			if _, err := os.Stat("Dockerfile"); err == nil {
				dockerfileExists = true
				fmt.Println("→ Context: Found Dockerfile.")
			} else {
				fmt.Println("→ Context: No Dockerfile found.")
			}

			// Compose context message
			contextMsg := fmt.Sprintf(
				"User input: %s. Context: inside a Git repo = %t. Files: %s. Dockerfile present = %t.",
				userInput,
				inGitRepo,
				strings.Join(filesList, ", "),
				dockerfileExists,
			)

			// Send to OpenAI
			suggestion, err := getAISuggestion(contextMsg)
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

func getAISuggestion(prompt string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY not set")
	}

	client := resty.New()
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", "Bearer "+apiKey).
		SetBody(map[string]interface{}{
			"model": "gpt-3.5-turbo",
			"messages": []map[string]string{
				{
					"role":    "system",
					"content": "You are a shell command assistant. Suggest correct shell commands based on user input and context. Keep answers concise.",
				},
				{
					"role":    "user",
					"content": prompt,
				},
			},
		}).
		Post("https://api.openai.com/v1/chat/completions")

	if err != nil {
		return "", err
	}

	type Choice struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}

	type OpenAIResponse struct {
		Choices []Choice `json:"choices"`
	}

	var aiResp OpenAIResponse
	fmt.Println("→ Raw AI response body:", resp.String())
	err = json.Unmarshal(resp.Body(), &aiResp)
	if err != nil {
		return "", err
	}

	if len(aiResp.Choices) > 0 {
		return aiResp.Choices[0].Message.Content, nil
	}

	return "No suggestion available.", nil
}
