package components

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	jsoniter "github.com/json-iterator/go"
	"github.com/json-iterator/go/extra"
)

// gin 默认的json binding 无法处理形如字符串数字"2"向整形数字2的转换，这里引入jsoniter来解决
func init() {
	extra.RegisterFuzzyDecoders()
}

var (
	JSONITER = jsoniterBinding{}
)

func Parse(ctx *gin.Context, param interface{}) error {
	b := binding.Default(ctx.Request.Method, ctx.ContentType())
	if b == binding.JSON {
		return ctx.ShouldBindWith(param, JSONITER)
	}
	return ctx.ShouldBindWith(param, b)
}

type jsoniterBinding struct{}

func (jsoniterBinding) Name() string {
	return "jsoniter"
}

func (jsoniterBinding) Bind(req *http.Request, obj interface{}) error {
	if req == nil || req.Body == nil {
		return fmt.Errorf("invalid request")
	}
	decoder := jsoniter.NewDecoder(req.Body)
	if binding.EnableDecoderUseNumber {
		decoder.UseNumber()
	}
	if binding.EnableDecoderDisallowUnknownFields {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(obj); err != nil {
		return err
	}
	return binding.Validator.ValidateStruct(obj)
}
