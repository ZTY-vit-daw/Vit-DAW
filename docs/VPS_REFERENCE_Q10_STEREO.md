# VPS 范文 — Waves Q10 Stereo
# Vit Plugin Skill Reference Document
#
# 用途：本文件是统一执行接口（executor）的契约范本。
# 任何满足此格式的 VPS 文档，执行器保证能按其描述控制对应效果器。
# 新插件的 VPS 应以本文件为模板，逐字段填写，不得跳过必填字段。
#
# 数据来源：
#   - 参数扫描：artifacts/waves_eq_topology_census/20260727_201627/raw/pages/q10_stereo/
#   - 生产烟测：artifacts/waves_eq_topology_census/20260727_201627/analysis/static_eq_phase3/live_smoke/
#   - 验收证据：Q10 真实执行 3396Hz / -3.0dB / Q=0.5，modify/disable/undo 全部通过

schema: vps/eq/v1

# ══════════════════════════════════════════════════════════════════════════════
# 必填：插件身份与版本绑定
# ══════════════════════════════════════════════════════════════════════════════

plugin_display_name: "Q10 Stereo"
plugin_format: VST3

topology_generation: "eqt1_23ce897133dda080c9ef87e5"
# topology_generation 是参数结构的哈希指纹（不含插件名、不含当前旋钮值）。
# 如果插件更新后参数结构发生变化，此哈希将不再匹配，执行器将拒绝使用本文档
# 并要求重新生成。这是防止 stale VPS 静默写错参数的核心保护机制。
# 填写方法：运行 plugin.get_parameters 后提交给结构识别器，取其输出的
# topology_generation 字段值。

# ══════════════════════════════════════════════════════════════════════════════
# 必填：通道契约
# ══════════════════════════════════════════════════════════════════════════════

channel_contract: shared
# shared   ：所有 EQ 参数通过单一 parameter_id 同时控制左右声道。
#            执行器对每个 role 只写一个参数。Q10 属于此类。
#
# mirrored ：左右声道各有独立 parameter_id，必须分别写入才能完整控制。
#            执行器会为每个 role 生成两条写入步骤（left + right）。
#            示例：EMO-F2 Stereo 的 Low Cut 需要写
#              Left Frequency / Right Frequency / Left On/Off / Right On/Off。
#
# 注意：如果你测试的插件在写入单声道参数后发现另一侧也跟着变化，
#       说明插件内部有 linked 模式，需要在 vps_notes 里说明，
#       但 channel_contract 仍然填 shared（执行器不需要额外写入）。
#       如果另一侧没有跟着变，且每个声道各有独立参数，则填 mirrored，
#       并在每个 role 下同时提供 left 和 right 的 param_id。

# ══════════════════════════════════════════════════════════════════════════════
# 必填：section 寻址方式
# ══════════════════════════════════════════════════════════════════════════════

addressing: resident
# resident    ：section 常驻，不能被创建或删除。
#               支持 upsert / modify / disable / undo，不支持 remove。
#               Q10 的10个频段始终存在。
#
# allocatable ：section 有完整的 Used/Unused 生命周期，可以被分配和释放。
#               支持全部五种 action 包括 remove。
#               示例：FabFilter Pro-Q 3 的频段从 Unused 状态创建。

# ══════════════════════════════════════════════════════════════════════════════
# 必填：EQ section 列表
# 每个 section 代表一个可独立控制的 EQ 频段。
# 执行器根据用户请求的目标频率，选择最近的 section 执行写入。
# ══════════════════════════════════════════════════════════════════════════════

