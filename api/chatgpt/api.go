package chatgpt

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	http "github.com/bogdanfinn/fhttp"
	"github.com/gin-gonic/gin"
	"github.com/linweiyuan/go-chatgpt-api/api"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

var (
	arkoseTokenUrl string
)

//goland:noinspection SpellCheckingInspection
func init() {
	arkoseTokenUrl = os.Getenv("GO_CHATGPT_API_ARKOSE_TOKEN_URL")
}

func getArkoseToken() (string, error) {
	paramsURL := "https://ai.fakeopen.com/api/arkose/params?format=all"
	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.4 Safari/605.1.15",
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", paramsURL, nil)
	if err != nil {
		return "", err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get arkose params: %d %s", resp.StatusCode, string(bodyBytes))
	}

	var data map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return "", err
	}

	if _, ok := data["endpoint"].(string); !ok {
		return "", fmt.Errorf("invalid arkose params: %v", data)
	}

	_, _ = json.Marshal(data["headers"])
	formJSON, _ := json.Marshal(data["form"])

	req, err = http.NewRequest("POST", data["endpoint"].(string), strings.NewReader(string(formJSON)))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err = client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get arkose token: %d %s", resp.StatusCode, string(bodyBytes))
	}

	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return "", err
	}

	token, ok := data["token"].(string)
	if !ok {
		return "", fmt.Errorf("failed to get arkose token: %v", data)
	}

	return token, nil
}


//goland:noinspection GoUnhandledErrorResult
func GetConversations(c *gin.Context) {
	offset, ok := c.GetQuery("offset")
	if !ok {
		offset = "0"
	}
	limit, ok := c.GetQuery("limit")
	if !ok {
		limit = "20"
	}
	handleGet(c, apiPrefix+"/conversations?offset="+offset+"&limit="+limit, getConversationsErrorMessage)
}

//goland:noinspection GoUnhandledErrorResult
func CreateConversation(c *gin.Context) {
	var request CreateConversationRequest
	if err := c.BindJSON(&request); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, api.ReturnMessage(parseJsonErrorMessage))
		return
	}

	if request.ConversationID == nil || *request.ConversationID == "" {
		request.ConversationID = nil
	}

	if request.Messages[0].Author.Role == "" {
		request.Messages[0].Author.Role = defaultRole
	}

	if strings.HasPrefix(request.Model, gpt4Model) {
		if arkoseTokenUrl != "" {
			req, _ := http.NewRequest(http.MethodGet, arkoseTokenUrl, nil)
			resp, err := api.Client.Do(req)
			if err != nil || resp.StatusCode != http.StatusOK {
				c.AbortWithStatusJSON(http.StatusInternalServerError, api.ReturnMessage("Failed to get arkose token."))
				return
			}
			responseMap := make(map[string]string)
			json.NewDecoder(resp.Body).Decode(&responseMap)
			request.ArkoseToken = responseMap["token"]
		} else {
			request.ArkoseToken, _ = getArkoseToken()
		}
	}

	resp, done := sendConversationRequest(c, request)
	if done {
		return
	}

	handleConversationResponse(c, resp, request)
}

//goland:noinspection GoUnhandledErrorResult
func GenerateTitle(c *gin.Context) {
	var request GenerateTitleRequest
	if err := c.BindJSON(&request); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, api.ReturnMessage(parseJsonErrorMessage))
		return
	}

	jsonBytes, _ := json.Marshal(request)
	handlePost(c, apiPrefix+"/conversation/gen_title/"+c.Param("id"), string(jsonBytes), generateTitleErrorMessage)
}

//goland:noinspection GoUnhandledErrorResult
func GetConversation(c *gin.Context) {
	handleGet(c, apiPrefix+"/conversation/"+c.Param("id"), getContentErrorMessage)
}

//goland:noinspection GoUnhandledErrorResult
func UpdateConversation(c *gin.Context) {
	var request PatchConversationRequest
	if err := c.BindJSON(&request); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, api.ReturnMessage(parseJsonErrorMessage))
		return
	}

	// bool default to false, then will hide (delete) the conversation
	if request.Title != nil {
		request.IsVisible = true
	}
	jsonBytes, _ := json.Marshal(request)
	handlePatch(c, apiPrefix+"/conversation/"+c.Param("id"), string(jsonBytes), updateConversationErrorMessage)
}

//goland:noinspection GoUnhandledErrorResult
func FeedbackMessage(c *gin.Context) {
	var request FeedbackMessageRequest
	if err := c.BindJSON(&request); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, api.ReturnMessage(parseJsonErrorMessage))
		return
	}

	jsonBytes, _ := json.Marshal(request)
	handlePost(c, apiPrefix+"/conversation/message_feedback", string(jsonBytes), feedbackMessageErrorMessage)
}

//goland:noinspection GoUnhandledErrorResult
func ClearConversations(c *gin.Context) {
	jsonBytes, _ := json.Marshal(PatchConversationRequest{
		IsVisible: false,
	})
	handlePatch(c, apiPrefix+"/conversations", string(jsonBytes), clearConversationsErrorMessage)
}

//goland:noinspection GoUnhandledErrorResult
func GetModels(c *gin.Context) {
	handleGet(c, apiPrefix+"/models", getModelsErrorMessage)
}

