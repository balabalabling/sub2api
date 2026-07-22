# Responses `custom_tool_call` Namespace 修复设计

## 背景

部分 OpenAI Responses 兼容上游会返回以下工具调用项：

```json
{
  "type": "custom_tool_call",
  "call_id": "call_example",
  "name": "exec",
  "namespace": "exec",
  "input": "{}"
}
```

Codex Desktop 会把 `namespace` 与 `name` 组合成工具路由名，因而把该调用识别为 `execexec`。sub2api 目前只清理发往上游的历史输入项 `namespace`，尚未统一清理上游响应中的重复字段。

## 目标

在所有 OpenAI Responses 出站路径中规范化 `custom_tool_call`：

1. `namespace == name` 时删除 `namespace`。
2. 工具名为 `exec` 时输出 `name: "exec"`，且删除 `namespace`。
3. 当 `namespace == "exec"` 且 `name` 为空时，补齐 `name: "exec"` 并删除 `namespace`。
4. 保持 `call_id`、`input` 及其他字段原样。
5. `custom_tool_call_output` 不参与字段改写，其 `call_id` 原样透传。

## 非目标

- 不递归删除任意业务数据中的 `namespace`。
- 不调整 `function_call`、MCP 工具或工具定义的命名空间语义。
- 不解析、重写或重新序列化 `custom_tool_call.input` 内容。
- 不改变账号路由、模型映射或请求侧工具定义。

## 设计

### 集中式响应规范化

在现有响应工具修正层增加一个纯 JSON 规范化函数。函数只检查以下 Responses 协议位置：

- 根对象自身；
- `item`；
- `output[]`；
- `response.output[]`。

只有对象的 `type` 为 `custom_tool_call` 时才应用规则。其他对象及嵌套的 `input` 数据保持不变。

### 出站路径

统一把规范化逻辑接入：

1. HTTP 流式 Responses 的每个 SSE data 事件；
2. HTTP 非流式 JSON 响应；
3. 上游 SSE 聚合为非流式 JSON 的响应；
4. Responses WebSocket 事件及 HTTP/WebSocket 桥接路径；
5. 流式终止事件中携带的 `response.output[]`。

现有 `correctToolCallsInResponseBody` 和事件修正入口作为公共接入点，避免各协议分支复制命名规则。

## 数据完整性

规范化前后必须满足：

- `call_id` 字符串逐字相同；
- `input` 的 JSON 类型和值完全相同；字符串输入也保持逐字相同；
- 对应的 `custom_tool_call_output.call_id` 不发生变化；
- 非重复命名空间不因本修复被全局删除。

## 测试

增加表驱动单元测试，覆盖：

1. `name="exec", namespace="exec"` 删除命名空间；
2. `name="exec"` 搭配其他命名空间仍删除命名空间；
3. `namespace="exec"` 且缺少 `name` 时补齐名称；
4. 其他工具的 `namespace == name` 删除重复字段；
5. 其他工具的非重复命名空间保持原状；
6. 根对象、`item`、`output[]`、`response.output[]` 四种结构；
7. SSE、非流式 JSON、WebSocket 出站入口；
8. `input` 与 `call_id` 修复前后完全一致；
9. `custom_tool_call_output.call_id` 保持一致。

## 验收标准

Codex Desktop 最终收到的 exec 调用为：

```json
{
  "type": "custom_tool_call",
  "call_id": "call_example",
  "name": "exec",
  "input": "{}"
}
```

响应中不再出现 exec 工具的 `namespace` 字段，且工具输出能够使用同一个 `call_id` 继续会话。