sections:

  # ── Band 1（完整示例，其余频段结构相同）──────────────────────────────────

  - section_key: "1"
    # section_key 是执行器用于 control_ref 的内部稳定标识符。
    # 对于 Q10 这类"Band N"结构，直接使用数字字符串。
    # 对于命名段（LF/HMF/HF），使用其名称（如 "lf"、"hmf"）。

    display_name: "Band 1"
    # display_name 仅用于错误信息和日志，不参与执行逻辑。

    roles:
      # ── role: frequency ────────────────────────────────────────────────────
      # 必填。执行器用此参数将频段移动到目标 Hz。
      frequency:
        param_id: "3"
        # param_id 是 plugin.get_parameters 返回的参数 ID。
        # 对 Q10：Band N 的 frequency param_id = (N-1)*5 + 3
        # Band 1=3, Band 2=8, Band 3=13 ... Band 10=48

        param_name: "Band 1 Frq"
        # param_name 仅供人类阅读，不参与执行。

        channel: shared
        # shared：单个参数控制双声道。
        # left / right：填写 channel_contract: mirrored 时使用。

        physical_domain:
          unit: Hz
          min: 16.0
          max: 21357.0
          # min/max 来自探针曲线端点的 display value 解析。
          # 执行器用这两个值做写前越界检查：
          # 目标 Hz 超出 [min, max] 范围时整批拒绝，不量化。

          curve:
            # 5点校准曲线：[normalized_value, physical_value]
            # normalized_value：VST3 [0.0, 1.0] 归一化值
            # physical_value：对应的 Hz 数值（从 display text 解析）
            # 这5个点必须来自对实际插件的 value-to-string 探针测量。
            # 执行器用这5点做双向插值：Hz → normalized（写入）和 normalized → Hz（readback 验证）。
            - [0.00,    16.0]
            - [0.25,    95.0]
            - [0.50,   578.0]
            - [0.75,  3513.0]
            - [1.00, 21357.0]

          value_law: logarithmic
          # logarithmic：频率参数几乎都是对数曲线（人耳感知是对数的）。
          # linear：增益类参数通常是线性。
          # enumerated：离散档位，见 enumerated_values 字段。
          # 当 value_law=logarithmic 时，执行器用对数插值；linear 用线性插值。
          # 曲线校准点不均匀分布时，两种插值都可能有误差——校准点越多越准确。

      # ── role: gain ─────────────────────────────────────────────────────────
      # Bell / Shelf 类型必填。Low/High Cut 不需要 gain，填了会被执行器忽略。
      gain:
        param_id: "2"
        # Band N gain param_id = (N-1)*5 + 2

        param_name: "Band 1 Gain"
        channel: shared

        physical_domain:
          unit: dB
          min: -18.0
          max: 18.0
          curve:
            - [0.00, -18.0]
            - [0.25,  -9.0]
            - [0.50,   0.0]
            - [0.75,   9.0]
            - [1.00,  18.0]
          value_law: linear

        gain_polarity: bipolar_single
        # bipolar_single：同一参数同时处理正向（boost）和负向（cut），
        #                 0.5 normalized 对应 0 dB，两侧对称。
        #                 这是绝大多数参数化 EQ 的增益结构。Q10 属于此类。
        #
        # split（不支持）：Boost 和 Cut 是两个独立参数（如 PuigTec），
        #                  不能满足"任意双极设点"的 EQ 控制语义，
        #                  执行器不支持此类插件的通用 EQ 控制。

      # ── role: q ────────────────────────────────────────────────────────────
      # Bell / Shelf 类型可选。用户请求中包含 Q 值时必须提供此 role。
      # Low/High Cut 的 Q 等价物通常是 slope（见下），不是 q。
      q:
        param_id: "4"
        # Band N Q param_id = (N-1)*5 + 4

        param_name: "Band 1 Q"
        channel: shared

        physical_domain:
          unit: ""
          # Q 是无量纲数，unit 填空字符串。
          min: 0.5
          max: 100.0
          curve:
            - [0.00,  0.5]
            - [0.25,  1.9]
            - [0.50,  7.1]
            - [0.75, 26.6]
            - [1.00, 100.0]
          value_law: logarithmic

      # ── role: filter_kind ──────────────────────────────────────────────────
      # 必填（如果 section 支持多种 shape）。
      # 如果 section 只有一种固定 shape（如专用 HPF 段），则不需要此 role，
      # 直接在 dedicated_shape 字段声明。
      filter_kind:
        param_id: "1"
        # Band N Type param_id = (N-1)*5 + 1

        param_name: "Band 1 Type"
        channel: shared

        enumerated_values:
          # 每个条目将一个公开 shape 名称映射到插件内部的枚举值。
          # public_shape：固定值，只能是 bell / low_shelf / high_shelf / low_cut / high_cut
          # normalized：写入此枚举项时发送的归一化值（从探针测量）
          # label：执行器 readback 时期望看到的 display text（用于验证写入成功）
          #
          # ⚠ Q10 的 Type 枚举共有6个值：
          #   Bell / Low-Shelf / Hi-Shelf / Low-Pass / Hi-Pass / PQ-Bell
          # 5点探针（0, 0.25, 0.5, 0.75, 1.0）没有直接命中 Hi-Shelf。
          # 推断：6个枚举等间距时 normalized 步长 = 1/5 = 0.2，
          #       Hi-Shelf 的精确 normalized ≈ 0.4（Bell=0, Low-Shelf=0.2, Hi-Shelf=0.4...）
          # 填写方法：在 normalized=0.4 处做单点 value-to-string 测量确认。
          # 下面用 [MEASURE] 标记需要实测确认的字段。

          - public_shape: bell
            normalized: 0.0        # 探针确认：n=0.0 → "Bell"
            label: "Bell"

          - public_shape: low_shelf
            normalized: 0.2        # 推断（探针在0.25处读到"Low-Shelf"，说明0.2在Low-Shelf范围内）
            label: "Low-Shelf"     # [MEASURE] 建议在 normalized=0.2 处确认 display text

          - public_shape: high_shelf
            normalized: 0.4        # [MEASURE] 推断值，必须在目标插件上实测确认
            label: "Hi-Shelf"      # [MEASURE]

          - public_shape: high_cut
            normalized: 0.6        # 推断（探针在0.5处读到"Low-Pass"，说明high_cut在0.6附近）
            label: "Low-Pass"      # [MEASURE] 建议在 normalized=0.6 处确认
            # 命名说明：Low-Pass 滤波器 = 切除高频 = public shape "high_cut"

          - public_shape: low_cut
            normalized: 0.8        # 推断（探针在0.75处读到"Hi-Pass"）
            label: "Hi-Pass"       # 基本确认，精确值建议在0.8处测量
            # 命名说明：Hi-Pass 滤波器 = 切除低频 = public shape "low_cut"

          # PQ-Bell（normalized≈1.0）：非标准形态，不映射到公开 shape，不写入 VPS。
          # 执行器不会使用它。

      # ── role: activation ───────────────────────────────────────────────────
      # 必填（如果 section 有独立开关）。
      # activation role 描述如何打开/关闭频段。
      # 执行器严格遵守写入顺序：activation 永远最后写，其他所有 role 先写。
      activation:
        param_id: "0"
        # Band N On/Off param_id = (N-1)*5 + 0（即 Band N 的第一个参数）

        param_name: "Band 1 On/Off"
        channel: shared

        activation_kind: explicit_toggle
        # explicit_toggle   ：有独立的开关参数（In/Out、On/Off、Enable等）。Q10属于此类。
        # frequency_sentinel：没有独立开关，靠将频率写成特殊"Off"值来关闭频段。
        #                     示例：VEQ3 的 HPF 将频率从 "OFF" 枚举写到目标档位表示打开。
        # always_active     ：没有任何开关，写入参数即生效。
        #                     示例：API-560 的固定频点不需要激活步骤。

        active_value:
          normalized: 0.5    # 探针确认：n=0.5 → "In"
          label: "In"
          # 执行器 readback 时验证 display text == "In"。
          # 如果 readback 显示 "Out"，判定激活失败，触发 rollback。

        inactive_value:
          normalized: 0.0    # 探针确认：n=0.0 → "Out"
          label: "Out"
          # disable 操作写此值。

    reachable_shapes: [bell, low_shelf, high_shelf, low_cut, high_cut]
    # 必须与 filter_kind.enumerated_values 中列出的 public_shape 完全一致。
    # 执行器从此列表判断本 section 能否承接用户请求的 shape。
    # 没有 filter_kind role 的专用 section（如纯 HPF）在此只填一个 shape。

  # ── Band 2–10（结构与 Band 1 完全相同，仅 param_id 不同）──────────────

  # param_id 规律：Band N 的各 role param_id = (N-1)*5 + offset
  #   activation  offset = 0
  #   filter_kind offset = 1
  #   gain        offset = 2
  #   frequency   offset = 3
  #   q           offset = 4
  #
  # Band 2: activation=5,  filter_kind=6,  gain=7,  frequency=8,  q=9
  # Band 3: activation=10, filter_kind=11, gain=12, frequency=13, q=14
  # Band 4: activation=15, filter_kind=16, gain=17, frequency=18, q=19
  # Band 5: activation=20, filter_kind=21, gain=22, frequency=23, q=24
  # Band 6: activation=25, filter_kind=26, gain=27, frequency=28, q=29
  # Band 7: activation=30, filter_kind=31, gain=32, frequency=33, q=34
  # Band 8: activation=35, filter_kind=36, gain=37, frequency=38, q=39
  # Band 9: activation=40, filter_kind=41, gain=42, frequency=43, q=44
  # Band 10:activation=45, filter_kind=46, gain=47, frequency=48, q=49

  - section_key: "2"
    display_name: "Band 2"
    roles:
      frequency:  {param_id: "8",  param_name: "Band 2 Frq",   channel: shared, inherits_domain_from: "1.frequency"}
      gain:       {param_id: "7",  param_name: "Band 2 Gain",  channel: shared, inherits_domain_from: "1.gain"}
      q:          {param_id: "9",  param_name: "Band 2 Q",     channel: shared, inherits_domain_from: "1.q"}
      filter_kind:{param_id: "6",  param_name: "Band 2 Type",  channel: shared, inherits_domain_from: "1.filter_kind"}
      activation: {param_id: "5",  param_name: "Band 2 On/Off",channel: shared, inherits_domain_from: "1.activation"}
    reachable_shapes: [bell, low_shelf, high_shelf, low_cut, high_cut]

  - section_key: "3"
    display_name: "Band 3"
    roles:
      frequency:  {param_id: "13", param_name: "Band 3 Frq",   channel: shared, inherits_domain_from: "1.frequency"}
      gain:       {param_id: "12", param_name: "Band 3 Gain",  channel: shared, inherits_domain_from: "1.gain"}
      q:          {param_id: "14", param_name: "Band 3 Q",     channel: shared, inherits_domain_from: "1.q"}
      filter_kind:{param_id: "11", param_name: "Band 3 Type",  channel: shared, inherits_domain_from: "1.filter_kind"}
      activation: {param_id: "10", param_name: "Band 3 On/Off",channel: shared, inherits_domain_from: "1.activation"}
    reachable_shapes: [bell, low_shelf, high_shelf, low_cut, high_cut]

  - section_key: "4"
    display_name: "Band 4"
    roles:
      frequency:  {param_id: "18", param_name: "Band 4 Frq",   channel: shared, inherits_domain_from: "1.frequency"}
      gain:       {param_id: "17", param_name: "Band 4 Gain",  channel: shared, inherits_domain_from: "1.gain"}
      q:          {param_id: "19", param_name: "Band 4 Q",     channel: shared, inherits_domain_from: "1.q"}
      filter_kind:{param_id: "16", param_name: "Band 4 Type",  channel: shared, inherits_domain_from: "1.filter_kind"}
      activation: {param_id: "15", param_name: "Band 4 On/Off",channel: shared, inherits_domain_from: "1.activation"}
    reachable_shapes: [bell, low_shelf, high_shelf, low_cut, high_cut]

  - section_key: "5"
    display_name: "Band 5"
    roles:
      frequency:  {param_id: "23", param_name: "Band 5 Frq",   channel: shared, inherits_domain_from: "1.frequency"}
      gain:       {param_id: "22", param_name: "Band 5 Gain",  channel: shared, inherits_domain_from: "1.gain"}
      q:          {param_id: "24", param_name: "Band 5 Q",     channel: shared, inherits_domain_from: "1.q"}
      filter_kind:{param_id: "21", param_name: "Band 5 Type",  channel: shared, inherits_domain_from: "1.filter_kind"}
      activation: {param_id: "20", param_name: "Band 5 On/Off",channel: shared, inherits_domain_from: "1.activation"}
    reachable_shapes: [bell, low_shelf, high_shelf, low_cut, high_cut]

  - section_key: "6"
    display_name: "Band 6"
    roles:
      frequency:  {param_id: "28", param_name: "Band 6 Frq",   channel: shared, inherits_domain_from: "1.frequency"}
      gain:       {param_id: "27", param_name: "Band 6 Gain",  channel: shared, inherits_domain_from: "1.gain"}
      q:          {param_id: "29", param_name: "Band 6 Q",     channel: shared, inherits_domain_from: "1.q"}
      filter_kind:{param_id: "26", param_name: "Band 6 Type",  channel: shared, inherits_domain_from: "1.filter_kind"}
      activation: {param_id: "25", param_name: "Band 6 On/Off",channel: shared, inherits_domain_from: "1.activation"}
    reachable_shapes: [bell, low_shelf, high_shelf, low_cut, high_cut]

  - section_key: "7"
    display_name: "Band 7"
    roles:
      frequency:  {param_id: "33", param_name: "Band 7 Frq",   channel: shared, inherits_domain_from: "1.frequency"}
      gain:       {param_id: "32", param_name: "Band 7 Gain",  channel: shared, inherits_domain_from: "1.gain"}
      q:          {param_id: "34", param_name: "Band 7 Q",     channel: shared, inherits_domain_from: "1.q"}
      filter_kind:{param_id: "31", param_name: "Band 7 Type",  channel: shared, inherits_domain_from: "1.filter_kind"}
      activation: {param_id: "30", param_name: "Band 7 On/Off",channel: shared, inherits_domain_from: "1.activation"}
    reachable_shapes: [bell, low_shelf, high_shelf, low_cut, high_cut]

  - section_key: "8"
    display_name: "Band 8"
    roles:
      frequency:  {param_id: "38", param_name: "Band 8 Frq",   channel: shared, inherits_domain_from: "1.frequency"}
      gain:       {param_id: "37", param_name: "Band 8 Gain",  channel: shared, inherits_domain_from: "1.gain"}
      q:          {param_id: "39", param_name: "Band 8 Q",     channel: shared, inherits_domain_from: "1.q"}
      filter_kind:{param_id: "36", param_name: "Band 8 Type",  channel: shared, inherits_domain_from: "1.filter_kind"}
      activation: {param_id: "35", param_name: "Band 8 On/Off",channel: shared, inherits_domain_from: "1.activation"}
    reachable_shapes: [bell, low_shelf, high_shelf, low_cut, high_cut]

  - section_key: "9"
    display_name: "Band 9"
    roles:
      frequency:  {param_id: "43", param_name: "Band 9 Frq",   channel: shared, inherits_domain_from: "1.frequency"}
      gain:       {param_id: "42", param_name: "Band 9 Gain",  channel: shared, inherits_domain_from: "1.gain"}
      q:          {param_id: "44", param_name: "Band 9 Q",     channel: shared, inherits_domain_from: "1.q"}
      filter_kind:{param_id: "41", param_name: "Band 9 Type",  channel: shared, inherits_domain_from: "1.filter_kind"}
      activation: {param_id: "40", param_name: "Band 9 On/Off",channel: shared, inherits_domain_from: "1.activation"}
    reachable_shapes: [bell, low_shelf, high_shelf, low_cut, high_cut]

  - section_key: "10"
    display_name: "Band 10"
    roles:
      frequency:  {param_id: "48", param_name: "Band 10 Frq",   channel: shared, inherits_domain_from: "1.frequency"}
      gain:       {param_id: "47", param_name: "Band 10 Gain",  channel: shared, inherits_domain_from: "1.gain"}
      q:          {param_id: "49", param_name: "Band 10 Q",     channel: shared, inherits_domain_from: "1.q"}
      filter_kind:{param_id: "46", param_name: "Band 10 Type",  channel: shared, inherits_domain_from: "1.filter_kind"}
      activation: {param_id: "45", param_name: "Band 10 On/Off",channel: shared, inherits_domain_from: "1.activation"}
    reachable_shapes: [bell, low_shelf, high_shelf, low_cut, high_cut]

