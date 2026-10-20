package ai

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/tiktoken-go/tokenizer"
)

type Usage struct {
	Input          int
	Output         int
	Cached         int
	CacheWrite     int
	Reasoning      int
	InputKnown     bool
	OutputKnown    bool
	CachedKnown    bool
	ReasoningKnown bool
}

func (usage *Usage) Merge(next Usage) {
	if next.InputKnown {
		usage.Input = next.Input
		usage.CacheWrite = next.CacheWrite
		usage.InputKnown = true
	}
	if next.OutputKnown {
		usage.Output = next.Output
		usage.OutputKnown = true
	}
	if next.CachedKnown {
		usage.Cached = next.Cached
		usage.CachedKnown = true
	}
	if next.ReasoningKnown {
		usage.Reasoning = next.Reasoning
		usage.ReasoningKnown = true
	}
}

type ContextPart struct {
	Name   string
	Bytes  int
	Tokens int
}
type ContextSize struct {
	Bytes          int
	Tokens         int
	Method         string
	Parts          []ContextPart
	HistoryOmitted bool
}

type RequestMetrics struct {
	Elapsed    time.Duration
	Round      int
	Complete   bool
	Context    ContextSize
	WireBytes  int
	Usage      Usage
	Duration   time.Duration
	FirstToken time.Duration
	FirstSQL   time.Duration
	Tools      time.Duration
	Failed     bool
}

type DiscoveryOptions struct {
	Model   string
	Observe func(RequestMetrics)
}

type wireObservation struct{ bytes int }
type observationKey struct{}

func recordWireBytes(ctx context.Context, size int) {
	if observation, ok := ctx.Value(observationKey{}).(*wireObservation); ok {
		observation.bytes = size
	}
}

type localCounter struct {
	mu    sync.Mutex
	codec tokenizer.Codec
}

var localCounters sync.Map

func countLocal(model, text string) (int, string) {
	value, ok := localCounters.Load(model)
	if !ok {
		codec, _ := tokenizer.ForModel(tokenizer.Model(model))
		value, _ = localCounters.LoadOrStore(model, &localCounter{codec: codec})
	}
	counter := value.(*localCounter)
	if counter.codec != nil {
		counter.mu.Lock()
		tokens, _, err := counter.codec.Encode(text)
		counter.mu.Unlock()
		if err == nil {
			return len(tokens), counter.codec.GetName() + " estimate"
		}
	}
	return (len(text) + 3) / 4, "byte estimate (bytes / 4)"
}

func prepareRequest(request Request) (Request, []byte, error) {
	data, err := json.Marshal(request)
	return request, data, err
}

func MeasureContext(model string, request Request) ContextSize {
	request, data, err := prepareRequest(request)
	if err != nil {
		return ContextSize{}
	}
	return measureMessages(model, request, []Message{{Role: "user", Text: string(data)}})
}

func measureMessages(model string, request Request, messages []Message) ContextSize {
	encoded, _ := json.Marshal(messages)
	definitions, _ := json.Marshal(metadataTools())
	size := ContextSize{Bytes: len(encoded) + len(definitions) + len(discoveryInstructions), HistoryOmitted: request.HistoryTruncated}
	add := func(name string, data []byte) {
		if len(data) == 0 {
			return
		}
		tokens, method := countLocal(model, string(data))
		size.Method = method
		size.Parts = append(size.Parts, ContextPart{Name: name, Bytes: len(data), Tokens: tokens})
	}
	encode := func(value any) []byte { data, _ := json.Marshal(value); return data }
	add("Instructions + tools", append([]byte(discoveryInstructions), definitions...))
	if len(request.History) > 0 {
		add("Conversation", encode(request.History))
	}
	schema := []byte{}
	if len(request.Tables) > 0 {
		schema = append(schema, encode(request.Tables)...)
	}
	if len(request.Databases) > 0 {
		schema = append(schema, encode(request.Databases)...)
	}
	if len(request.Catalog) > 0 {
		schema = append(schema, encode(request.Catalog)...)
	}
	if len(request.Schemas) > 0 {
		schema = append(schema, encode(request.Schemas)...)
	}
	for _, message := range messages {
		if message.Role == "tool" {
			schema = append(schema, encode(message.Text)...)
		}
	}
	add("Schema context", schema)
	add("Current message", encode(struct{ Prompt, Question, Answer string }{request.Prompt, request.Clarification, request.Answer}))
	if request.SelectedSQL != "" || request.EditorSQL != "" {
		add("Editor context", encode(struct{ Selected, Editor string }{request.SelectedSQL, request.EditorSQL}))
	}
	counted := 0
	for _, part := range size.Parts {
		counted += part.Bytes
	}
	overhead := max(0, size.Bytes-counted)
	if overhead > 0 {
		size.Parts = append(size.Parts, ContextPart{Name: "Encoding / discovery state", Bytes: overhead, Tokens: (overhead + 3) / 4})
	}
	for _, part := range size.Parts {
		size.Tokens += part.Tokens
	}
	return size
}

