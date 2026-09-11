// Copyright 2018 The agentx authors
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package agentx

import (
	"sort"
	"sync"

	"github.com/zoneBen/go-agentx/pdu"
	"github.com/zoneBen/go-agentx/value"
)

// TableHandler 实现了 SNMP 表格的处理器
// SNMP 表格的 OID 结构: baseOID.entry.column.rowIndex...
// 例如: 1.3.6.1.2.1.2.2.1.2.1 表示 ifTable.ifEntry.ifDescr.index=1
type TableHandler struct {
	mu      sync.RWMutex
	baseOID value.OID // 表的基础 OID (包含 entry，如 1.3.6.1.2.1.2.2.1)
	rows    []*TableRow
	columns []uint32 // 已排序的列号列表
}

// NewTableHandler 创建一个新的表格处理器
// baseOID 应该包含到 entry 级别，如 "1.3.6.1.2.1.2.2.1" (ifTable.ifEntry)
func NewTableHandler(baseOID value.OID) *TableHandler {
	return &TableHandler{
		baseOID: baseOID,
		rows:    make([]*TableRow, 0),
		columns: make([]uint32, 0),
	}
}

// AddRow 添加一行数据到表格
func (t *TableHandler) AddRow(row *TableRow) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.rows = append(t.rows, row)
	// 按索引排序
	t.sortRows()

	// 更新列列表
	for col := range row.Values {
		if !t.hasColumn(col) {
			t.columns = append(t.columns, col)
			t.sortColumns()
		}
	}
}

// RemoveRow 根据索引移除一行
func (t *TableHandler) RemoveRow(index ...uint32) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	for i, row := range t.rows {
		if t.indexEquals(row.Index, index) {
			t.rows = append(t.rows[:i], t.rows[i+1:]...)
			return true
		}
	}
	return false
}

// GetRow 根据索引获取一行
func (t *TableHandler) GetRow(index ...uint32) *TableRow {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, row := range t.rows {
		if t.indexEquals(row.Index, index) {
			return row
		}
	}
	return nil
}

// RowCount 返回表格行数
func (t *TableHandler) RowCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.rows)
}

// Get 实现 Handler 接口，根据 OID 获取值
func (t *TableHandler) Get(oid value.OID) (value.OID, pdu.VariableType, interface{}, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// 解析 OID，提取列号和行索引
	column, rowIndex, ok := t.parseOID(oid)
	if !ok {
		return nil, pdu.VariableTypeNoSuchObject, nil, nil
	}

	// 查找行
	row := t.findRow(rowIndex)
	if row == nil {
		return nil, pdu.VariableTypeNoSuchInstance, nil, nil
	}

	// 获取单元格值
	cell, ok := row.Get(column)
	if !ok {
		return nil, pdu.VariableTypeNoSuchInstance, nil, nil
	}

	return oid, cell.Type, cell.Value, nil
}

// GetNext 实现 Handler 接口，获取下一个 OID 的值
func (t *TableHandler) GetNext(from value.OID, includeFrom bool, to value.OID) (value.OID, pdu.VariableType, interface{}, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.rows) == 0 || len(t.columns) == 0 {
		return nil, pdu.VariableTypeEndOfMIBView, nil, nil
	}

	// 判断 from 是否在表的范围内
	baseLen := len(t.baseOID)

	// 如果 from 比 baseOID 短或者不是 baseOID 的子 OID
	if len(from) < baseLen {
		commonPrefix := from.CommonPrefix(t.baseOID)
		if len(commonPrefix) < len(from) {
			// from 不是 baseOID 的前缀，需要比较大小
			if value.CompareOIDs(from, t.baseOID) >= 0 {
				return nil, pdu.VariableTypeEndOfMIBView, nil, nil
			}
		}
		// from 是 baseOID 的前缀或者比 baseOID 小，返回第一个元素
		return t.getFirstCell()
	}

	// 检查 from 的 baseOID 部分是否匹配
	if value.CompareOIDs(from[:baseLen], t.baseOID) != 0 {
		if value.CompareOIDs(from, t.baseOID) < 0 {
			return t.getFirstCell()
		}
		return nil, pdu.VariableTypeEndOfMIBView, nil, nil
	}

	// 解析列和索引
	column, rowIndex, ok := t.parseOID(from)
	if !ok {
		// 如果只有 baseOID 或无法解析，返回第一个元素
		if includeFrom {
			return t.getFirstCell()
		}
		return t.getFirstCell()
	}

	// 查找下一个有效的单元格
	return t.findNextCell(column, rowIndex, includeFrom)
}

