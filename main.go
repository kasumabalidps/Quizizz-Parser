package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
)

type Config struct {
	QuizID      string `json:"quiz_id"`
	WebhookURL  string `json:"webhook_url"`
	WebhookName string `json:"webhook_name"`
	ProfileURL  string `json:"profile_url"`
}

type Quiz struct {
	Data struct {
		Quiz struct {
			Info struct {
				Questions []struct {
					ID        string `json:"id"`
					Structure struct {
						Query struct {
							Text string `json:"text"`
						} `json:"query"`
						Answer  int `json:"answer"`
						Options []struct {
							ID   string `json:"id"`
							Text string `json:"text"`
						} `json:"options"`
					} `json:"structure"`
				} `json:"questions"`
			} `json:"info"`
		} `json:"quiz"`
	} `json:"data"`
}

func removeHTMLTags(text string) string {
	text = strings.ReplaceAll(text, "<br>", "\n")
	re := regexp.MustCompile(`<[^>]*>`)
	text = re.ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "&nbsp;", "")
	return text
}

func getAnswer(id string) string {
	resp, err := http.Get("https://quizizz.com/quiz/" + id)
	if err != nil {
		fmt.Println("Error fetching data:", err)
		return "Error fetching data"
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return "Error reading response body"
	}

	var data Quiz
	err = json.Unmarshal(body, &data)
	if err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
		return "Error unmarshaling JSON"
	}

	type QuestionAnswer struct {
		ID          string
		Question    string
		Answer      string
		AnswerIndex int
	}

	var qaList []QuestionAnswer

	for _, question := range data.Data.Quiz.Info.Questions {
		questionText := removeHTMLTags(question.Structure.Query.Text)
		answer := removeHTMLTags(question.Structure.Options[question.Structure.Answer].Text)
		qa := QuestionAnswer{
			ID:          question.ID,
			Question:    questionText,
			Answer:      answer,
			AnswerIndex: question.Structure.Answer,
		}
		qaList = append(qaList, qa)
	}

	sort.Slice(qaList, func(i, j int) bool {
		return qaList[i].ID < qaList[j].ID
	})

	var result strings.Builder
	for i, qa := range qaList {
		result.WriteString(fmt.Sprintf("%d. **%s**\n**Answer:** `%s`\n\n", i+1, qa.Question, qa.Answer))
	}
	return result.String()
}

func sendToDiscord(webhookURL, webhookName, profileURL, message string) {
	type Embed struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Color       int    `json:"color"`
	}

	type Payload struct {
		Username  string  `json:"username"`
		AvatarURL string  `json:"avatar_url"`
		Embeds    []Embed `json:"embeds"`
	}

	embed := Embed{
		Title:       "Quiz Answers",
		Description: message,
		Color:       3447003,
	}

	payload := Payload{
		Username:  webhookName,
		AvatarURL: profileURL,
		Embeds:    []Embed{embed},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Error marshaling payload:", err)
		return
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		fmt.Println("Error sending message to Discord:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error sending message to Discord: %s\nResponse body: %s\n", resp.Status, string(body))
		return
	}

	fmt.Println("Message sent to Discord successfully!")
}

func loadConfig(filename string) (*Config, error) {
	configFile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer configFile.Close()

	bytes, err := ioutil.ReadAll(configFile)
	if err != nil {
		return nil, err
	}

	var config Config
	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func main() {
	config, err := loadConfig("config.json")
	if err != nil {
		fmt.Println("Error loading config:", err)
		return
	}

	result := getAnswer(config.QuizID)
	sendToDiscord(config.WebhookURL, config.WebhookName, config.ProfileURL, result)
}
