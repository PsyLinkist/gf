// Copyright 2018 gf Author(https://gitee.com/johng/gf). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://gitee.com/johng/gf.

package garray

import (
    "gitee.com/johng/gf/g/container/gtype"
    "gitee.com/johng/gf/g/internal/rwmutex"
    "gitee.com/johng/gf/g/util/gconv"
    "gitee.com/johng/gf/g/util/grand"
    "math"
    "sort"
    "strings"
)

// 默认按照从小到大进行排序
type SortedArray struct {
    mu          *rwmutex.RWMutex             // 互斥锁
    cap         int                          // 初始化设置的数组容量
    array       []interface{}                // 底层数组
    unique      *gtype.Bool                  // 是否要求不能重复
    compareFunc func(v1, v2 interface{}) int // 比较函数，返回值 -1: v1 < v2；0: v1 == v2；1: v1 > v2
}

func NewSortedArray(cap int, compareFunc func(v1, v2 interface{}) int, unsafe...bool) *SortedArray {
    return &SortedArray{
        mu          : rwmutex.New(unsafe...),
        unique      : gtype.NewBool(),
        array       : make([]interface{}, 0, cap),
        compareFunc : compareFunc,
    }
}

func NewSortedArrayEmpty(compareFunc func(v1, v2 interface{}) int, unsafe...bool) *SortedArray {
    return NewSortedArray(0, compareFunc, unsafe...)
}

func NewSortedArrayFrom(array []interface{}, compareFunc func(v1, v2 interface{}) int, unsafe...bool) *SortedArray {
    a := NewSortedArray(0, compareFunc, unsafe...)
    a.array = array
    sort.Slice(a.array, func(i, j int) bool {
        return a.compareFunc(a.array[i], a.array[j]) < 0
    })
    return a
}

// 设置底层数组变量.
func (a *SortedArray) SetArray(array []interface{}) *SortedArray {
    a.mu.Lock()
    defer a.mu.Unlock()
    a.array = array
    sort.Slice(a.array, func(i, j int) bool {
        return a.compareFunc(a.array[i], a.array[j]) < 0
    })
    return a
}

// 将数组重新排序(从小到大).
func (a *SortedArray) Sort(reverse...bool) *SortedArray {
    a.mu.Lock()
    defer a.mu.Unlock()
    sort.Slice(a.array, func(i, j int) bool {
        return a.compareFunc(a.array[i], a.array[j]) < 0
    })
    return a
}

// 添加加数据项
func (a *SortedArray) Add(values...interface{}) *SortedArray {
    if len(values) == 0 {
        return a
    }
    a.mu.Lock()
    defer a.mu.Unlock()
    for _, value := range values {
        index, cmp := a.binSearch(value, false)
        if a.unique.Val() && cmp == 0 {
            continue
        }
        if index < 0 {
            a.array = append(a.array, value)
            continue
        }
        // 加到指定索引后面
        if cmp > 0 {
            index++
        }
        rear   := append([]interface{}{}, a.array[index : ]...)
        a.array = append(a.array[0 : index], value)
        a.array = append(a.array, rear...)
    }
    return a
}

// 获取指定索引的数据项, 调用方注意判断数组边界
func (a *SortedArray) Get(index int) interface{} {
    a.mu.RLock()
    defer a.mu.RUnlock()
    value := a.array[index]
    return value
}

// 删除指定索引的数据项, 调用方注意判断数组边界
func (a *SortedArray) Remove(index int) interface{} {
    a.mu.Lock()
    defer a.mu.Unlock()
    // 边界删除判断，以提高删除效率
    if index == 0 {
        value  := a.array[0]
        a.array = a.array[1 : ]
        return value
    } else if index == len(a.array) - 1 {
        value  := a.array[index]
        a.array = a.array[: index]
        return value
    }
    // 如果非边界删除，会涉及到数组创建，那么删除的效率差一些
    value  := a.array[index]
    a.array = append(a.array[ : index], a.array[index + 1 : ]...)
    return value
}

// 将最左端(索引为0)的数据项移出数组，并返回该数据项
func (a *SortedArray) PopLeft() interface{} {
    a.mu.Lock()
    defer a.mu.Unlock()
    value  := a.array[0]
    a.array = a.array[1 : ]
    return value
}

// 将最右端(索引为length - 1)的数据项移出数组，并返回该数据项
func (a *SortedArray) PopRight() interface{} {
    a.mu.Lock()
    defer a.mu.Unlock()
    index  := len(a.array) - 1
    value  := a.array[index]
    a.array = a.array[: index]
    return value
}

// Calculate the sum of values in an array.
//
// 对数组中的元素项求和(将元素值转换为int类型后叠加)。
func (a *SortedArray) Sum() (sum int) {
    a.mu.RLock()
    defer a.mu.RUnlock()
    for _, v := range a.array {
        sum += gconv.Int(v)
    }
    return
}

// 数组长度
func (a *SortedArray) Len() int {
    a.mu.RLock()
    length := len(a.array)
    a.mu.RUnlock()
    return length
}

