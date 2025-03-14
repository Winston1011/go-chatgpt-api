package api

//goland:noinspection GoSnakeCaseUsage
import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/gin-gonic/gin"
	_ "github.com/linweiyuan/go-chatgpt-api/env"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	docker_client "github.com/docker/docker/client"
)

//goland:noinspection SpellCheckingInspection
const (
	ChatGPTApiPrefix    = "/chatgpt"
	ChatGPTApiUrlPrefix = "https://chat.openai.com"

	PlatformApiPrefix    = "/platform"
	PlatformApiUrlPrefix = "https://api.openai.com"

	defaultErrorMessageKey             = "errorMessage"
	AuthorizationHeader                = "Authorization"
	ContentType                        = "application/x-www-form-urlencoded"
	UserAgent                          = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36"
	Auth0Url                           = "https://auth0.openai.com"
	LoginUsernameUrl                   = Auth0Url + "/u/login/identifier?state="
	LoginPasswordUrl                   = Auth0Url + "/u/login/password?state="
	ParseUserInfoErrorMessage          = "Failed to parse user login info."
	ParseUsageInfoErrorMessage         = "Failed to parse usage param ."
	GetAuthorizedUrlErrorMessage       = "Failed to get authorized url."
	GetStateErrorMessage               = "Failed to get state."
	EmailInvalidErrorMessage           = "Email is not valid."
	EmailOrPasswordInvalidErrorMessage = "Email or password is not correct."
	GetAccessTokenErrorMessage         = "Failed to get access token."
	defaultTimeoutSeconds              = 300 // 5 minutes

	ReadyHint = "Service go-chatgpt-api is ready."
)

var Client tls_client.HttpClient

type LoginInfo struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type UsageParam struct {
	StartDate string `json:"start_date" form:"start_date"`
	EndDate string `json:"end_date" form:"end_date"`
}

type AuthLogin interface {
	GetAuthorizedUrl(csrfToken string) (string, int, error)
	GetState(authorizedUrl string) (string, int, error)
	CheckUsername(state string, username string) (int, error)
	CheckPassword(state string, username string, password string) (string, int, error)
	GetAccessToken(code string) (string, int, error)
}

//goland:noinspection GoUnhandledErrorResult
func init() {
	Client, _ = tls_client.NewHttpClient(tls_client.NewNoopLogger(), []tls_client.HttpClientOption{
		tls_client.WithCookieJar(tls_client.NewCookieJar()),
		tls_client.WithTimeoutSeconds(defaultTimeoutSeconds),
		tls_client.WithClientProfile(tls_client.Okhttp4Android13),
	}...)
}

//goland:noinspection GoUnhandledErrorResult,SpellCheckingInspection
func NewHttpClient() tls_client.HttpClient {
	client, _ := tls_client.NewHttpClient(tls_client.NewNoopLogger(), []tls_client.HttpClientOption{
		tls_client.WithCookieJar(tls_client.NewCookieJar()),
		tls_client.WithClientProfile(tls_client.Okhttp4Android13),
	}...)

	proxyUrl := os.Getenv("GO_CHATGPT_API_PROXY")
	if proxyUrl != "" {
		client.SetProxy(proxyUrl)
	}

	return client
}

//goland:noinspection GoUnhandledErrorResult
func Proxy(c *gin.Context) {
	url := c.Request.URL.Path
	if strings.Contains(url, ChatGPTApiPrefix) {
		url = strings.ReplaceAll(url, ChatGPTApiPrefix, ChatGPTApiUrlPrefix)
	} else {
		url = strings.ReplaceAll(url, PlatformApiPrefix, PlatformApiUrlPrefix)
	}

	method := c.Request.Method
	queryParams := c.Request.URL.Query().Encode()
	if queryParams != "" {
		url += "?" + queryParams
	}

	// if not set, will return 404
	c.Status(http.StatusOK)

	var req *http.Request
	if method == http.MethodGet {
		req, _ = http.NewRequest(http.MethodGet, url, nil)
	} else {
		body, _ := io.ReadAll(c.Request.Body)
		req, _ = http.NewRequest(method, url, bytes.NewReader(body))
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Authorization", GetAccessToken(c.GetHeader(AuthorizationHeader)))
	resp, err := Client.Do(req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, ReturnMessage(err.Error()))
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

func ReturnMessage(msg string) gin.H {
	return gin.H{
		defaultErrorMessageKey: msg,
	}
}

func GetAccessToken(accessToken string) string {
	if !strings.HasPrefix(accessToken, "Bearer") {
		return "Bearer " + accessToken
	}
	return accessToken
}

//goland:noinspection SpellCheckingInspection
func HealthCheck(c *gin.Context) {
	cli, err := docker_client.NewClientWithOpts(docker_client.FromEnv)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, ReturnMessage("Failed to connect to docker daemon."))
		return
	}

	containers, err := cli.ContainerList(c, types.ContainerListOptions{})
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, ReturnMessage("Failed to list containers."))
		return
	}

	containerID := ""
	for _, container := range containers {
		if container.Image == "linweiyuan/go-chatgpt-api" {
			containerID = container.ID
			break
		}
	}

	containerInfo, err := cli.ContainerInspect(c, containerID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, ReturnMessage("Failed to get container info."))
		return
	}

	responseMap := make(map[string]interface{})
	responseMap["ImageID"] = containerInfo.Image

	c.JSON(http.StatusOK, responseMap)
}
