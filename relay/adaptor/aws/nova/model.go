package aws

// Request is the request to AWS Nova
//
// https://docs.aws.amazon.com/nova/latest/userguide/complete-request-schema.html
// Message represents a single message in the conversation
type Message struct {
	Role    string    `json:"role"`
	Content []Content `json:"content"`
}

// Content represents different types of content in a message
type Content struct {
	Text  *string    `json:"text,omitempty"`
	Image *ImageData `json:"image,omitempty"`
	Video *VideoData `json:"video,omitempty"`
}

// ImageData represents image content
type ImageData struct {
	Format string      `json:"format"` // jpeg, png, gif, webp
	Source ImageSource `json:"source"`
}

// VideoData represents video content
type VideoData struct {
	Format string      `json:"format"` // mkv, mov, mp4, webm, three_gp, flv, mpeg, mpg, wmv
	Source VideoSource `json:"source"`
}

// ImageSource represents the source of an image
type ImageSource struct {
	Bytes string `json:"bytes,omitempty"` // Base64编码的字符串
}

// VideoSource represents the source of a video
type VideoSource struct {
	S3Location *S3Location `json:"s3Location,omitempty"`
	Bytes      string      `json:"bytes,omitempty"` // Base64编码的字符串
}

// S3Location represents an S3 location
type S3Location struct {
	URI         string `json:"uri"`
	BucketOwner string `json:"bucketOwner,omitempty"`
}

// SystemMessage represents a system message
type SystemMessage struct {
	Text string `json:"text"`
}

// InferenceConfig represents the configuration for inference
type InferenceConfig struct {
	MaxNewTokens  int      `json:"max_new_tokens,omitempty"`
	Temperature   float64  `json:"temperature,omitempty"`
	TopP          float64  `json:"top_p,omitempty"`
	TopK          int      `json:"top_k,omitempty"`
	StopSequences []string `json:"stopSequences,omitempty"`
}

// ToolSpec represents the specification of a tool
type ToolSpec struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	InputSchema JSONSchema `json:"inputSchema"`
}

// JSONSchema represents a JSON schema
type JSONSchema struct {
	JSON JSONSchemaDefinition `json:"json"`
}

// JSONSchemaDefinition represents the definition of a JSON schema
type JSONSchemaDefinition struct {
	Type       string                 `json:"type"`
	Properties map[string]PropertyDef `json:"properties"`
	Required   []string               `json:"required"`
}

// PropertyDef represents a property definition in JSON schema
type PropertyDef struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ToolConfig represents the configuration for tools
type ToolConfig struct {
	Tools      []Tool     `json:"tools"`
	ToolChoice ToolChoice `json:"toolChoice"`
}

// Tool represents a tool
type Tool struct {
	ToolSpec ToolSpec `json:"toolSpec"`
}

// ToolChoice represents the tool choice configuration
type ToolChoice struct {
	Auto map[string]interface{} `json:"auto"`
}

// Request represents the complete request structure
type Request struct {
	System          []SystemMessage `json:"system"`
	Messages        []Message       `json:"messages"`
	InferenceConfig InferenceConfig `json:"inferenceConfig,omitempty"`
	ToolConfig      *ToolConfig     `json:"toolConfig,omitempty"`
}

// Response is the response from AWS Nova
//
// ResponseMetadata 包含请求的元数据信息
type ResponseMetadata struct {
	RequestId      string            `json:"RequestId"`
	HTTPStatusCode int               `json:"HTTPStatusCode"`
	HTTPHeaders    map[string]string `json:"HTTPHeaders"`
	RetryAttempts  int               `json:"RetryAttempts"`
}

// Usage 包含token使用情况
type Usage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
	TotalTokens  int `json:"totalTokens"`
}

// Metrics 包含性能指标
type Metrics struct {
	LatencyMs int `json:"latencyMs"`
}

// OutputMessage 包含助手的回复消息
type OutputMessage struct {
	Role    string    `json:"role"`
	Content []Content `json:"content"`
}

// Output 包含响应的主要内容
type Output struct {
	Message OutputMessage `json:"message"`
}

// Response 表示完整的响应结构
type Response struct {
	ResponseMetadata ResponseMetadata `json:"ResponseMetadata"`
	Output           Output           `json:"output"`
	StopReason       string           `json:"stopReason"`
	Usage            Usage            `json:"usage"`
	Metrics          Metrics          `json:"metrics"`
}

// // {'generation': 'Hi', 'prompt_token_count': 15, 'generation_token_count': 1, 'stop_reason': None}
// type StreamResponse struct {
// 	Generation           string `json:"generation"`
// 	PromptTokenCount     int    `json:"prompt_token_count"`
// 	GenerationTokenCount int    `json:"generation_token_count"`
// 	StopReason           string `json:"stop_reason"`
// }
