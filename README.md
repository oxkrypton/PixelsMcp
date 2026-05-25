# PixelsMcp

PixelsMcp 是一个面向远程部署的 MCP 图像生成服务。推荐的生产环境部署方式是将服务运行在服务器上，对外暴露一个 HTTP MCP 地址供用户的 AI Agent 连接使用。用户无需下载二进制文件、配置本地环境，也不需要在自己的机器上提供图像生成服务的 API 密钥。

视频演示：[PixelsMcp 使用演示](https://www.bilibili.com/video/BV1mwGZ6gEQ2/?spm_id_from=333.1387.homepage.video_card.click&vd_source=4686f9b9ad86488be59c4d32d28ac14d)

## 项目结构

```
PixelsMcp/
├── cmd/pixelsmcp/        # 主程序入口
├── internal/
│   ├── app/              # 应用初始化、配置、HTTP 服务
│   ├── server/           # MCP 服务端、工具定义与处理
│   └── service/
│       └── imagegen/     # 图像生成核心服务（Provider、文件名、提示词等）
├── deploy/               # 部署相关文件（systemd 服务等）
├── frontend/             # Web 前端界面
└── README.md
```

## 云端 / HTTP MCP 服务

在服务器上配置好 Provider 凭证后启动服务：

```bash
PIXELSMCP_API_KEY=your-key \
PIXELSMCP_BASE_URL=https://api.example.com \
PIXELSMCP_MODEL=your-model \
PIXELSMCP_REFERENCE_MODEL=your-model \
go run ./cmd/pixelsmcp
```

默认传输方式为 HTTP。MCP 端点地址：

```text
http://127.0.0.1:8080/mcp
```

健康检查端点：

```text
http://127.0.0.1:8080/healthz
```

部署完成后，将公网 MCP 地址提供给用户即可，例如：

```text
https://your-domain.example/mcp
```

### Claude 客户端配置

用户可以在 `~/.mcp.json` 中添加远程 MCP 地址来让 Claude 连接到已部署的服务：

```toml
[mcp_servers.pixelsmcp]
url = "https://your-domain.example/mcp"
```

Provider、API 密钥、模型以及图片存储等配置全部保留在服务端。用户只需要知道已部署的 MCP 地址即可使用。

## 开发者本地调试

`init` 和 `setup` 命令是为克隆了开源项目、希望在本地调试或修改服务的开发者准备的。它们会在项目根目录下创建 `.env.local` 文件并校验 Provider 配置。

```bash
git clone https://github.com/oxkrypton/PixelsMcp.git
cd PixelsMcp
go run ./cmd/pixelsmcp setup   # 或: go run ./cmd/pixelsmcp init
go run ./cmd/pixelsmcp
```

仅用于本地 MCP 客户端调试时，开发者也可以显式使用 stdio 传输方式：

```bash
PIXELSMCP_TRANSPORT=stdio go run ./cmd/pixelsmcp
```

## 配置项

以下环境变量均为服务端配置。连接到已部署 MCP 地址的终端用户无需设置它们。

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PIXELSMCP_TRANSPORT` | `http` | 传输方式。`http` 用于云端/服务器部署，`stdio` 仅用于本地 MCP 客户端调试。 |
| `PIXELSMCP_ADDR` | `:8080` | HTTP 监听地址（仅在 `http` 传输方式下生效）。 |
| `PIXELSMCP_ENDPOINT` | `/mcp` | MCP HTTP 端点路径。 |
| `PIXELSMCP_HEALTH_ENDPOINT` | `/healthz` | 健康检查端点路径。 |
| `PIXELSMCP_CORS_ORIGINS` | 空 | 允许的浏览器跨域来源，多个以逗号分隔。 |
| `PIXELSMCP_PROVIDER` | `openai-compatible` | 服务端使用的图像生成 Provider 适配器。 |
| `PIXELSMCP_API_KEY` | 必填 | 服务端调用图像生成 API 使用的密钥。 |
| `PIXELSMCP_BASE_URL` | 必填 | 服务端图像生成 API 的基础地址。 |
| `PIXELSMCP_MODEL` | `Kwai-Kolors/Kolors` | 服务端默认使用的图像生成模型。 |
| `PIXELSMCP_REFERENCE_MODEL` | `Qwen/Qwen-Image-Edit-2509` | 当请求包含参考图 `reference_image` 时使用的编辑模型。 |
| `PIXELSMCP_EXTRA_HEADERS` | 空 | 额外的 Provider 请求头，支持 JSON 对象或键值对列表格式。 |
| `PIXELSMCP_TIMEOUT` | `2m0s` | 请求 Provider 的 HTTP 超时时间。 |
| `PIXELSMCP_IMAGE_SAVE_DIR` | `./generated-images` | 服务端保存生成图片的目录（当 `output_path` 未指定时使用）。 |

```

## 工具

- `generate_image`：根据 `prompt` 生成单张图片。支持可选的 `reference_image`（参考图）、`background_color`（背景色）、调优参数、`seed`（随机种子）、`negative_prompt`（负向提示词）和 `output_path`（输出路径）。图片保存在服务端并返回文件信息。
- `generate_sprite_sheet`：根据 `prompt` 生成精灵表（sprite sheet）。支持 `action`（动作描述）、`frame_count`（帧数）、`layout`（布局）等参数，以及可选的 `reference_image`、`frame_width`、`frame_height`、`spacing`、`background_color`、调优参数、`seed`、`negative_prompt` 和 `output_path`。图片保存在服务端并返回文件信息。

`background_color` 用于设置纯色背景，例如 `#00FF00`（绿色）或 `#FF00FF`（品红色）。精灵表默认帧尺寸为 64x64、间距 2px、浅灰色背景（如果不指定这些几何参数和 `background_color`）。

`output_path` 用于指定图片写入的本地工作区路径，必须是绝对路径。如果指向一个已存在的目录，PixelsMcp 会在该目录下自动命名并保存图片；如果指向一个文件路径，PixelsMcp 会写入该文件，且在缺少扩展名时自动补充图片扩展名。响应中的 `saved_path` 是服务端实际写入的绝对路径，`local_path` 作为兼容性别名保留。

`reference_image` 仅在需要服务端通过 `PIXELSMCP_REFERENCE_MODEL` 处理参考图时使用。它可以是 http(s) 链接、`data:image/...` 格式的 base64 地址，或不带 `data:` 前缀的原始 base64 图片数据。如果你的客户端会自动将 `data:image/...` 字符串渲染为图像，建议发送原始 base64 数据（不含 `data:` 前缀），服务端会自动补充前缀后再调用 Provider。如果仍然需要纯文生图功能，请不要将默认的 `PIXELSMCP_MODEL` 设为仅支持参考图编辑的模型。对于 SiliconFlow 的 Qwen 编辑模型，`image_size` 和 `guidance_scale` 会自动被省略，因为这些模型不支持这些字段。

### 图片生成参数示例

```json
{
  "prompt": "一位赛博朋克工程师的肖像",
  "background_color": "#FF00FF",
  "reference_image": "https://example.com/reference.png",
  "image_size": "1024x1024",
  "guidance_scale": 7.5,
  "num_inference_steps": 28,
  "seed": 12345,
  "negative_prompt": "模糊，低质量",
  "output_path": "/Users/krypton/Documents/go-tiny-claw/generated-images/cyberpunk-engineer.png"
}
```

### 精灵表生成参数示例

```json
{
  "prompt": "像素风蓝色披风骑士",
  "action": "行走",
  "frame_count": 8,
  "layout": "horizontal",
  "frame_width": 64,
  "frame_height": 64,
  "spacing": 2,
  "background_color": "#00FF00",
  "reference_image": "data:image/png;base64,...",
  "image_size": "1024x1024",
  "guidance_scale": 7.5,
  "num_inference_steps": 28,
  "seed": 12345,
  "negative_prompt": "多余肢体，写实3D，渐变背景",
  "output_path": "/Users/krypton/Documents/go-tiny-claw/generated-images"
}
```

支持的布局提示词包括 `horizontal`（横向）、`vertical`（纵向）和 `3x3`。其他布局文本会原样透传给图像模型，MCP 服务端不会拦截。可选的调优参数、`seed` 和 `negative_prompt` 会在提供时转发给兼容的图像生成 Provider。

## 项目结构

```
PixelsMcp/
├── cmd/pixelsmcp/        # 主程序入口
├── internal/
│   ├── app/              # 应用初始化、配置、HTTP 服务
│   ├── server/           # MCP 服务端、工具定义与处理
│   └── service/
│       └── imagegen/     # 图像生成核心服务（Provider、文件名、提示词等）
├── deploy/               # 部署相关文件（systemd 服务等）
├── frontend/             # Web 前端界面
└── README.md
```

## 许可证

MIT License
