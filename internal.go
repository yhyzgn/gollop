// Copyright 2020 yhyzgn gollop
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// author : 颜洪毅
// e-mail : yhyzgn@gmail.com
// time   : 2020-04-01 10:24 下午
// version: 1.0.0
// desc   : 接口定义

package gollop

import (
	"io"
	"time"
)

// 连接池接口
type Pooler interface {
	io.Closer // 可关闭
}

// 连接器接口
type Connector interface {
	io.Closer             // 可关闭
	CreatedAt() time.Time // 连接创建时间
	InUse(inUse bool)  // 设置连接使用状态
	IsInUse() bool        // 获取连接使用状态
	Err() error           // 获取连接器错误信息
}
