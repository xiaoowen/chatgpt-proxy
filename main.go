package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/pkoukk/tiktoken-go"
	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
)

func init() {
	initFns := []func(){
		initLog, initRedis,
	}
	for _, fn := range initFns {
		fn()
	}
	initFnsReturnErr := []func() error{
		initUsers, initGPTSetting,
	}
	for _, fn := range initFnsReturnErr {
		if err := fn(); err != nil {
			logrus.Fatalf("init failed. %s", err.Error())
		}
	}
}

func initLog() {
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetReportCaller(true)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
			return "", fmt.Sprintf(" %s:%d", path.Base(frame.File), frame.Line)
		},
	})
}

func main() {
	// hello world
	http.HandleFunc("/hello", func(writer http.ResponseWriter, request *http.Request) {
		if _, err := writer.Write([]byte("hello world!")); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
	})
	// 获取支持模型
	http.HandleFunc("/models", func(writer http.ResponseWriter, request *http.Request) {
		models := []string{
			openai.GPT432K0314, openai.GPT432K, openai.GPT40314, openai.GPT4,
			openai.GPT3Dot5Turbo0301, openai.GPT3Dot5Turbo,
			openai.GPT3TextDavinci003, openai.GPT3TextDavinci002,
		}
		render(writer, models)
	})

	// 新用户
	http.HandleFunc("/newuser", func(writer http.ResponseWriter, request *http.Request) {
		if !isPostRequest(writer, request) {
			return
		}
		user, err := getRequestUser(writer, request)
		if err != nil {
			return
		}
		if isAdminUser(writer, user) == false {
			return
		}

		newUser := &User{}
		if err := json.NewDecoder(request.Body).Decode(newUser); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		if newUser.ExpireTimestamp == 0 {
			newUser.ExpireTimestamp = time.Now().Add(time.Hour * 24 * 7).Unix() // 默认7天可用期
		}

		newUser.Token = generateUserToken(32)
		if err := newUser.New(); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		render(writer, newUser)
	})

	// 编辑用户
	http.HandleFunc("/edituser", func(writer http.ResponseWriter, request *http.Request) {
		if !isPostRequest(writer, request) {
			return
		}
		user, err := getRequestUser(writer, request)
		if err != nil {
			return
		}
		if isAdminUser(writer, user) == false {
			return
		}

		editUser := &User{}
		if err := json.NewDecoder(request.Body).Decode(editUser); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		if editUser.Token != user.Token {
			http.Error(writer, "invalid token", http.StatusForbidden)
			return
		}
		// compare field
		if editUser.Admin == !user.Admin {
			user.Admin = editUser.Admin
		}
		if editUser.ExpireTimestamp != user.ExpireTimestamp && editUser.ExpireTimestamp > 0 {
			user.ExpireTimestamp = editUser.ExpireTimestamp
		}
		if editUser.Nickname != user.Nickname {
			user.Nickname = editUser.Nickname
		}
		if editUser.Avatar != editUser.Avatar {
			user.Avatar = editUser.Avatar
		}
		if editUser.Setting != nil {
			user.Setting = editUser.Setting
		}
		// save
		if err := user.Update(); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		render(writer, user)
	})
	// 全部用户
	http.HandleFunc("/users", func(writer http.ResponseWriter, request *http.Request) {
		if user, err := getRequestUser(writer, request); err != nil {
			return
		} else {
			if isAdminUser(writer, user) == false {
				return
			}
			render(writer, getAllUser())
		}
	})
	// 用户信息
	http.HandleFunc("/profile", func(writer http.ResponseWriter, request *http.Request) {
		if user, err := getRequestUser(writer, request); err != nil {
			return
		} else {
			render(writer, user)
		}
	})
	// 全局设置
	http.HandleFunc("/setting", func(writer http.ResponseWriter, request *http.Request) {
		if user, err := getRequestUser(writer, request); err != nil {
			return
		} else {
			if isAdminUser(writer, user) == false {
				return
			}
			render(writer, gptSetting)
		}
	})
	// 全局设置更新
	http.HandleFunc("/settingedit", func(writer http.ResponseWriter, request *http.Request) {
		if !isPostRequest(writer, request) {
			return
		}
		if user, err := getRequestUser(writer, request); err != nil {
			return
		} else {
			if isAdminUser(writer, user) == false {
				return
			}
			setting := &GPTSetting{}
			if err := json.NewDecoder(request.Body).Decode(setting); err != nil {
				http.Error(writer, err.Error(), http.StatusBadRequest)
				return
			}
			setting.Update()
		}
	})
	// 聊天
	http.HandleFunc("/chat", chatHandler)
	// 聊天-流模式
	http.HandleFunc("/chatstream", chatStreamHandler)

	logrus.Info("http server listen :80")
	if err := http.ListenAndServe(":80", nil); err != nil {
		log.Fatalln(err)
	}
}

func isAdminUser(writer http.ResponseWriter, user *User) bool {
	if user == nil || user.Admin == false {
		http.Error(writer, "no permission", http.StatusForbidden)
		return false
	}
	return user.Admin
}

