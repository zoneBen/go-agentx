// Copyright 2018 The agentx authors
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package agentx

import "github.com/zoneBen/go-agentx/pdu"

// TableRow 定义表格中的一行数据
// Index 是行索引（可以是单个值或多个值组成的复合索引）
// Values 是该行各列的值，key 为列号
type TableRow struct {
	Index  []uint32
	Values map[uint32]*TableCell
}

// TableCell 定义表格单元格
type TableCell struct {
	Type  pdu.VariableType
	Value interface{}
}

// NewTableRow 创建一个新的表行
func NewTableRow(index ...uint32) *TableRow {
	return &TableRow{
		Index:  index,
		Values: make(map[uint32]*TableCell),
	}
}

// Set 设置指定列的值
func (r *TableRow) Set(column uint32, varType pdu.VariableType, value interface{}) *TableRow {
	r.Values[column] = &TableCell{
		Type:  varType,
		Value: value,
	}
	return r
}

// Get 获取指定列的值
func (r *TableRow) Get(column uint32) (*TableCell, bool) {
	cell, ok := r.Values[column]
	return cell, ok
}
