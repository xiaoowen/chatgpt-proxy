package data

import (
	"github.com/pkoukk/tiktoken-go"
	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
)

func GetTokensByModel(text, model string) (tokens int) {
	tkm, err := tiktoken.EncodingForModel(model)
	if err != nil {
		logrus.Error(err)
		return
	}
	token := tkm.Encode(text, nil, nil)
	return len(token)
}

func GetTokensFromMessages(messages []openai.ChatCompletionMessage, model string) (tokens int) {
	tkm, err := tiktoken.EncodingForModel(model)
	if err != nil {
		logrus.Error(err)
		return
	}

	var tokensPerMessage int
	var tokensPerName int
	if model == openai.GPT3Dot5Turbo0301 || model == openai.GPT3Dot5Turbo {
		tokensPerMessage = 4
		tokensPerName = -1
	} else if model == openai.GPT40314 || model == openai.GPT4 {
		tokensPerMessage = 3
		tokensPerName = 1
	} else {
		logrus.Warn("warning: model not found. using cl100k_base encoding")
		tokensPerMessage = 3
		tokensPerName = 1
	}

	for _, message := range messages {
		tokens += tokensPerMessage +
			len(tkm.Encode(message.Content, nil, nil)) +
			len(tkm.Encode(message.Role, nil, nil)) +
			len(tkm.Encode(message.Name, nil, nil))
		if message.Name != "" {
			tokens += tokensPerName
		}
	}
	tokens += 3
	return
}
