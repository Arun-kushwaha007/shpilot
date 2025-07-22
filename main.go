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
	var showRaw bool

	var rootCmd = &cobra.Command{
		Use:   "shpilot [query]",
		Short: "shpilot suggests shell commands with AI (Gemini-powered)",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			userInput := args[0]
			fmt.Println("→ You typed:", userInput)

			// Context detection
			inGitRepo := false
			if out, err := exec.Command("git", "rev-parse", "--is-inside-work-tree").Output(); err == nil &&
				strings.TrimSpace(string(out)) == "true" {
				inGitRepo = true
				fmt.Println("→ Context: Inside a Git repo.")
			}

			filesList := []string{}
			if entries, err := os.ReadDir("."); err == nil {
				for _, entry := range entries {
					filesList = append(filesList, entry.Name())
				}
			}

			dockerfileExists := false
			if _, err := os.Stat("Dockerfile"); err == nil {
				dockerfileExists = true
				fmt.Println("→ Context: Dockerfile found.")
			}

			// Prompt
			contextMsg := fmt.Sprintf(`You are a shell assistant. 
User input: "%s"
Context:
- In Git repo: %t
- Files: %s
- Dockerfile: %t

Respond strictly in JSON with:
{
  "command": "shell command",
  "description": "what it does",
  "notes": "optional tips or edge cases"
}`,
				userInput, inGitRepo, strings.Join(filesList, ", "), dockerfileExists)

			suggestion, rawText, err := getGeminiSuggestion(contextMsg)
			if err != nil {
				fmt.Println("❌ Error:", err)
				return
			}

		
			fmt.Println("✅ AI Suggestion:")
			fmt.Println("🔹 Command    :", suggestion.Command)
			fmt.Println("🔹 Description:", suggestion.Description)
			fmt.Println("🔹 Notes      :", suggestion.Notes)
			if showRaw {
				fmt.Println("\n📝 Raw response:\n", rawText)
			}
		},
	}

	rootCmd.Flags().BoolVar(&showRaw, "raw", false, "Show raw Gemini output")
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type StructuredSuggestion struct {
	Command     string `json:"command"`
	Description string `json:"description"`
	Notes       string `json:"notes"`
}

func getGeminiSuggestion(prompt string) (StructuredSuggestion, string, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return StructuredSuggestion{}, "", fmt.Errorf("GEMINI_API_KEY not set")
	}

	client := resty.New()
	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=" + apiKey

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
	}

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(requestBody).
		Post(url)

	if err != nil {
		return StructuredSuggestion{}, "", err
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

	err = json.Unmarshal(resp.Body(), &result)
	if err != nil {
		return StructuredSuggestion{}, "", err
	}

	if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
		rawText := result.Candidates[0].Content.Parts[0].Text

	
		rawText = strings.TrimSpace(rawText)
		if strings.HasPrefix(rawText, "```json") {
			rawText = strings.TrimPrefix(rawText, "```json")
		} else if strings.HasPrefix(rawText, "```") {
			rawText = strings.TrimPrefix(rawText, "```")
		}
		if strings.HasSuffix(rawText, "```") {
			rawText = strings.TrimSuffix(rawText, "```")
		}

		// Unmarshal into the structured format
		var suggestion StructuredSuggestion
		err = json.Unmarshal([]byte(rawText), &suggestion)
		if err != nil {
			return StructuredSuggestion{}, rawText, fmt.Errorf("failed to parse AI response as structured JSON: %v\nRaw response:\n%s", err, rawText)
		}
		return suggestion, rawText, nil
	}

	return StructuredSuggestion{}, "", fmt.Errorf("no suggestion available")
}
