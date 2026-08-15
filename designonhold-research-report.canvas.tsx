import {
  Callout,
  CollapsibleSection,
  Divider,
  Grid,
  H1,
  H2,
  Stack,
  Stat,
  Table,
  Tag,
  Text,
  Timeline,
} from 'qoder/canvas';

export default function DesignOnHoldResearchReport() {
  return (
    <Stack gap={20}>
      <H1>DesignOnHold 设计研究完成报告</H1>
      <Text tone="secondary">
        研究对象：「订阅地址池 / 装配拼接 / 配置生成与分发 / Xray 对接」整体设计及其与项目实际状态的衔接 ·
        纯研究分析任务（未修改任何文件、未提问）· 2026-08-15
      </Text>

      <Grid columns={5} gap={12}>
        <Stat value="10" label="研究角度" />
        <Stat value="5" label="冲突 / 缺口" tone="warning" />
        <Stat value="10" label="待决点（Q1~Q10）" />
        <Stat value="3" label="构建方案并列呈现" />
        <Stat value="0" label="文件修改" tone="success" />
      </Grid>

      <Divider />

      <H2>成就摘要</H2>
      <Text>
        以十个研究角度对 DesignOnHold.md 四版块设计进行系统性评估：架构与职责分层、模块耦合与依赖、
        数据模型一致性、AGENTS.md 符合性、基础/高级模式兼容性、Design1.md 基线对齐、构建衔接、
        前端架构、运行时性能、测试性与迁移成本。每个疑点均代入多个候选方案进行多轮推演对比后收敛，
        最终输出 10 项待决点推荐取值与 3 个并列构建方案（含适用前提、优势、取舍与推荐理由）。
      </Text>

      <H2>关键步骤</H2>
      <Timeline
        events={[
          {
            title: '取证：代码扩展点核验',
            description:
              '确认 ContentProvider 预留、CreateVersion 强制激活行为、SwitchVersion 存在、download.go 对 group_selections 的耦合、cron ticker 模式、前端预览 Modal 形态',
            tone: 'info',
          },
          {
            title: '角度一~三：架构 / 耦合 / 数据模型',
            description:
              '发现冲突 D1（节点统一表 vs xray_inbounds）、D2（代理组持久化缺失）、D3（无蓝图模板注入规则缺口）；收敛渲染器纯函数化方案',
            tone: 'warning',
          },
          {
            title: '角度四~七：版本语义 / AGENTS / 分层 / 基线',
            description:
              '发现冲突 C1（入池+显式分发 vs 强制激活）与安全缺口（条目注入面校验、手动节点 UUID 加密、池同步并发互斥）；梳理 Design1 基线脱节清单',
            tone: 'warning',
          },
          {
            title: '角度八~十：前端 / 性能 / 测试迁移',
            description:
              'diff 预览推荐自研零依赖实现；50MB 拉取流式逐行解析；Phase 1 破坏性改造需拆 2~3 Step 以匹配测试重写工作量',
            tone: 'info',
          },
          {
            title: '收敛：待决清单 + 构建方案',
            description:
              'Q1~Q10 全部给出推荐取值；方案 A（三段递进单 Build4，推荐）、方案 B（Build4/5 分册）、方案 C（装配先行，不推荐）并列呈现',
            tone: 'success',
          },
        ]}
      />

      <H2>冲突与待决点（含推演推荐）</H2>
      <Table
        headers={['编号', '疑点 / 冲突', '推荐收敛']}
        rows={[
          ['D1/Q1', '节点统一表 vs 5.9 保留 xray_inbounds（文档内部矛盾）', 'xray_inbounds 升级为统一 nodes 表（+source/uuid_encrypted）'],
          ['D2/Q2', '代理组持久化缺失（设计空白）', '全局 proxy_groups 表 + 预置库种子数据'],
          ['D3/Q5', '无蓝图（上传）模板的注入规则缺口', '按组分配 ∪ 公共节点注入，无候选集约束'],
          ['C1/Q3', '入池+显式分发 vs CreateVersion 强制激活', 'CreateVersion 增加 activate opt-in 参数（仅订阅传 false）'],
          ['Q4', '首次入池空窗（无激活版本不可下载）', '首次入池自动激活（产品语义可再议）'],
          ['Q6', '高级开关 OFF 时占位标记行为未定义', '原样返回占位注释（语法无害）'],
          ['Q7', '素材条目注入面（逗号/换行可伪造规则行）', '按类型白名单校验，非法行跳过记因'],
          ['Q8', '手动节点 UUID 凭据存储', 'AES-256-GCM 加密（复用签名密钥派生）'],
          ['Q9', '装配预览 diff 实现方式', '自研朴素行级 diff（零新依赖）'],
          ['Q10', 'Design1-UI 缺新页面 GUI 规格', 'Build 编写前补齐或以 DesignOnHold 为准（需拍板）'],
        ]}
        rowTone={['danger', 'danger', 'warning', 'danger', undefined, undefined, 'warning', 'warning', undefined, undefined]}
      />

      <H2>构建衔接方案对比</H2>
      <Table
        headers={['方案', '结构', '适用前提', '优势 / 取舍', '结论']}
        rows={[
          [
            'A',
            '三段递进单 Build4：①订阅地址池化 ②基础模式装配 ③高级模式',
            '接受存量清空重建；Phase 1 先行',
            '每 Phase 独立验收、无中间态悬空；Phase 1 为纯破坏性改造需拆 2~3 Step',
            '推荐',
          ],
          [
            'B',
            'Build4（基础）/ Build5（高级）分册',
            '希望隔离 xray-core 依赖风险',
            '基础模式可交付里程碑清晰；多一次文档闭环开销',
            '备选',
          ],
          [
            'C',
            '装配引擎先行（渲染器早于池改造）',
            '希望尽早验证双渲染器',
            '产物无落地容器，违反 Step 完整验收惯例',
            '不推荐',
          ],
        ]}
        rowTone={['success', undefined, 'danger']}
      />

      <CollapsibleSection title="调查文件与验证证据" defaultOpen={false}>
        <Stack gap={12}>
          <Table
            headers={['关键论断', '证据来源', '状态']}
            rows={[
              ['CreateVersion 事务内强制切换激活指针', 'backend/internal/version/version.go L121-176（setCurrentLocked 调用）', '已核实'],
              ['SwitchVersion 可支撑显式分发端点', 'backend/internal/version/version.go L216', '已核实'],
              ['下载链路耦合 group_selections（两处 JOIN + 显式预览分支）', 'backend/internal/download/download.go L96-192', '已核实'],
              ['ContentProvider 注释预留「装配生成」第三实现', 'backend/internal/version/version.go L67-71', '已核实'],
              ['Build3 六 Step 全部验收通过；候选 #1 仍为旧装配器口径', 'Build3.md §一/§五', '已核实'],
              ['Design1 §3.9/§5.3/§九 仍按旧设计描述', 'Design1.md L465/L782/L921', '已核实'],
              ['cron 仅 ticker 模式（访问日志清理）', 'backend/internal/cron/cleanup.go L12', '已核实'],
              ['前端无 diff/编辑器库，预览为纯文本 Modal', 'frontend/src/views/admin/VersionManageView.vue', '已核实'],
            ]}
          />
          <Text tone="secondary" size="small">
            约束遵守：全程未执行文件写操作（仅 Read/Grep/search 取证）；未调用提问工具；方案 A/B/C 并列呈现未过早锁定单一结论。
          </Text>
        </Stack>
      </CollapsibleSection>

      <Callout tone="info" title="最终结论">
        四版块设计方向自洽，与既有代码扩展点高度吻合；风险集中于 5 个文档内部冲突/缺口（均已给出候选推演与推荐解）。
        启动前置条件：① 确认 Q1~Q10 取值；② 一次性修订 DesignOnHold（D1~D3 + C1 + 安全缺口）；
        ③ 清理文档同步债务（Design1 §3.9/§5.3/§九、Build3 候选 #1、AGENTS §8.4）。推荐采用方案 A 三段递进构建。
      </Callout>

      <Text tone="secondary" size="small">
        <Tag tone="neutral">纯研究任务</Tag> 无文件变更 · 无截图产物 · 报告基于 worktree 当前状态取证
      </Text>
    </Stack>
  );
}