func decodeUsage(format string, data []byte) Usage {
	var envelope struct {
		Usage struct {
			Input        *int `json:"input_tokens"`
			Output       *int `json:"output_tokens"`
			Prompt       *int `json:"prompt_tokens"`
			Completion   *int `json:"completion_tokens"`
			Read         *int `json:"cache_read_input_tokens"`
			Write        *int `json:"cache_creation_input_tokens"`
			InputDetails struct {
				Cached *int `json:"cached_tokens"`
			} `json:"input_tokens_details"`
			PromptDetails struct {
				Cached *int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
			OutputDetails struct {
				Reasoning *int `json:"reasoning_tokens"`
			} `json:"output_tokens_details"`
			CompletionDetails struct {
				Reasoning *int `json:"reasoning_tokens"`
			} `json:"completion_tokens_details"`
		} `json:"usage"`
		Google struct {
			Input     *int `json:"promptTokenCount"`
			Output    *int `json:"candidatesTokenCount"`
			Cached    *int `json:"cachedContentTokenCount"`
			Reasoning *int `json:"thoughtsTokenCount"`
		} `json:"usageMetadata"`
	}
	if json.Unmarshal(data, &envelope) != nil {
		return Usage{}
	}
	usage := Usage{}
	set := func(value *int, target *int, known *bool) {
		if value != nil && *value >= 0 {
			*target = *value
			*known = true
		}
	}
	switch format {
	case "gemini":
		set(envelope.Google.Input, &usage.Input, &usage.InputKnown)
		set(envelope.Google.Output, &usage.Output, &usage.OutputKnown)
		set(envelope.Google.Cached, &usage.Cached, &usage.CachedKnown)
		set(envelope.Google.Reasoning, &usage.Reasoning, &usage.ReasoningKnown)
		if usage.ReasoningKnown {
			usage.Output += usage.Reasoning
			usage.OutputKnown = true
		}
	case "anthropic":
		set(envelope.Usage.Input, &usage.Input, &usage.InputKnown)
		set(envelope.Usage.Output, &usage.Output, &usage.OutputKnown)
		set(envelope.Usage.Read, &usage.Cached, &usage.CachedKnown)
		if envelope.Usage.Write != nil && *envelope.Usage.Write >= 0 {
			usage.CacheWrite = *envelope.Usage.Write
		}
		if usage.InputKnown {
			usage.Input += usage.Cached + usage.CacheWrite
		}
	case "responses":
		set(envelope.Usage.Input, &usage.Input, &usage.InputKnown)
		set(envelope.Usage.Output, &usage.Output, &usage.OutputKnown)
		set(envelope.Usage.InputDetails.Cached, &usage.Cached, &usage.CachedKnown)
		set(envelope.Usage.OutputDetails.Reasoning, &usage.Reasoning, &usage.ReasoningKnown)
	default:
		set(envelope.Usage.Prompt, &usage.Input, &usage.InputKnown)
		set(envelope.Usage.Completion, &usage.Output, &usage.OutputKnown)
		set(envelope.Usage.PromptDetails.Cached, &usage.Cached, &usage.CachedKnown)
		set(envelope.Usage.CompletionDetails.Reasoning, &usage.Reasoning, &usage.ReasoningKnown)
	}
	return usage
}
