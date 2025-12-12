// Copyright 2018 The agentx authors
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package agentx_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zoneBen/go-agentx"
	"github.com/zoneBen/go-agentx/pdu"
	"github.com/zoneBen/go-agentx/value"
)

func TestTableHandler_BasicOperations(t *testing.T) {
	// 创建表处理器，baseOID 为 1.3.6.1.4.1.45995.10.1 (类似 ifTable.ifEntry)
	baseOID := value.MustParseOID("1.3.6.1.4.1.45995.10.1")
	th := agentx.NewTableHandler(baseOID)

	// 添加行数据
	// 行1: index=1
	row1 := agentx.NewTableRow(1)
	row1.Set(1, pdu.VariableTypeInteger, int32(1))       // 列1: index
	row1.Set(2, pdu.VariableTypeOctetString, "eth0")     // 列2: name
	row1.Set(3, pdu.VariableTypeCounter32, uint32(1000)) // 列3: counter
	th.AddRow(row1)

	// 行2: index=2
	row2 := agentx.NewTableRow(2)
	row2.Set(1, pdu.VariableTypeInteger, int32(2))
	row2.Set(2, pdu.VariableTypeOctetString, "eth1")
	row2.Set(3, pdu.VariableTypeCounter32, uint32(2000))
	th.AddRow(row2)

	// 行3: index=3
	row3 := agentx.NewTableRow(3)
	row3.Set(1, pdu.VariableTypeInteger, int32(3))
	row3.Set(2, pdu.VariableTypeOctetString, "lo")
	row3.Set(3, pdu.VariableTypeCounter32, uint32(500))
	th.AddRow(row3)

	assert.Equal(t, 3, th.RowCount())

	// 测试 GetRow
	r := th.GetRow(2)
	require.NotNil(t, r)
	cell, ok := r.Get(2)
	require.True(t, ok)
	assert.Equal(t, "eth1", cell.Value)

	// 测试 RemoveRow
	removed := th.RemoveRow(2)
	assert.True(t, removed)
	assert.Equal(t, 2, th.RowCount())
	assert.Nil(t, th.GetRow(2))
}

func TestTableHandler_Get(t *testing.T) {
	baseOID := value.MustParseOID("1.3.6.1.4.1.45995.10.1")
	th := agentx.NewTableHandler(baseOID)

	// 添加测试数据
	row1 := agentx.NewTableRow(1)
	row1.Set(1, pdu.VariableTypeInteger, int32(1))
	row1.Set(2, pdu.VariableTypeOctetString, "eth0")
	th.AddRow(row1)

	row2 := agentx.NewTableRow(2)
	row2.Set(1, pdu.VariableTypeInteger, int32(2))
	row2.Set(2, pdu.VariableTypeOctetString, "eth1")
	th.AddRow(row2)

	// 测试 Get - 存在的 OID
	// OID: 1.3.6.1.4.1.45995.10.1.2.1 = baseOID.column.rowIndex
	oid := value.MustParseOID("1.3.6.1.4.1.45995.10.1.2.1")
	resultOID, varType, val, err := th.Get(oid)
	require.NoError(t, err)
	assert.Equal(t, oid.String(), resultOID.String())
	assert.Equal(t, pdu.VariableTypeOctetString, varType)
	assert.Equal(t, "eth0", val)

	// 测试 Get - 不存在的行
	oid = value.MustParseOID("1.3.6.1.4.1.45995.10.1.2.99")
	resultOID, varType, _, err = th.Get(oid)
	require.NoError(t, err)
	assert.Nil(t, resultOID)
	assert.Equal(t, pdu.VariableTypeNoSuchInstance, varType)

	// 测试 Get - 不存在的列
	oid = value.MustParseOID("1.3.6.1.4.1.45995.10.1.99.1")
	resultOID, varType, _, err = th.Get(oid)
	require.NoError(t, err)
	assert.Nil(t, resultOID)
	assert.Equal(t, pdu.VariableTypeNoSuchInstance, varType)
}

