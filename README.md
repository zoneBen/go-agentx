# go-agentx

[![Go Reference](https://pkg.go.dev/badge/github.com/zoneBen/go-agentx.svg)](https://pkg.go.dev/github.com/zoneBen/go-agentx)
[![License](https://img.shields.io/badge/license-LGPLv3-blue.svg)](LICENSE)

纯 Go 实现的 [AgentX 协议](http://tools.ietf.org/html/rfc2741) 库，用于扩展 SNMP 守护进程，将 OID 子树的请求分发到你的 Go 应用程序。

> 本项目 fork 自 [posteo/go-agentx](https://github.com/posteo/go-agentx)，在其基础上增加了 SNMP Table 支持等功能。

## 功能特性

- 完整支持所有变量类型：Integer、OctetString、Null、ObjectIdentifier、IPAddress、Counter32、Gauge32、TimeTicks、Opaque、Counter64、NoSuchObject、NoSuchInstance、EndOfMIBView
- 支持 Get、GetNext、GetBulk 请求
- 支持 SNMP Table（表格）处理 — `TableHandler`
- 断线自动重连，自动恢复会话和注册
- 线程安全，并发安全

## 安装

```bash
go get github.com/zoneBen/go-agentx
```

## 快速开始

### 基础示例：ListHandler

使用 `ListHandler` 提供一组静态 OID 值：

```go
package main

import (
    "log"
    "net"
    "time"

    "github.com/zoneBen/go-agentx"
    "github.com/zoneBen/go-agentx/pdu"
    "github.com/zoneBen/go-agentx/value"
)

func main() {
    // 连接到 SNMP 守护进程的 AgentX 端口
    client, err := agentx.Dial("tcp", "localhost:705")
    if err != nil {
        log.Fatal(err)
    }
    client.Timeout = 1 * time.Minute
    client.ReconnectInterval = 1 * time.Second

    // 创建会话
    session, err := client.Session()
    if err != nil {
        log.Fatal(err)
    }

    // 使用 ListHandler 提供一组 OID 值
    listHandler := &agentx.ListHandler{}

    // Integer 类型
    item := listHandler.Add("1.3.6.1.4.1.45995.3.1")
    item.Type = pdu.VariableTypeInteger
    item.Value = int32(-123)

    // OctetString 类型
    item = listHandler.Add("1.3.6.1.4.1.45995.3.2")
    item.Type = pdu.VariableTypeOctetString
    item.Value = "hello agentx"

    // Counter32 类型
    item = listHandler.Add("1.3.6.1.4.1.45995.3.3")
    item.Type = pdu.VariableTypeCounter32
    item.Value = uint32(123)

    // Gauge32 类型
    item = listHandler.Add("1.3.6.1.4.1.45995.3.4")
    item.Type = pdu.VariableTypeGauge32
    item.Value = uint32(456)

    // IPAddress 类型
    item = listHandler.Add("1.3.6.1.4.1.45995.3.5")
    item.Type = pdu.VariableTypeIPAddress
    item.Value = net.IP{10, 0, 0, 1}

    // TimeTicks 类型
    item = listHandler.Add("1.3.6.1.4.1.45995.3.6")
    item.Type = pdu.VariableTypeTimeTicks
    item.Value = 123 * time.Second

    // Counter64 类型
    item = listHandler.Add("1.3.6.1.4.1.45995.3.7")
    item.Type = pdu.VariableTypeCounter64
    item.Value = uint64(12345678901234567890)

    session.Handler = listHandler

    // 注册 OID 子树（优先级 127）
    if err := session.Register(127, value.MustParseOID("1.3.6.1.4.1.45995.3")); err != nil {
        log.Fatal(err)
    }

    select {} // 保持运行
}
```

### 表格示例：TableHandler

使用 `TableHandler` 提供 SNMP 表格数据，自动处理 GetNext / GetBulk 遍历：

```go
package main

import (
    "log"
    "time"

    "github.com/zoneBen/go-agentx"
    "github.com/zoneBen/go-agentx/pdu"
    "github.com/zoneBen/go-agentx/value"
)

func main() {
    client, err := agentx.Dial("tcp", "localhost:705")
    if err != nil {
        log.Fatal(err)
    }
    client.Timeout = 1 * time.Minute
    client.ReconnectInterval = 1 * time.Second

    session, err := client.Session()
    if err != nil {
        log.Fatal(err)
    }

    // 创建表格处理器，baseOID 到 entry 级别
    // 例如 ifTable 的 entry OID 是 1.3.6.1.2.1.2.2.1
    tableOID := value.MustParseOID("1.3.6.1.4.1.45995.4.1")
    tableHandler := agentx.NewTableHandler(tableOID)

    // 添加第一行（索引 = 1）
    row1 := agentx.NewTableRow(1)
    row1.Set(1, pdu.VariableTypeInteger, int32(1))       // 列1: ifIndex
    row1.Set(2, pdu.VariableTypeOctetString, "eth0")    // 列2: ifDescr
    row1.Set(3, pdu.VariableTypeCounter32, uint32(1000)) // 列3: ifInOctets
    row1.Set(4, pdu.VariableTypeCounter32, uint32(2000)) // 列4: ifOutOctets
    tableHandler.AddRow(row1)

    // 添加第二行（索引 = 2）
    row2 := agentx.NewTableRow(2)
    row2.Set(1, pdu.VariableTypeInteger, int32(2))
    row2.Set(2, pdu.VariableTypeOctetString, "eth1")
    row2.Set(3, pdu.VariableTypeCounter32, uint32(5000))
    row2.Set(4, pdu.VariableTypeCounter32, uint32(6000))
    tableHandler.AddRow(row2)

    session.Handler = tableHandler

    // 注册表格的 OID 子树
    if err := session.Register(127, value.MustParseOID("1.3.6.1.4.1.45995.4")); err != nil {
        log.Fatal(err)
    }

    // 运行时动态更新表格数据
    go func() {
        ticker := time.NewTicker(5 * time.Second)
        defer ticker.Stop()
        for range ticker.C {
            row := tableHandler.GetRow(1)
            if row != nil {
                row.Set(3, pdu.VariableTypeCounter32, uint32(time.Now().Unix()%10000))
            }
        }
    }()

    select {}
}
```

### 自定义 Handler

实现 `agentx.Handler` 接口以提供自定义的 OID 处理逻辑：

```go
type Handler interface {
    Get(oid value.OID) (value.OID, pdu.VariableType, interface{}, error)
    GetNext(from value.OID, includeFrom bool, to value.OID) (value.OID, pdu.VariableType, interface{}, error)
}
```

## 断线重连

如果与 SNMP 守护进程的连接断开，客户端会自动重连。通过设置 `ReconnectInterval` 指定重连间隔：

```go
client.ReconnectInterval = 5 * time.Second
```

重连成功后，客户端会自动恢复之前打开的会话和注册的 OID 子树。

## 变量类型

| 类型 | Go 类型 | 说明 |
|------|---------|------|
| `VariableTypeInteger` | `int32` | 32 位整数 |
| `VariableTypeOctetString` | `string` | 字符串 |
| `VariableTypeNull` | `nil` | 空值 |
| `VariableTypeObjectIdentifier` | `string` | OID 字符串 |
| `VariableTypeIPAddress` | `net.IP` | IP 地址 |
| `VariableTypeCounter32` | `uint32` | 32 位计数器 |
| `VariableTypeGauge32` | `uint32` | 32 位标尺 |
| `VariableTypeTimeTicks` | `time.Duration` | 时间戳（百分之一秒） |
| `VariableTypeOpaque` | `[]byte` | 不透明数据 |
| `VariableTypeCounter64` | `uint64` | 64 位计数器 |

## TableHandler API

| 方法 | 说明 |
|------|------|
| `NewTableHandler(baseOID value.OID)` | 创建表格处理器，baseOID 包含 entry 级别 |
| `AddRow(row *TableRow)` | 添加一行数据 |
| `RemoveRow(index ...uint32) bool` | 按索引移除一行 |
| `GetRow(index ...uint32) *TableRow` | 按索引获取一行 |
| `RowCount() int` | 返回表格行数 |
| `NewTableRow(index ...uint32)` | 创建新行，支持复合索引 |
| `row.Set(column, type, value)` | 设置行中某列的值 |
| `row.Get(column)` | 获取行中某列的值 |

## 协议状态

已实现：
- 所有 PDU 变量类型编解码
- Get、GetNext、GetBulk 请求处理
- 会话管理与注册
- 断线重连与会话恢复
- 表格（Table）处理

未实现：
- Set 请求
- Trap/Notification

## License

本项目基于 LGPLv3 许可证开源，详见 [LICENSE](LICENSE) 文件。
