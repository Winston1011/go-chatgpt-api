package chatgpt

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/linweiyuan/go-chatgpt-api/api"
	"io"
	"log"
	"math/rand"
	"net/url"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
)

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

	if request.Model == gpt4Model || request.Model == gpt4BrowsingModel || request.Model == gpt4PluginsModel {
		formParams := fmt.Sprintf(
			"public_key=%s&site=%s&userbrowser=%s&capi_version=%s&capi_mode=%s&style_theme=%s&rnd=%s",
			gpt4ArkoseTokenPublicKey,
			url.QueryEscape(gpt4ArkoseTokenSite),
			url.QueryEscape(gpt4ArkoseTokenUserBrowser),
			gpt4ArkoseTokenCapiVersion,
			gpt4ArkoseTokenCapiMode,
			gpt4ArkoseTokenStyleTheme,
			generateArkoseTokenRnd(),
		)
		req, _ := http.NewRequest(http.MethodPost, gpt4ArkoseTokenUrl, strings.NewReader(formParams))
		req.Header.Set("Content-Type", api.ContentType)
		resp, err := api.Client.Do(req)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, api.ReturnMessage(err.Error()))
			return
		}

		responseMap := make(map[string]string)
		json.NewDecoder(resp.Body).Decode(&responseMap)
		request.ArkoseToken = responseMap["token"]
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

func generateArkoseTokenRnd() string {
	rand.NewSource(time.Now().UnixNano())
	return fmt.Sprintf("%.17f", rand.Float64())
}
