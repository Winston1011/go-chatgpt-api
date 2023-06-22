package platform

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log"

	"fmt"
	"github.com/linweiyuan/go-chatgpt-api/components"

	"io"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/linweiyuan/go-chatgpt-api/api"

	http "github.com/bogdanfinn/fhttp"
)

func ListModels(c *gin.Context) {
	handleGet(c, apiListModels)
}

func RetrieveModel(c *gin.Context) {
	model := c.Param("model")
	handleGet(c, fmt.Sprintf(apiRetrieveModel, model))
}

//goland:noinspection GoUnhandledErrorResult
func CreateChatCompletions(c *gin.Context) {
	body, _ := io.ReadAll(c.Request.Body)
	var request struct {
		Stream bool `json:"stream"`
	}
	json.Unmarshal(body, &request)

	url := c.Request.URL.Path
	if strings.Contains(url, "/chat") {
		url = apiCreateChatCompletions
	} else {
		url = apiCreateCompletions
	}
	resp, err := handlePost(c, url, body, request.Stream)
	if err != nil {
		return
	}

	defer resp.Body.Close()
	if request.Stream {
		handleCompletionsResponse(c, resp)
	} else {
		io.Copy(c.Writer, resp.Body)
	}
}

func CreateCompletions(c *gin.Context) {
	CreateChatCompletions(c)
}

//goland:noinspection GoUnhandledErrorResult
func handleCompletionsResponse(c *gin.Context, resp *http.Response) {
	c.Writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")

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

		c.Writer.Write([]byte(line + "\n\n"))
		c.Writer.Flush()
	}

	defer resp.Body.Close()
	io.Copy(c.Writer, resp.Body)
}

//goland:noinspection GoUnhandledErrorResult
func CreateEmbeddings(c *gin.Context) {
	var request CreateEmbeddingsRequest
	c.ShouldBindJSON(&request)
	data, _ := json.Marshal(request)
	resp, err := handlePost(c, apiCreateEmbeddings, data, false)
	if err != nil {
		return
	}

	defer resp.Body.Close()
	io.Copy(c.Writer, resp.Body)
}

func CreateModeration(c *gin.Context) {
	var request CreateModerationRequest
	c.ShouldBindJSON(&request)
	data, _ := json.Marshal(request)
	resp, err := handlePost(c, apiCreateModeration, data, false)
	if err != nil {
		return
	}

	defer resp.Body.Close()
	io.Copy(c.Writer, resp.Body)
}

func ListFiles(c *gin.Context) {
	handleGet(c, apiListFiles)
}

func GetCreditGrants(c *gin.Context) {
	handleGet(c, apiGetCreditGrants)
}

func GetGetUsage(c *gin.Context) {
	var usageParam api.UsageParam
	if err := components.Parse(c, &usageParam); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, api.ReturnMessage(api.ParseUsageInfoErrorMessage))
		return
	}
	handleGet(c, fmt.Sprintf(apiGetUsage, usageParam.StartDate, usageParam.EndDate))
}

func GetSubscription(c *gin.Context) {
	handleGet(c, apiGetSubscription)
}

func GetApiKeys(c *gin.Context) {
	handleGet(c, apiGetApiKeys)
}

//goland:noinspection GoUnhandledErrorResult
func handleGet(c *gin.Context, url string) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", api.GetAccessToken(c.GetHeader(api.AuthorizationHeader)))
	resp, _ := api.Client.Do(req)
	log.Println(req)
	defer resp.Body.Close()
	io.Copy(c.Writer, resp.Body)
}

func handlePost(c *gin.Context, url string, data []byte, stream bool) (*http.Response, error) {
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	log.Println(req)
	req.Header.Set("Authorization", api.GetAccessToken(c.GetHeader(api.AuthorizationHeader)))
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.Client.Do(req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, api.ReturnMessage(err.Error()))
		return nil, err
	}

	return resp, nil
}
