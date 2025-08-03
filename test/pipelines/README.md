# Pipeline Test Cases

这个目录包含了多种类型的流水线YAML定义文件，用于测试不同的流水线场景。

## 文件说明

### 1. simple-linear.yaml
**简单线性流水线**
- 场景：基础的构建 -> 测试 -> 部署流程
- 特点：线性依赖，适合简单项目
- 节点数：3个
- 依赖关系：build → test → deploy

### 2. parallel-tasks.yaml
**并行任务流水线**
- 场景：构建后并行执行多种测试
- 特点：并行执行，提高效率
- 节点数：5个
- 依赖关系：build → (unit-test, integration-test, security-scan) → deploy

### 3. complex-dag.yaml
**复杂DAG流水线**
- 场景：企业级多阶段部署流程
- 特点：包含审批节点、多环境部署
- 节点数：10个
- 依赖关系：复杂的有向无环图

### 4. microservices.yaml
**微服务部署流水线**
- 场景：多个微服务并行构建和部署
- 特点：服务间并行处理
- 节点数：10个
- 依赖关系：多个并行分支最终汇聚

### 5. conditional-flow.yaml
**条件流水线**
- 场景：根据条件执行不同路径
- 特点：包含条件判断和分支
- 节点数：7个
- 依赖关系：基于条件的动态路径

### 6. matrix-build.yaml
**矩阵构建流水线**
- 场景：多版本、多平台并行构建
- 特点：大量并行任务
- 节点数：11个
- 依赖关系：扇出后汇聚的模式

## 测试用途

这些YAML文件可以用于：

1. **功能测试**：验证不同类型的流水线是否能正确解析和执行
2. **性能测试**：测试并行任务的处理能力
3. **边界测试**：测试复杂依赖关系的处理
4. **集成测试**：验证完整的流水线生命周期管理

## 使用方法

```bash
# 解析YAML文件
go test -v ./test -run TestParseYAML

# 执行特定流水线
go test -v ./test -run TestExecutePipeline -args simple-linear.yaml

# 批量测试所有流水线
go test -v ./test -run TestAllPipelines
```

## 扩展

可以根据需要添加更多测试场景：
- 错误处理流水线
- 超时和重试机制
- 资源限制测试
- 权限和安全测试