func getRequestUser(writer http.ResponseWriter, request *http.Request) (user *User, err error) {
	user = getUser(request.Header.Get("Access-Token"))
	if user == nil {
		http.Error(writer, "invalid user", http.StatusForbidden)
		return nil, errors.New("invalid user")
	}
	if user.ExpireTimestamp > 0 && user.ExpireTimestamp < time.Now().Unix() {
		http.Error(writer, "license expires", http.StatusForbidden)
		return nil, errors.New("license expires")
	}
	return user, nil
}

func isPostRequest(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Must POST", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

func chatStreamHandler(w http.ResponseWriter, r *http.Request) {
	instance, err := getChatInstance(w, r, true)
	if err != nil {
		logrus.Error(err)
		return
	}
	// stream true
	instance.payload.Stream = true

	stream, err := instance.client.CreateChatCompletionStream(context.Background(), instance.payload)
	if err != nil {
		http.Error(w, fmt.Sprintf("chat_completion_stream error: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	// stream return
	// w.Header().Set("content-type", "application/json;charset=utf-8")
	// cors
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	// sse
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	w.WriteHeader(http.StatusOK)
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	usage := &openai.Usage{
		PromptTokens: getTokensFromMessages(instance.payload.Messages, instance.payload.Model),
	}

	responseText := ""
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			usage.CompletionTokens = getTokensByModel(responseText, instance.payload.Model)
			usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
			if err := instance.user.UpdateCostUsage(usage); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			ret := &chatResponse{Done: true, Content: "", TotalTokens: usage.TotalTokens, UserCost: instance.user.Cost}
			flushResponse(f, w, ret)
			return
		}
		if err != nil {
			logrus.Errorf("stream recv error: %s", err.Error())
			return
		}

		ret := &chatResponse{
			Content: resp.Choices[0].Delta.Content,
		}
		responseText += ret.Content
		flushResponse(f, w, ret)
	}
}

func flushResponse(f http.Flusher, w http.ResponseWriter, ret *chatResponse) {
	if b, err := json.Marshal(ret); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
		b = append(b, '\n', '\n') // flush的内容保证以\n结尾，追加两个换行
		if _, err := w.Write(b); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		f.Flush()
	}
}

func chatHandler(w http.ResponseWriter, r *http.Request) {
	instance, err := getChatInstance(w, r, false)
	if err != nil {
		logrus.Error(err)
		return
	}

	// stream false
	instance.payload.Stream = false

	resp, err := instance.client.CreateChatCompletion(context.Background(), instance.payload)
	if err != nil {
		http.Error(w, fmt.Sprintf("chat_completion error: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	// 计算费用
	if err := instance.user.UpdateCostUsage(&resp.Usage); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ret := &chatResponse{
		Done:        true,
		Created:     resp.Created,
		Content:     resp.Choices[0].Message.Content,
		TotalTokens: resp.Usage.TotalTokens,
		UserCost:    instance.user.Cost,
	}
	render(w, ret)
}

type chatInstance struct {
	user    *User
	client  *openai.Client
	payload openai.ChatCompletionRequest
}

func getChatInstance(w http.ResponseWriter, r *http.Request, isStream bool) (instance *chatInstance, err error) {
	instance = &chatInstance{}

	instance.payload = openai.ChatCompletionRequest{}
	if isStream {
		if err := json.NewDecoder(strings.NewReader(r.FormValue("data"))).Decode(&instance.payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return nil, err
		}
	} else {
		if !isPostRequest(w, r) {
			return nil, errors.New("method not allowed")
		}
		if err := json.NewDecoder(r.Body).Decode(&instance.payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return nil, err
		}
	}

	messageLen := len(instance.payload.Messages)
	if messageLen == 0 {
		http.Error(w, "content empty", http.StatusBadRequest)
		return nil, errors.New("content empty")
	}

	instance.user = getUser(r.Header.Get("Access-Token"))
	if instance.user == nil {
		http.Error(w, "invalid user", http.StatusForbidden)
		return nil, errors.New("invalid user")
	}
	logrus.Infof("received content: %#v", instance.payload.Messages)

	gptSetting := GetSettingWithUser(instance.user.Setting)
	// 历史消息数量
	if gptSetting.MaxHistoryMessages == 0 {
		gptSetting.MaxHistoryMessages = 1
	}
	instance.payload.Messages = instance.payload.Messages[messageLen-gptSetting.MaxHistoryMessages:]
	instance.payload.Model = gptSetting.Model // 强制设置
	// client
	instance.client = openai.NewClient(gptSetting.APIKey)

	return instance, nil
}

type chatResponse struct {
	Done        bool   `json:"done,omitempty"`
	Created     int64  `json:"created,omitempty"`
	Content     string `json:"content"`
	TotalTokens int    `json:"total_tokens,omitempty"`
	UserCost    *Cost  `json:"user_cost,omitempty"`
}

func render(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if b, err := json.Marshal(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
		w.Header().Set("Content-Type", "application/json;charset-urf-8")
		if _, err := w.Write(b); err != nil {
			logrus.Errorf("write response err: %s", err)
			return
		}
	}
}

func getTokensByModel(text, model string) (tokens int) {
	tkm, err := tiktoken.EncodingForModel(model)
	if err != nil {
		logrus.Error(err)
		return
	}
	token := tkm.Encode(text, nil, nil)
	return len(token)
}

func getTokensFromMessages(messages []openai.ChatCompletionMessage, model string) (tokens int) {
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