func GetAccountCheck(c *gin.Context) {
	handleGet(c, apiPrefix+"/accounts/check", getAccountCheckErrorMessage)
}

//goland:noinspection GoUnhandledErrorResult
func handleGet(c *gin.Context, url string, errorMessage string) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("User-Agent", api.UserAgent)
	req.Header.Set("Authorization", api.GetAccessToken(c.GetHeader(api.AuthorizationHeader)))
	req.Header.Set("Accept", "text/event-stream")
	resp, err := api.Client.Do(req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, api.ReturnMessage(err.Error()))
		return
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		responseMap := make(map[string]interface{})
		json.NewDecoder(resp.Body).Decode(&responseMap)
		c.AbortWithStatusJSON(resp.StatusCode, responseMap)
		return
	}

	io.Copy(c.Writer, resp.Body)
}

//goland:noinspection GoUnhandledErrorResult
func handlePost(c *gin.Context, url string, requestBody string, errorMessage string) {
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(requestBody))
	handlePostOrPatch(c, req, errorMessage)
}

//goland:noinspection GoUnhandledErrorResult
func handlePatch(c *gin.Context, url string, requestBody string, errorMessage string) {
	req, _ := http.NewRequest(http.MethodPatch, url, strings.NewReader(requestBody))
	handlePostOrPatch(c, req, errorMessage)
}

//goland:noinspection GoUnhandledErrorResult
func handlePostOrPatch(c *gin.Context, req *http.Request, errorMessage string) {
	req.Header.Set("User-Agent", api.UserAgent)
	req.Header.Set("Authorization", api.GetAccessToken(c.GetHeader(api.AuthorizationHeader)))
	resp, err := api.Client.Do(req)
	log.Println("patch req", req)
	log.Println("patch resp", resp)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, api.ReturnMessage(err.Error()))
		return
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.AbortWithStatusJSON(resp.StatusCode, api.ReturnMessage(errorMessage))
		return
	}

	io.Copy(c.Writer, resp.Body)
}

//goland:noinspection GoUnhandledErrorResult
func sendConversationRequest(c *gin.Context, request CreateConversationRequest) (*http.Response, bool) {
	jsonBytes, _ := json.Marshal(request)
	req, _ := http.NewRequest(http.MethodPost, api.ChatGPTApiUrlPrefix+"/backend-api/conversation", bytes.NewBuffer(jsonBytes))
	req.Header.Set("User-Agent", api.UserAgent)
	req.Header.Set("Authorization", api.GetAccessToken(c.GetHeader(api.AuthorizationHeader)))
	req.Header.Set("Accept", "text/event-stream")
	resp, err := api.Client.Do(req)
	log.Println("conversation req: ", req)
	log.Println("conversation resp: ", resp)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, api.ReturnMessage(err.Error()))
		return nil, true
	}

	if resp.StatusCode != http.StatusOK {
		responseMap := make(map[string]interface{})
		json.NewDecoder(resp.Body).Decode(&responseMap)
		c.AbortWithStatusJSON(resp.StatusCode, responseMap)
		return nil, true
	}

	return resp, false
}

//goland:noinspection GoUnhandledErrorResult
func handleConversationResponse(c *gin.Context, resp *http.Response, request CreateConversationRequest) {
	c.Writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")

	isMaxTokens := false
	continueParentMessageID := ""
	continueConversationID := ""

	defer resp.Body.Close()
	reader := bufio.NewReader(resp.Body)
	for {
		if c.Request.Context().Err() != nil {
			break
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "event") ||
			strings.HasPrefix(line, "data: 20") ||
			line == "" {
			continue
		}

		responseJson := line[6:]
		if strings.HasPrefix(responseJson, "[DONE]") && isMaxTokens && request.AutoContinue {
			continue
		}

		// no need to unmarshal every time, but if response content has this "max_tokens", need to further check
		if strings.TrimSpace(responseJson) != "" && strings.Contains(responseJson, responseTypeMaxTokens) {
			var createConversationResponse CreateConversationResponse
			json.Unmarshal([]byte(responseJson), &createConversationResponse)
			message := createConversationResponse.Message
			if message.Metadata.FinishDetails.Type == responseTypeMaxTokens && createConversationResponse.Message.Status == responseStatusFinishedSuccessfully {
				isMaxTokens = true
				continueParentMessageID = message.ID
				continueConversationID = createConversationResponse.ConversationID
			}
		}

		c.Writer.Write([]byte(line + "\n\n"))
		c.Writer.Flush()
	}

	if isMaxTokens && request.AutoContinue {
		continueConversationRequest := CreateConversationRequest{
			ArkoseToken:                request.ArkoseToken,
			HistoryAndTrainingDisabled: request.HistoryAndTrainingDisabled,
			Model:                      request.Model,
			TimezoneOffsetMin:          request.TimezoneOffsetMin,

			Action:          actionContinue,
			ParentMessageID: continueParentMessageID,
			ConversationID:  &continueConversationID,
		}
		resp, done := sendConversationRequest(c, continueConversationRequest)
		if done {
			return
		}

		handleConversationResponse(c, resp, continueConversationRequest)
	}
}