# ══════════════════════════════════════════════════════════════════════════════
# 执行器内置规则（不需要填写，此处仅供参考）
# ══════════════════════════════════════════════════════════════════════════════

# executor_write_order:
#   1. filter_kind（如有）
#   2. frequency
#   3. gain（Cut 类型无 gain，跳过）
#   4. q / slope（可选）
#   5. activation（永远最后）
#
# 执行器保证：
#   - 写前对每个 physical_domain 做越界检查，越界整批拒绝
#   - 写后 fresh readback 验证 normalized 误差 ≤ 1e-4
#   - 枚举类型验证 display label 精确匹配
#   - 发现意外参数变化（unplanned_parameter_change）时整批 rollback
#   - rollback 后再次 readback 确认恢复，恢复失败明确报告不谎称已恢复

# ══════════════════════════════════════════════════════════════════════════════
# 非必填：备注
# ══════════════════════════════════════════════════════════════════════════════

vps_notes:
  - "Q10 共10个结构相同的参数化频段，所有域参数完全相同。"
  - "Type 枚举有6个值（含 PQ-Bell），本 VPS 只映射5个标准公开 shape。"
  - "Hi-Shelf 的 normalized 值（约0.4）需在目标实例上用 value-to-string 实测确认。"
  - "参数 ID 0-49 对应10个频段，50-56 为全局路由参数（不进入 EQ 控制路径）。"
  - "已通过真实烟测：3396Hz / -3.0dB / Q=0.5，modify / disable / undo 全部通过。"

generation_source: "structural_recognizer + live_smoke_test"
# structural_recognizer：由结构识别器从参数面自动生成
# agent_probe          ：agent 对不确定字段做了定向探测补充
# manual               ：人工填写
# 本文件为混合来源：识别器自动生成 + 烟测验证
