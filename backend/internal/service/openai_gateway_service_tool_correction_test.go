package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestCodexToolCorrector_CustomToolCallNamespace(t *testing.T) {
	t.Parallel()

	const input = `{"cmd":"Write-Output  one  two","literal":"namespace exec must stay in input"}`
	tests := []struct {
		name          string
		payload       string
		itemPath      string
		wantName      string
		wantNamespace string
	}{
		{
			name:     "root exec duplicate namespace",
			payload:  `{"type":"custom_tool_call","call_id":"call_root","name":"exec","namespace":"exec","input":"` + strings.ReplaceAll(input, `"`, `\"`) + `"}`,
			itemPath: "",
			wantName: "exec",
		},
		{
			name:     "stream item exec namespace",
			payload:  `{"type":"response.output_item.added","item":{"type":"custom_tool_call","call_id":"call_item","name":"exec","namespace":"tools","input":"raw input"}}`,
			itemPath: "item",
			wantName: "exec",
		},
		{
			name:     "top level output duplicate namespace",
			payload:  `{"output":[{"type":"custom_tool_call","call_id":"call_output","name":"shell","namespace":"shell","input":"pwd"}]}`,
			itemPath: "output.0",
			wantName: "shell",
		},
		{
			name:     "terminal response output fills missing exec name",
			payload:  `{"type":"response.completed","response":{"output":[{"type":"custom_tool_call","call_id":"call_terminal","namespace":"exec","input":"dir"}]}}`,
			itemPath: "response.output.0",
			wantName: "exec",
		},
		{
			name:          "non duplicate namespace remains",
			payload:       `{"type":"custom_tool_call","call_id":"call_other","name":"search","namespace":"web","input":"query"}`,
			itemPath:      "",
			wantName:      "search",
			wantNamespace: "web",
		},
	}

	corrector := NewCodexToolCorrector()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := corrector.CorrectToolCallsInSSEBytes([]byte(tt.payload))
			if tt.wantNamespace == "" {
				require.True(t, changed)
			} else {
				require.False(t, changed)
			}

			prefix := tt.itemPath
			if prefix != "" {
				prefix += "."
			}
			require.Equal(t, tt.wantName, gjson.GetBytes(got, prefix+"name").String())
			if tt.wantNamespace == "" {
				require.False(t, gjson.GetBytes(got, prefix+"namespace").Exists())
			} else {
				require.Equal(t, tt.wantNamespace, gjson.GetBytes(got, prefix+"namespace").String())
			}
		})
	}
}

func TestOpenAIGatewayService_CustomToolCallPreservesInputAndCallID(t *testing.T) {
	t.Parallel()

	service := &OpenAIGatewayService{toolCorrector: NewCodexToolCorrector()}
	payload := []byte(`{
		"output":[
			{"type":"custom_tool_call","call_id":"call_same","name":"exec","namespace":"exec","input":"{\n  \"cmd\": \"echo  one  two\"\n}"},
			{"type":"custom_tool_call_output","call_id":"call_same","output":"done"}
		]
	}`)

	beforeInput := gjson.GetBytes(payload, "output.0.input").String()
	got := service.correctToolCallsInResponseBody(payload)

	require.Equal(t, beforeInput, gjson.GetBytes(got, "output.0.input").String())
	require.Equal(t, "call_same", gjson.GetBytes(got, "output.0.call_id").String())
	require.Equal(t, "call_same", gjson.GetBytes(got, "output.1.call_id").String())
	require.False(t, gjson.GetBytes(got, "output.0.namespace").Exists())
	require.Equal(t, "done", gjson.GetBytes(got, "output.1.output").String())
}

func TestOpenAIGatewayService_CustomToolCallNamespaceInSSEBody(t *testing.T) {
	t.Parallel()

	service := &OpenAIGatewayService{toolCorrector: NewCodexToolCorrector()}
	body := "event: response.output_item.added\n" +
		`data: {"type":"response.output_item.added","item":{"type":"custom_tool_call","call_id":"call_sse","name":"exec","namespace":"exec","input":"echo  unchanged"}}` + "\n\n" +
		"event: response.completed\n" +
		`data: {"type":"response.completed","response":{"output":[{"type":"custom_tool_call","call_id":"call_sse","name":"exec","namespace":"exec","input":"echo  unchanged"}]}}` + "\n\n"

	got := service.correctToolCallsInSSEBody(body)

	require.NotContains(t, got, `"namespace":"exec"`)
	require.Equal(t, 2, strings.Count(got, `"call_id":"call_sse"`))
	require.Equal(t, 2, strings.Count(got, `"input":"echo  unchanged"`))
	require.Equal(t, 2, strings.Count(got, `"name":"exec"`))
}