func TestTableHandler_GetNext(t *testing.T) {
	baseOID := value.MustParseOID("1.3.6.1.4.1.45995.10.1")
	th := agentx.NewTableHandler(baseOID)

	// 添加测试数据
	row1 := agentx.NewTableRow(1)
	row1.Set(1, pdu.VariableTypeInteger, int32(1))
	row1.Set(2, pdu.VariableTypeOctetString, "eth0")
	th.AddRow(row1)

	row2 := agentx.NewTableRow(2)
	row2.Set(1, pdu.VariableTypeInteger, int32(2))
	row2.Set(2, pdu.VariableTypeOctetString, "eth1")
	th.AddRow(row2)

	// SNMP 表遍历顺序（列优先）:
	// 1.3.6.1.4.1.45995.10.1.1.1 (col=1, row=1)
	// 1.3.6.1.4.1.45995.10.1.1.2 (col=1, row=2)
	// 1.3.6.1.4.1.45995.10.1.2.1 (col=2, row=1)
	// 1.3.6.1.4.1.45995.10.1.2.2 (col=2, row=2)

	toOID := value.MustParseOID("1.3.6.1.4.1.45995.11") // 范围结束

	t.Run("从baseOID开始", func(t *testing.T) {
		from := value.MustParseOID("1.3.6.1.4.1.45995.10.1")
		resultOID, varType, val, err := th.GetNext(from, false, toOID)
		require.NoError(t, err)
		assert.Equal(t, "1.3.6.1.4.1.45995.10.1.1.1", resultOID.String())
		assert.Equal(t, pdu.VariableTypeInteger, varType)
		assert.Equal(t, int32(1), val)
	})

	t.Run("从第一个元素获取下一个", func(t *testing.T) {
		from := value.MustParseOID("1.3.6.1.4.1.45995.10.1.1.1")
		resultOID, varType, val, err := th.GetNext(from, false, toOID)
		require.NoError(t, err)
		assert.Equal(t, "1.3.6.1.4.1.45995.10.1.1.2", resultOID.String())
		assert.Equal(t, pdu.VariableTypeInteger, varType)
		assert.Equal(t, int32(2), val)
	})

	t.Run("列内最后一行的下一个应该是下一列的第一行", func(t *testing.T) {
		from := value.MustParseOID("1.3.6.1.4.1.45995.10.1.1.2")
		resultOID, varType, val, err := th.GetNext(from, false, toOID)
		require.NoError(t, err)
		assert.Equal(t, "1.3.6.1.4.1.45995.10.1.2.1", resultOID.String())
		assert.Equal(t, pdu.VariableTypeOctetString, varType)
		assert.Equal(t, "eth0", val)
	})

	t.Run("表末尾返回EndOfMIBView", func(t *testing.T) {
		from := value.MustParseOID("1.3.6.1.4.1.45995.10.1.2.2")
		resultOID, varType, _, err := th.GetNext(from, false, toOID)
		require.NoError(t, err)
		assert.Nil(t, resultOID)
		assert.Equal(t, pdu.VariableTypeEndOfMIBView, varType)
	})

	t.Run("includeFrom为true时包含当前OID", func(t *testing.T) {
		from := value.MustParseOID("1.3.6.1.4.1.45995.10.1.1.1")
		resultOID, _, val, err := th.GetNext(from, true, toOID)
		require.NoError(t, err)
		assert.Equal(t, "1.3.6.1.4.1.45995.10.1.1.1", resultOID.String())
		assert.Equal(t, int32(1), val)
	})
}

func TestTableHandler_MultiIndexRow(t *testing.T) {
	// 测试多索引的表（复合索引）
	baseOID := value.MustParseOID("1.3.6.1.4.1.45995.20.1")
	th := agentx.NewTableHandler(baseOID)

	// 使用复合索引 (ifIndex, ipAddress)
	// 比如: 1.10.0.0.1 表示接口1上的IP 10.0.0.1
	row1 := agentx.NewTableRow(1, 10, 0, 0, 1)
	row1.Set(1, pdu.VariableTypeInteger, int32(1))
	row1.Set(2, pdu.VariableTypeOctetString, "Primary")
	th.AddRow(row1)

	row2 := agentx.NewTableRow(1, 10, 0, 0, 2)
	row2.Set(1, pdu.VariableTypeInteger, int32(1))
	row2.Set(2, pdu.VariableTypeOctetString, "Secondary")
	th.AddRow(row2)

	row3 := agentx.NewTableRow(2, 192, 168, 1, 1)
	row3.Set(1, pdu.VariableTypeInteger, int32(2))
	row3.Set(2, pdu.VariableTypeOctetString, "Gateway")
	th.AddRow(row3)

	assert.Equal(t, 3, th.RowCount())

	// 测试 Get
	oid := value.MustParseOID("1.3.6.1.4.1.45995.20.1.2.1.10.0.0.2")
	resultOID, varType, val, err := th.Get(oid)
	require.NoError(t, err)
	assert.Equal(t, oid.String(), resultOID.String())
	assert.Equal(t, pdu.VariableTypeOctetString, varType)
	assert.Equal(t, "Secondary", val)

	// 测试 GetNext 遍历
	toOID := value.MustParseOID("1.3.6.1.4.1.45995.21")
	from := value.MustParseOID("1.3.6.1.4.1.45995.20.1")

	// 应该返回第一列第一行
	resultOID, _, val, _ = th.GetNext(from, false, toOID)
	assert.Equal(t, "1.3.6.1.4.1.45995.20.1.1.1.10.0.0.1", resultOID.String())
	assert.Equal(t, int32(1), val)
}

func TestTableHandler_EmptyTable(t *testing.T) {
	baseOID := value.MustParseOID("1.3.6.1.4.1.45995.30.1")
	th := agentx.NewTableHandler(baseOID)

	// 空表的 Get 应该返回 NoSuchObject
	oid := value.MustParseOID("1.3.6.1.4.1.45995.30.1.1.1")
	resultOID, varType, _, err := th.Get(oid)
	require.NoError(t, err)
	assert.Nil(t, resultOID)
	assert.Equal(t, pdu.VariableTypeNoSuchObject, varType)

	// 空表的 GetNext 应该返回 EndOfMIBView
	toOID := value.MustParseOID("1.3.6.1.4.1.45995.31")
	resultOID, varType, _, err = th.GetNext(oid, false, toOID)
	require.NoError(t, err)
	assert.Nil(t, resultOID)
	assert.Equal(t, pdu.VariableTypeEndOfMIBView, varType)
}
