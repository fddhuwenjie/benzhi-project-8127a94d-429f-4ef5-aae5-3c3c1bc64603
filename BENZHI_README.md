# BENZHI_README

## 项目说明
- 项目：benzhi-project-8127a94d-429f-4ef5-aae5-3c3c1bc64603
- 项目用途：面向地方档案馆的口述史公开版授权封存台，完整实现录音与授权基线冻结、片段约束、确定性冲突检查、遮蔽整改、独立退回复核、不可变清单封存和完整性验证。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 项目描述
- 项目名称：口述史公开版授权封存台
- 项目介绍：面向地方档案馆的口述史公开版治理应用，把原始录音登记、知情授权约束、片段级披露审查、必要遮蔽、独立复核和公开清单封存收束为一条可追溯流程，避免未经授权的人名、地点或敏感叙述进入公开版本。
- 项目概述：面向地方档案馆的口述史公开版治理应用，把原始录音登记、知情授权约束、片段级披露审查、必要遮蔽、独立复核和公开清单封存收束为一条可追溯流程，避免未经授权的人名、地点或敏感叙述进入公开版本。
- 核心工作流：档案员建立口述史案件并冻结录音与授权基线，逐段登记文字稿和披露约束，运行确定性冲突检查后完成遮蔽与证据说明；系统生成候选公开版供不同人员独立复核，退回项修订后再次送审，批准时封存不可变公开清单并提供完整性校验。
- 对外接口：Go 服务直接提供无需 Node 构建的原生单页浏览器工作台及仅供该页面使用的同源 JSON 接口；页面包含案件状态栏、片段与授权约束表格、冲突处理区、公开版预览、复核面板和封存校验视图。服务支持 -addr=127.0.0.1:<port>，也支持以 PORT 端口号绑定 127.0.0.1:<PORT>，默认监听 127.0.0.1:19081，且不得默认绑定 8080、80、3000 或 0.0.0.0。

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...

cd /app && GOTOOLCHAIN=local go run ./cmd/oralhistory -self-check -addr=127.0.0.1:19081

cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh

./build_benzhi_docker.sh benzhi-project-8127a94d-429f-4ef5-aae5-3c3c1bc64603-amd64 linux/amd64

./build_benzhi_docker.sh benzhi-project-8127a94d-429f-4ef5-aae5-3c3c1bc64603-arm64 linux/arm64

docker run -it benzhi-project-8127a94d-429f-4ef5-aae5-3c3c1bc64603-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/oralhistory -self-check -addr=127.0.0.1:19081`
