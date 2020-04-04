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
// time   : 2020-04-02 7:47 下午
// version: 1.0.0
// desc   : 一些声明与定义

package gollop

import (
	"runtime"
	"time"
)

// 连接请求
type connRequest struct {
	conn Connector
	err  error
}

// 获取连接策略
type connRequestStrategy uint8

const (
	strategyAlwaysNew = iota // 始终创建新连接
	strategyIdleOrNew        // 优先使用空闲连接
)

var (
	now                = time.Now         // right now
	connRequestSize    = 1000000          // 请求队列大小
	defaultMaxIdleConn = 4 * runtime.NumCPU() // 默认最大空闲数量
)
