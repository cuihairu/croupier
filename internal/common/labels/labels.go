// Package labels 定义跨端共享的 agent 注册标签键。标签由 agent 在注册时
// 自报（RegisterRequest.labels），server 经 sanitizeAgentLabels 收敛后持久
// 化进 agent_sessions.labels；任何读取同一标签的端必须用这里的常量，
// 禁止散落裸字符串（键名漂移=静默失配）。
package labels

// AgentBinarySHA256 是 agent 运行二进制内容 sha256（hex 小写）的注册标签键。
// §10.5 指纹持久锚点：agent 每次注册自报（运行实例指纹，进程生命周期缓存
// 一次），随注册标签持久化——system-info 数据面在 agent 失联或跨 server
// 重启后靠它回答「最后已知指纹」。
const AgentBinarySHA256 = "agentBinarySha256"