// parseOID 从 OID 中解析出列号和行索引
// OID 格式: baseOID.column.rowIndex...
func (t *TableHandler) parseOID(oid value.OID) (column uint32, rowIndex []uint32, ok bool) {
	baseLen := len(t.baseOID)
	if len(oid) <= baseLen {
		return 0, nil, false
	}

	column = oid[baseLen]
	if len(oid) > baseLen+1 {
		rowIndex = oid[baseLen+1:]
	}
	return column, rowIndex, true
}

// buildOID 根据列号和行索引构建完整的 OID
func (t *TableHandler) buildOID(column uint32, rowIndex []uint32) value.OID {
	result := make(value.OID, len(t.baseOID)+1+len(rowIndex))
	copy(result, t.baseOID)
	result[len(t.baseOID)] = column
	copy(result[len(t.baseOID)+1:], rowIndex)
	return result
}

// findRow 根据索引查找行
func (t *TableHandler) findRow(index []uint32) *TableRow {
	for _, row := range t.rows {
		if t.indexEquals(row.Index, index) {
			return row
		}
	}
	return nil
}

// findNextCell 查找下一个有效的单元格
func (t *TableHandler) findNextCell(column uint32, rowIndex []uint32, includeFrom bool) (value.OID, pdu.VariableType, interface{}, error) {
	// 按 SNMP 表遍历顺序：先按列，再按行
	// 遍历顺序: col1.row1 -> col1.row2 -> col2.row1 -> col2.row2 ...

	for _, col := range t.columns {
		if col < column {
			continue
		}

		for _, row := range t.rows {
			cell, hasCell := row.Get(col)
			if !hasCell {
				continue
			}

			cellOID := t.buildOID(col, row.Index)

			// 比较当前 cell 的 OID 与 from
			cmp := value.CompareOIDs(cellOID, t.buildOID(column, rowIndex))

			if cmp > 0 || (cmp == 0 && includeFrom) {
				return cellOID, cell.Type, cell.Value, nil
			}
		}
	}

	return nil, pdu.VariableTypeEndOfMIBView, nil, nil
}

// getFirstCell 获取表格中的第一个单元格
func (t *TableHandler) getFirstCell() (value.OID, pdu.VariableType, interface{}, error) {
	if len(t.columns) == 0 || len(t.rows) == 0 {
		return nil, pdu.VariableTypeEndOfMIBView, nil, nil
	}

	// 按列优先顺序查找第一个有效单元格
	for _, col := range t.columns {
		for _, row := range t.rows {
			cell, ok := row.Get(col)
			if ok {
				oid := t.buildOID(col, row.Index)
				return oid, cell.Type, cell.Value, nil
			}
		}
	}

	return nil, pdu.VariableTypeEndOfMIBView, nil, nil
}

// sortRows 按索引对行进行排序
func (t *TableHandler) sortRows() {
	sort.Slice(t.rows, func(i, j int) bool {
		return t.compareIndex(t.rows[i].Index, t.rows[j].Index) < 0
	})
}

// sortColumns 对列号进行排序
func (t *TableHandler) sortColumns() {
	sort.Slice(t.columns, func(i, j int) bool {
		return t.columns[i] < t.columns[j]
	})
}

// hasColumn 检查列是否已存在
func (t *TableHandler) hasColumn(col uint32) bool {
	for _, c := range t.columns {
		if c == col {
			return true
		}
	}
	return false
}

// indexEquals 比较两个索引是否相等
func (t *TableHandler) indexEquals(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// compareIndex 比较两个索引的大小
func (t *TableHandler) compareIndex(a, b []uint32) int {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	for i := 0; i < minLen; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}

	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}
