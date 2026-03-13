package app

import (
	"context"
	"errors"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"proyecto-cursos/services/courses-api/internal/service"
)

type OpenAIRecommendationClient struct {
	client *openai.Client
	model  string
}

func NewOpenAIRecommendationClient(apiKey, model string) *OpenAIRecommendationClient {
	if strings.TrimSpace(apiKey) == "" {
		return nil
	}

	trimmedModel := strings.TrimSpace(model)
	if trimmedModel == "" {
		trimmedModel = "gpt-4.1"
	}

	return &OpenAIRecommendationClient{
		client: openai.NewClient(strings.TrimSpace(apiKey)),
		model:  trimmedModel,
	}
}

func (c *OpenAIRecommendationClient) Recommend(ctx context.Context, systemPrompt string, history []service.RecommendationMessage, question string) (string, error) {
	if c == nil || c.client == nil {
		return "", service.ErrRecommendationsDisabled
	}

	requestCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	messages := make([]openai.ChatCompletionMessage, 0, len(history)+2)
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: systemPrompt,
	})
	for _, message := range history {
		role := openai.ChatMessageRoleUser
		if message.Role == "assistant" {
			role = openai.ChatMessageRoleAssistant
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    role,
			Content: message.Content,
		})
	}
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: strings.TrimSpace(question),
	})

	response, err := c.client.CreateChatCompletion(requestCtx, openai.ChatCompletionRequest{
		Model:       c.model,
		Temperature: 0.6,
		Messages:    messages,
	})
	if err != nil {
		return "", err
	}
	if len(response.Choices) == 0 {
		return "", errors.New("openai returned no choices")
	}

	return strings.TrimSpace(response.Choices[0].Message.Content), nil
}
