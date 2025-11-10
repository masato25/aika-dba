# LLM 配置指南

本文檔將說明如何配置 Aika DBA 系統中的 LLM（大型語言模型）功能。系統支持多種 LLM 提供者，包括 OpenAI、本地 OpenAI 兼容服務器、Ollama 和 llama.cpp。

## 配置結構

LLM 配置在 `config.yaml` 文件中的 `llm` 部分進行設置。基本配置結構如下：

```yaml
llm:
  provider: "openai"          # LLM 提供者
  model: "gpt-4"             # 模型名稱
  api_key: ""                # API 密鑰（建議使用環境變數）
  base_url: ""               # API 基礎 URL（可選）
  host: "localhost"          # 本地 LLM 主機
  port: 8080                 # 本地 LLM 端口
  timeout_seconds: 60        # 請求超時時間
  context_size: 4096         # 上下文大小 (tokens)
  max_tokens: 1024          # 最大響應 tokens
  temperature: 0.7          # 採樣溫度 (0.0-2.0)
  top_p: 0.9               # Top-p 採樣
  top_k: 40                # Top-k 採樣（僅用於某些提供者）
  stop_words:              # 停止詞列表
    - "</s>"
    - "\n\n"
```

## 提供者特定配置

### 1. OpenAI

使用 OpenAI 的 API 服務：

```yaml
llm:
  provider: "openai"
  model: "gpt-4"            # 或 gpt-3.5-turbo
  api_key: ""               # 從環境變數 OPENAI_API_KEY 讀取
  base_url: ""              # 可選，使用預設 OpenAI API 端點
  temperature: 0.7
  top_p: 0.9
  max_tokens: 1024
```

環境變數設置：
```bash
export OPENAI_API_KEY="your-api-key-here"
export OPENAI_BASE_URL="https://api.openai.com/v1"  # 可選
```

### 2. 本地 OpenAI 兼容服務器

使用與 OpenAI API 兼容的本地服務器（如 vLLM、FastChat）：

```yaml
llm:
  provider: "local"
  model: "mistral-7b"
  host: "localhost"
  port: 8080
  temperature: 0.7
  top_p: 0.9
  max_tokens: 1024
```

### 3. Ollama

使用 Ollama 本地服務：

```yaml
llm:
  provider: "ollama"
  model: "llama2"           # 或其他已在 Ollama 中安裝的模型
  host: "localhost"
  port: 11434
  temperature: 0.7
  top_p: 0.9
  max_tokens: 1024
```

### 4. llama.cpp

使用 llama.cpp API 服務器：

```yaml
llm:
  provider: "llamacpp"
  model: "mistral-7b"       # 模型名稱（僅供記錄）
  host: "localhost"
  port: 8080
  temperature: 0.7
  top_p: 0.9
  top_k: 40                # llama.cpp 特有參數
  stop_words:              # 自定義停止詞
    - "</s>"
    - "\n\n"
  max_tokens: 1024
```

## 參數說明

| 參數 | 說明 | 預設值 | 適用提供者 |
|------|------|--------|------------|
| `provider` | LLM 提供者名稱 | - | 全部 |
| `model` | 使用的模型名稱 | - | 全部 |
| `api_key` | API 密鑰 | - | OpenAI |
| `base_url` | API 基礎 URL | 提供者預設 | OpenAI, Local |
| `host` | 服務器主機地址 | localhost | Local, Ollama, LlamaCpp |
| `port` | 服務器端口 | - | Local, Ollama, LlamaCpp |
| `temperature` | 採樣溫度 | 0.7 | 全部 |
| `top_p` | 核心採樣參數 | 0.9 | 全部 |
| `top_k` | Top-K 採樣 | 40 | LlamaCpp |
| `max_tokens` | 最大輸出 tokens | 1024 | 全部 |
| `context_size` | 上下文窗口大小 | 4096 | 全部 |
| `stop_words` | 停止生成的詞列表 | [] | LlamaCpp |

## 環境變數

支持通過環境變數覆蓋配置文件中的設置：

- `OPENAI_API_KEY`: OpenAI API 密鑰
- `OPENAI_BASE_URL`: OpenAI API 基礎 URL
- `LLM_HOST`: LLM 服務器主機
- `LLM_PORT`: LLM 服務器端口
- `LLM_MODEL`: 模型名稱
- `LLM_CONTEXT_SIZE`: 上下文大小
- `LLM_MAX_TOKENS`: 最大輸出 tokens

## 最佳實踐

1. **API 密鑰安全性**：
   - 永遠不要在配置文件中直接存儲 API 密鑰
   - 使用環境變數來設置敏感資訊

2. **性能優化**：
   - 根據需求調整 `context_size` 和 `max_tokens`
   - 較小的值可以提高響應速度和減少資源使用

3. **質量控制**：
   - 較低的 `temperature`（0.1-0.3）適合精確任務
   - 較高的 `temperature`（0.7-1.0）適合創意任務

4. **錯誤處理**：
   - 設置合適的 `timeout_seconds` 避免請求掛起
   - 使用 `stop_words` 控制輸出長度和格式

## 故障排除

1. 連接錯誤：
   - 檢查 host 和 port 設置
   - 確認服務器是否正在運行
   - 檢查防火牆設置

2. 授權錯誤：
   - 確認 API 密鑰是否正確設置
   - 檢查環境變數是否正確導出

3. 響應超時：
   - 增加 `timeout_seconds` 值
   - 減少 `max_tokens` 或 `context_size`

4. 生成質量問題：
   - 調整 `temperature` 和 `top_p` 參數
   - 檢查是否使用了合適的模型