// TestOpenAIGatewayService_ToolCorrection 测试 OpenAIGatewayService 中的工具修正集成
func TestOpenAIGatewayService_ToolCorrection(t *testing.T) {
	// 创建一个简单的 service 实例来测试工具修正
	service := &OpenAIGatewayService{
		toolCorrector: NewCodexToolCorrector(),
	}

	tests := []struct {
		name     string
		input    []byte
		expected string
		changed  bool
	}{
		{
			name: "correct apply_patch in response body",
			input: []byte(`{
				"choices": [{
					"message": {
						"tool_calls": [{
							"function": {"name": "apply_patch"}
						}]
					}
				}]
			}`),
			expected: "edit",
			changed:  true,
		},
		{
			name: "correct update_plan in response body",
			input: []byte(`{
				"tool_calls": [{
					"function": {"name": "update_plan"}
				}]
			}`),
			expected: "todowrite",
			changed:  true,
		},
		{
			name: "no change for correct tool name",
			input: []byte(`{
				"tool_calls": [{
					"function": {"name": "edit"}
				}]
			}`),
			expected: "edit",
			changed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.correctToolCallsInResponseBody(tt.input)
			resultStr := string(result)

			// 检查是否包含期望的工具名称
			if !strings.Contains(resultStr, tt.expected) {
				t.Errorf("expected result to contain %q, got %q", tt.expected, resultStr)
			}

			// 对于预期有变化的情况，验证结果与输入不同
			if tt.changed && string(result) == string(tt.input) {
				t.Error("expected result to be different from input, but they are the same")
			}

			// 对于预期无变化的情况，验证结果与输入相同
			if !tt.changed && string(result) != string(tt.input) {
				t.Error("expected result to be same as input, but they are different")
			}
		})
	}
}

// TestOpenAIGatewayService_ToolCorrectorInitialization 测试工具修正器是否正确初始化
func TestOpenAIGatewayService_ToolCorrectorInitialization(t *testing.T) {
	service := &OpenAIGatewayService{
		toolCorrector: NewCodexToolCorrector(),
	}

	if service.toolCorrector == nil {
		t.Fatal("toolCorrector should not be nil")
	}

	// 测试修正器可以正常工作
	data := `{"tool_calls":[{"function":{"name":"apply_patch"}}]}`
	corrected, changed := service.toolCorrector.CorrectToolCallsInSSEData(data)

	if !changed {
		t.Error("expected tool call to be corrected")
	}

	if !strings.Contains(corrected, "edit") {
		t.Errorf("expected corrected data to contain 'edit', got %q", corrected)
	}
}

// TestToolCorrectionStats 测试工具修正统计功能
func TestToolCorrectionStats(t *testing.T) {
	service := &OpenAIGatewayService{
		toolCorrector: NewCodexToolCorrector(),
	}

	// 执行几次修正
	testData := []string{
		`{"tool_calls":[{"function":{"name":"apply_patch"}}]}`,
		`{"tool_calls":[{"function":{"name":"update_plan"}}]}`,
		`{"tool_calls":[{"function":{"name":"apply_patch"}}]}`,
	}

	for _, data := range testData {
		service.toolCorrector.CorrectToolCallsInSSEData(data)
	}

	stats := service.toolCorrector.GetStats()

	if stats.TotalCorrected != 3 {
		t.Errorf("expected 3 corrections, got %d", stats.TotalCorrected)
	}

	if stats.CorrectionsByTool["apply_patch->edit"] != 2 {
		t.Errorf("expected 2 apply_patch->edit corrections, got %d", stats.CorrectionsByTool["apply_patch->edit"])
	}

	if stats.CorrectionsByTool["update_plan->todowrite"] != 1 {
		t.Errorf("expected 1 update_plan->todowrite correction, got %d", stats.CorrectionsByTool["update_plan->todowrite"])
	}
}
