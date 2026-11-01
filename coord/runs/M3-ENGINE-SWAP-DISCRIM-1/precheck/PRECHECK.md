# M3-ENGINE-SWAP-DISCRIM-1（238）预检腿产物——模型 id + 单价核实

- 时间：2026-11-01 21:1x +0800；凭据=user config 同一账号（baseUrl=https://api.deepseek.com/v1，只读引用，key 不落盘）
- 用户裁定引擎=deepseek-v4（2026-11-01）；user config defaultModel=deepseek-flash（只读亲核，零触碰）

## 1. /models 端点（GET https://api.deepseek.com/models）

- HTTP 200；账号可用模型 id：`deepseek-flash`、`deepseek-v4-pro`
- **`deepseek-v4` 不是有效 id**——按卡面预检条款：记录实际可用强档 id 并采用之 → **本实验采用 `deepseek-v4-pro`**（回执披露）

## 2. 单调用探针（POST /v1/chat/completions, model=deepseek-v4-pro）

- HTTP 200，latency 1996ms；echo_model=**deepseek-v4-pro**（服务端回显亲证）
- usage：prompt 88 / completion 8（reasoning_tokens=8）；思考模式默认开启
- 探针结论：账号有权限，id 可用，无 401/限流

## 3. 单价（官方 pricing 页 https://api-docs.deepseek.com/zh-cn/quick_start/pricing，2026-11-01 抓取）

| 每百万 tokens | deepseek-v4-flash | deepseek-v4-pro | 倍率 |
|---|---|---|---|
| 输入（缓存命中） | ¥0.02 | ¥0.025 | 1.25× |
| 输入（缓存未命中） | ¥1 | ¥3 | **3×** |
| 输出 | ¥2 | ¥6 | **3×** |

- 注：user config 的 `deepseek-flash` id 由 DeepSeek-V4.1-Flash-1017 提供并按 Flash 价计费（quick_start 页脚注），对照基线（237 alignrun1）即此档。
- 上下文 1M/输出 384K 同 flash；并发限制 v4-pro=500（flash=2500），单 run 串行不受影响。

## 4. 预算预估（§8 披露口径）

- 237 flash 基线用量：86 计费调用，1,291,342 in（含 cache）/116,112 out → flash 实付量级 ≈ ¥1.3~1.6（按未命中上界 ¥1.29×1+¥0.116×2≈¥1.52）
- v4-pro 同调用结构上界（全未命中）：¥1.29×3+¥0.116×6≈**¥4.57**；有缓存命中则显著低于此（缓存命中价仅 1.25×）
- 单 run 预算 ≤237 量级（卡面），v4-pro 单价 3× 已如实披露，属可承受成本

## 5. 结论

预检通过：v4 强档可用（id=deepseek-v4-pro），无引擎面栈错误，实验腿放行。