// 返回原始数据数组
func (a *SortedArray) Slice() []interface{} {
    array := ([]interface{})(nil)
    if a.mu.IsSafe() {
        a.mu.RLock()
        defer a.mu.RUnlock()
        array = make([]interface{}, len(a.array))
        copy(array, a.array)
    } else {
        array = a.array
    }
    return array
}

// 查找指定数值是否存在
func (a *SortedArray) Contains(value interface{}) bool {
    _, r := a.Search(value)
    return r == 0
}

// 查找指定数值的索引位置，返回索引位置(具体匹配位置或者最后对比位置)及查找结果
// 返回值: 最后比较位置, 比较结果
func (a *SortedArray) Search(value interface{}) (index int, result int) {
    return a.binSearch(value, true)
}

// 查找指定数值的索引位置，返回索引位置(具体匹配位置或者最后对比位置)及查找结果
// 返回值: 最后比较位置, 比较结果
func (a *SortedArray) binSearch(value interface{}, lock bool)(index int, result int) {
    if len(a.array) == 0 {
        return -1, -2
    }
    if lock {
        a.mu.RLock()
        defer a.mu.RUnlock()
    }
    min := 0
    max := len(a.array) - 1
    mid := 0
    cmp := -2
    for min <= max {
        mid = int((min + max) / 2)
        cmp = a.compareFunc(value, a.array[mid])
        switch {
            case cmp < 0 : max = mid - 1
            case cmp > 0 : min = mid + 1
            default :
                return mid, cmp
        }
    }
    return mid, cmp
}

// 设置是否允许数组唯一
func (a *SortedArray) SetUnique(unique bool) *SortedArray {
    oldUnique := a.unique.Val()
    a.unique.Set(unique)
    if unique && oldUnique != unique {
        a.Unique()
    }
    return a
}

// 清理数组中重复的元素项
func (a *SortedArray) Unique() *SortedArray {
    a.mu.Lock()
    defer a.mu.Unlock()
    i := 0
    for {
        if i == len(a.array) - 1 {
            break
        }
        if a.compareFunc(a.array[i], a.array[i + 1]) == 0 {
            a.array = append(a.array[ : i + 1], a.array[i + 1 + 1 : ]...)
        } else {
            i++
        }
    }
    return a
}

// 清空数据数组
func (a *SortedArray) Clear() *SortedArray {
    a.mu.Lock()
    if len(a.array) > 0 {
        a.array = make([]interface{}, 0, a.cap)
    }
    a.mu.Unlock()
    return a
}

// 使用自定义方法执行加锁修改操作
func (a *SortedArray) LockFunc(f func(array []interface{})) *SortedArray {
    a.mu.Lock(true)
    defer a.mu.Unlock(true)
    f(a.array)
    return a
}

// 使用自定义方法执行加锁读取操作
func (a *SortedArray) RLockFunc(f func(array []interface{})) *SortedArray {
    a.mu.RLock(true)
    defer a.mu.RUnlock(true)
    f(a.array)
    return a
}

// 合并两个数组.
func (a *SortedArray) Merge(array *SortedArray) *SortedArray {
    a.mu.Lock()
    defer a.mu.Unlock()
    if a != array {
        array.mu.RLock()
        defer array.mu.RUnlock()
    }
    a.array = append(a.array, array.array...)
    sort.Slice(a.array, func(i, j int) bool {
        return a.compareFunc(a.array[i], a.array[j]) < 0
    })
    return a
}

// Chunks an array into arrays with size elements. The last chunk may contain less than size elements.
func (a *SortedArray) Chunk(size int) [][]interface{} {
    if size < 1 {
        panic("size: cannot be less than 1")
    }
    a.mu.RLock()
    defer a.mu.RUnlock()
    length := len(a.array)
    chunks := int(math.Ceil(float64(length) / float64(size)))
    var n [][]interface{}
    for i, end := 0, 0; chunks > 0; chunks-- {
        end = (i + 1) * size
        if end > length {
            end = length
        }
        n = append(n, a.array[i*size : end])
        i++
    }
    return n
}

// Extract a slice of the array(If in concurrent safe usage, it returns a copy of the slice; else a pointer).
// It returns the sequence of elements from the array array as specified by the offset and length parameters.
func (a *SortedArray) SubSlice(offset, size int) []interface{} {
    a.mu.RLock()
    defer a.mu.RUnlock()
    if offset > len(a.array) {
        return nil
    }
    if offset + size > len(a.array) {
        size = len(a.array) - offset
    }
    if a.mu.IsSafe() {
        s := make([]interface{}, size)
        copy(s, a.array[offset:])
        return s
    } else {
        return a.array[offset:]
    }
}

// Picks one or more random entries out of an array(a copy), and returns the key (or keys) of the random entries.
func (a *SortedArray) Rand(size int) []interface{} {
    a.mu.RLock()
    defer a.mu.RUnlock()
    if size > len(a.array) {
        size = len(a.array)
    }
    n := make([]interface{}, size)
    for i, v := range grand.Perm(len(a.array)) {
        n[i] = a.array[v]
        if i == size - 1 {
            break
        }
    }
    return n
}

// Join array elements with a string.
func (a *SortedArray) Join(glue string) string {
    a.mu.RLock()
    defer a.mu.RUnlock()
    return strings.Join(gconv.Strings(a.array), glue)
}