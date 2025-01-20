// Package aws provides the AWS adaptor for the relay service.
package aws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/ctxkey"
	"github.com/songquanpeng/one-api/common/helper"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/common/random"
	"github.com/songquanpeng/one-api/relay/adaptor/aws/utils"
	"github.com/songquanpeng/one-api/relay/adaptor/openai"
	relaymodel "github.com/songquanpeng/one-api/relay/model"
)

// https://docs.aws.amazon.com/bedrock/latest/userguide/model-ids.html
var AwsModelIDMap = map[string]string{
	"amazon.nova-micro": "us.amazon.nova-micro-v1:0",
	"amazon.nova-lite":  "us.amazon.nova-lite-v1:0",
	"amazon.nova-pro":   "us.amazon.nova-pro-v1:0",
}

func awsModelID(requestModel string) (string, error) {
	if awsModelID, ok := AwsModelIDMap[requestModel]; ok {
		return awsModelID, nil
	}

	return "", errors.Errorf("model %s not found", requestModel)
}

// https://docs.anthropic.com/claude/reference/messages-streaming
func StreamResponseNova2OpenAI(novaResponse *StreamResponse) *openai.ChatCompletionsStreamResponse {
	var choice openai.ChatCompletionsStreamResponseChoice
	choice.Delta.Content = novaResponse.Generation
	choice.Delta.Role = "assistant"
	finishReason := novaResponse.StopReason
	if finishReason != "null" {
		choice.FinishReason = &finishReason
	}
	var openaiResponse openai.ChatCompletionsStreamResponse
	openaiResponse.Object = "chat.completion.chunk"
	openaiResponse.Choices = []openai.ChatCompletionsStreamResponseChoice{choice}
	return &openaiResponse
}

func ResponseNova2OpenAI(novaResponse *Response) *openai.TextResponse {
	var responseText string
	if len(novaResponse.Output.Message.Content) > 0 {
		responseText = *novaResponse.Output.Message.Content[0].Text
	}
	// tools := make([]relaymodel.Tool, 0)
	choice := openai.TextResponseChoice{
		Index: 0,
		Message: relaymodel.Message{
			Role:    "assistant",
			Content: responseText,
			Name:    nil,
			// ToolCalls: tools,
		},
		FinishReason: novaResponse.StopReason,
	}
	fullTextResponse := openai.TextResponse{
		Id: fmt.Sprintf("chatcmpl-%s", random.GetUUID()),
		// Model:   novaResponse.,
		Object:  "chat.completion",
		Created: helper.GetTimestamp(),
		Choices: []openai.TextResponseChoice{choice},
	}
	return &fullTextResponse
}

func Handler(c *gin.Context, awsCli *bedrockruntime.Client, modelName string) (*relaymodel.ErrorWithStatusCode, *relaymodel.Usage) {
	// 获取并验证模型ID
	awsModelId, err := awsModelID(c.GetString(ctxkey.RequestModel))
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "awsModelID")), nil
	}

	// 准备AWS请求
	awsReq := &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(awsModelId),
		Accept:      aws.String("application/json"),
		ContentType: aws.String("application/json"),
	}

	// 获取转换后的Claude请求
	novaReq_, ok := c.Get(ctxkey.ConvertedRequest)
	if !ok {
		return utils.WrapErr(errors.New("request not found")), nil
	}
	novaReq := novaReq_.(*Request)

	// 序列化请求
	awsReq.Body, err = json.Marshal(novaReq)
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "marshal request")), nil
	}

	// 调用AWS API
	awsResp, err := awsCli.InvokeModel(c.Request.Context(), awsReq)
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "InvokeModel")), nil
	}

	// 解析Nova响应
	var novaResponse Response
	err = json.Unmarshal(awsResp.Body, &novaResponse)
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "unmarshal response")), nil
	}
	fullTextResponse := ResponseNova2OpenAI(&novaResponse)
	fullTextResponse.Model = modelName
	usage := relaymodel.Usage{
		PromptTokens:     novaResponse.Usage.InputTokens,
		CompletionTokens: novaResponse.Usage.OutputTokens,
		TotalTokens:      novaResponse.Usage.InputTokens + novaResponse.Usage.OutputTokens,
	}
	fullTextResponse.Usage = usage
	jsonResponse, err := json.Marshal(fullTextResponse)
	if err != nil {
		return openai.ErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError), nil
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(http.StatusOK)
	_, err = c.Writer.Write(jsonResponse)

	c.JSON(http.StatusOK, novaResponse)
	return nil, &usage
}
func StreamHandler(c *gin.Context, awsCli *bedrockruntime.Client) (*relaymodel.ErrorWithStatusCode, *relaymodel.Usage) {
	createdTime := helper.GetTimestamp()
	awsModelId, err := awsModelID(c.GetString(ctxkey.RequestModel))
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "awsModelID")), nil
	}

	awsReq := &bedrockruntime.InvokeModelWithResponseStreamInput{
		ModelId:     aws.String(awsModelId),
		Accept:      aws.String("application/json"),
		ContentType: aws.String("application/json"),
	}

	novaReq, ok := c.Get(ctxkey.ConvertedRequest)
	if !ok {
		return utils.WrapErr(errors.New("request not found")), nil
	}

	awsReq.Body, err = json.Marshal(novaReq)
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "marshal request")), nil
	}

	awsResp, err := awsCli.InvokeModelWithResponseStream(c.Request.Context(), awsReq)
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "InvokeModelWithResponseStream")), nil
	}
	stream := awsResp.GetStream()
	defer stream.Close()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	var usage relaymodel.Usage
	c.Stream(func(w io.Writer) bool {
		event, ok := <-stream.Events()
		if !ok {
			c.Render(-1, common.CustomEvent{Data: "data: [DONE]"})
			return false
		}

		switch v := event.(type) {
		case *types.ResponseStreamMemberChunk:
			var novaResp StreamResponse
			err := json.NewDecoder(bytes.NewReader(v.Value.Bytes)).Decode(&novaResp)
			if err != nil {
				logger.SysError("error unmarshalling stream response: " + err.Error())
				return false
			}

			if novaResp.PromptTokenCount > 0 {
				usage.PromptTokens = novaResp.PromptTokenCount
			}
			if novaResp.StopReason == "stop" {
				usage.CompletionTokens = novaResp.GenerationTokenCount
				usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
			}
			response := StreamResponseNova2OpenAI(&novaResp)
			response.Id = fmt.Sprintf("chatcmpl-%s", random.GetUUID())
			response.Model = c.GetString(ctxkey.OriginalModel)
			response.Created = createdTime
			jsonStr, err := json.Marshal(response)
			if err != nil {
				logger.SysError("error marshalling stream response: " + err.Error())
				return true
			}
			c.Render(-1, common.CustomEvent{Data: "data: " + string(jsonStr)})
			return true
		case *types.UnknownUnionMember:
			fmt.Println("unknown tag:", v.Tag)
			return false
		default:
			fmt.Println("union is nil or unknown type")
			return false
		}
	})

	return nil, &usage
}
