package aws

import (
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/songquanpeng/one-api/common/ctxkey"
	"github.com/songquanpeng/one-api/common/image"
	"github.com/songquanpeng/one-api/relay/adaptor"
	"github.com/songquanpeng/one-api/relay/adaptor/aws/utils"
	"github.com/songquanpeng/one-api/relay/meta"
	"github.com/songquanpeng/one-api/relay/model"
)

var _ utils.AwsAdapter = new(Adaptor)

type Adaptor struct {
}

func (a *Adaptor) ConvertRequest(c *gin.Context, relayMode int, request *model.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}

	// 创建Nova请求结构
	novaRequest := Request{
		InferenceConfig: InferenceConfig{
			MaxNewTokens:  request.MaxTokens,
			Temperature:   *request.Temperature,
			TopP:          *request.TopP,
			TopK:          request.TopK,
			StopSequences: request.Stop.([]string),
		},
	}

	// 处理工具调用
	if len(request.Tools) > 0 {
		tools := make([]Tool, 0, len(request.Tools))
		for _, tool := range request.Tools {
			if params, ok := tool.Function.Parameters.(map[string]any); ok {
				properties := make(map[string]PropertyDef)
				// 转换属性定义
				if props, ok := params["properties"].(map[string]any); ok {
					for name, prop := range props {
						if propMap, ok := prop.(map[string]any); ok {
							properties[name] = PropertyDef{
								Type:        propMap["type"].(string),
								Description: propMap["description"].(string),
							}
						}
					}
				}

				required := make([]string, 0)
				if req, ok := params["required"].([]any); ok {
					for _, r := range req {
						required = append(required, r.(string))
					}
				}

				tools = append(tools, Tool{
					ToolSpec: ToolSpec{
						Name:        tool.Function.Name,
						Description: tool.Function.Description,
						InputSchema: JSONSchema{
							JSON: JSONSchemaDefinition{
								Type:       params["type"].(string),
								Properties: properties,
								Required:   required,
							},
						},
					},
				})
			}
		}

		if len(tools) > 0 {
			novaRequest.ToolConfig = &ToolConfig{
				Tools: tools,
				ToolChoice: ToolChoice{
					Auto: map[string]interface{}{}, // Nova使用空map表示auto
				},
			}
		}
	}

	// 处理消息
	for _, message := range request.Messages {
		if message.Role == "system" {
			novaRequest.System = append(novaRequest.System, SystemMessage{
				Text: message.StringContent(),
			})
			continue
		}

		novaMessage := Message{
			Role: message.Role,
		}

		// 处理消息内容
		if message.IsStringContent() {
			messageString := message.StringContent()
			// 处理文本内容
			novaMessage.Content = append(novaMessage.Content, Content{
				Text: &messageString,
			})
		} else {
			// 处理多模态内容
			contents := message.ParseContent()
			for _, content := range contents {
				var novaContent Content
				switch content.Type {
				case model.ContentTypeText:
					text := content.Text
					novaContent.Text = &text
				case model.ContentTypeImageURL:
					// 获取图片数据并转换为base64
					_, base64Data, err := image.GetImageFromUrl(content.ImageURL.Url)
					if err != nil {
						continue
					}
					novaContent.Image = &ImageData{
						Format: "jpeg", // 假设格式为JPEG，实际应该根据实际情况判断
						Source: ImageSource{
							Bytes: base64Data,
						},
					}
				}
				novaMessage.Content = append(novaMessage.Content, novaContent)
			}
		}

		novaRequest.Messages = append(novaRequest.Messages, novaMessage)
	}

	c.Set(ctxkey.RequestModel, request.Model)
	c.Set(ctxkey.ConvertedRequest, novaRequest)
	return novaRequest, nil
}

func (a *Adaptor) DoResponse(c *gin.Context, awsCli *bedrockruntime.Client, meta *meta.Meta) (usage *model.Usage, err *model.ErrorWithStatusCode) {
	if meta.IsStream {
		err, usage = StreamHandler(c, awsCli)
	} else {
		err, usage = Handler(c, awsCli, meta.ActualModelName)
	}
	return
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Request, meta *meta.Meta) error {
	adaptor.SetupCommonRequestHeader(c, req, meta)
	req.Header.Set("x-api-key", meta.APIKey)
	anthropicVersion := c.Request.Header.Get("anthropic-version")
	if anthropicVersion == "" {
		anthropicVersion = "2023-06-01"
	}
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("anthropic-beta", "messages-2023-12-15")

	// https://x.com/alexalbert__/status/1812921642143900036
	// claude-3-5-sonnet can support 8k context
	if strings.HasPrefix(meta.ActualModelName, "claude-3-5-sonnet") {
		req.Header.Set("anthropic-beta", "max-tokens-3-5-sonnet-2024-07-15")
	}

	return nil
}

func (a *Adaptor) ConvertImageRequest(request *model.ImageRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return request, nil
}
